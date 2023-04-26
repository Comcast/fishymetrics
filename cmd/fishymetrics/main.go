package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	logg "log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/comcast/fishymetrics/buildinfo"
	"github.com/comcast/fishymetrics/cisco/c220"
	"github.com/comcast/fishymetrics/cisco/s3260m4"
	"github.com/comcast/fishymetrics/cisco/s3260m5"
	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/comcast/fishymetrics/hpe/dl20"
	"github.com/comcast/fishymetrics/hpe/dl360"
	"github.com/comcast/fishymetrics/hpe/moonshot"
	"github.com/comcast/fishymetrics/logger"
	"github.com/comcast/fishymetrics/middleware/muxprom"
	cm_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	app = "fishymetrics"
)

var (
	username     *string
	password     *string
	oobTimeout   *time.Duration
	oobScheme    *string
	logPath      *string
	exporterPort *string

	log *zap.Logger

	// vault specific configs
	vaultAddr             *string
	vaultRoleId           *string
	vaultSecretId         *string
	vaultKv2Path          *string
	vaultKv2MountPath     *string
	vaultKv2UserField     *string
	vaultKv2PasswordField *string

	// credential hashmap so we can locally store creds for each chassis
	vault   *cm_vault.Vault
	counter int
)

var wg = sync.WaitGroup{}

func handler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var uri string
	var exporter prometheus.Collector
	var err error

	target := query.Get("target")
	if len(query["target"]) != 1 || target == "" {
		log.Error("'target' parameter not set correctly", zap.String("target", target), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, "'target' parameter not set correctly", http.StatusBadRequest)
		return
	}

	moduleName := query.Get("module")
	if len(query["module"]) != 1 || moduleName == "" {
		log.Error("'module' parameter not set correctly", zap.String("module", moduleName), zap.String("target", target), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, "'module' parameter not set correctly", http.StatusBadRequest)
		return
	}

	log.Info("started scrape", zap.String("module", moduleName), zap.String("target", target), zap.Any("trace_id", r.Context().Value("traceID")))

	// check if vault is configured
	if vault != nil {
		var credential *common.Credential
		var err error
		// check if ChassisCredentials hashmap contains the credentials we need otherwise get them from vault
		if c, ok := common.ChassisCreds.Get(target); ok {
			credential = c
		} else {
			credential, err = common.ChassisCreds.GetCredentials(ctx, target)
			if err != nil {
				log.Error("issue retrieving credentials from vault using target "+target, zap.Error(err), zap.Any("trace_id", r.Context().Value("traceID")))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			common.ChassisCreds.Set(target, credential)
		}
	}

	registry := prometheus.NewRegistry()

	if moduleName == "moonshot" {
		uri = "/rest/v1/chassis/1"
	} else {
		uri = "/redfish/v1"
	}

	switch moduleName {
	case "moonshot":
		exporter = moonshot.NewExporter(r.Context(), target, uri)
	case "dl360":
		exporter = dl360.NewExporter(r.Context(), target, uri)
	case "dl20":
		exporter = dl20.NewExporter(r.Context(), target, uri)
	case "c220":
		exporter, err = c220.NewExporter(r.Context(), target, uri)
	case "s3260m4":
		exporter, err = s3260m4.NewExporter(r.Context(), target, uri)
	case "s3260m5":
		exporter, err = s3260m5.NewExporter(r.Context(), target, uri)
	default:
		log.Error("'module' parameter does not match available options", zap.String("module", moduleName), zap.String("target", target), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, "'module' parameter does not match available options: [moonshot, dl360, dl20, c220, s3260m4, s3260m5]", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Error("failed to create chassis exporter", zap.Error(err), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, fmt.Sprintf("failed to create chassis exporter - %s", err.Error()), http.StatusInternalServerError)
		return
	}

	registry.MustRegister(exporter)
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	ctx := context.Background()
	doneRenew := make(chan bool, 1)
	tokenLifecycle := make(chan bool, 1)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	// This is a temporary fix for a bug on the Cisco redfish API
	// Unsolicited response received on idle HTTP channel starting with "\n"; err=<nil>
	logg.SetOutput(io.Discard)

	// We check for command line arguements first
	username = flag.String("user", "", "OOB static username")
	password = flag.String("password", "", "OOB static password")
	oobTimeout = flag.Duration("timeout", 15*time.Second, "OOB scrape timeout")
	oobScheme = flag.String("scheme", "https", "OOB Scheme to use")
	logPath = flag.String("log-path", "/var/log/fishymetrics", "directory path where log files are written")
	exporterPort = flag.String("port", "9533", "exporter port")
	vaultAddr = flag.String("vault-addr", "https://vault.com", "Vault instance address to get chassis credentials from")
	vaultRoleId = flag.String("vault-role-id", "", "Vault Role ID for AppRole")
	vaultSecretId = flag.String("vault-secret-id", "", "Vault Secret ID for AppRole")
	vaultKv2Path = flag.String("vault-kv2-path", "path/to/secrets", "Vault path where kv2 secrets will be retreived")
	vaultKv2MountPath = flag.String("vault-kv2-mount-path", "kv2", "Vault config path where kv2 secrets are mounted")
	vaultKv2UserField = flag.String("vault-kv2-user-field", "user", "Vault kv2 secret field where we get the username")
	vaultKv2PasswordField = flag.String("vault-kv2-password-field", "password", "Vault kv2 secret field where we get the password")

	flag.Parse()

	// Check if env variables are set
	if os.Getenv("USERNAME") != "" {
		*username = os.Getenv("USERNAME")
	}

	if os.Getenv("PASSWORD") != "" {
		*password = os.Getenv("PASSWORD")
	}

	if os.Getenv("OOB_TIMEOUT") != "" {
		*oobTimeout, _ = time.ParseDuration(os.Getenv("OOB_TIMEOUT"))
	}

	if os.Getenv("OOB_SCHEME") != "" {
		*oobScheme = os.Getenv("OOB_SCHEME")
	}

	if os.Getenv("LOG_PATH") != "" {
		*logPath = os.Getenv("LOG_PATH")
	}

	if os.Getenv("EXPORTER_PORT") != "" {
		*exporterPort = os.Getenv("EXPORTER_PORT")
	}

	if os.Getenv("VAULT_ADDRESS") != "" {
		*vaultAddr = os.Getenv("VAULT_ADDRESS")
	}

	if os.Getenv("VAULT_ROLE_ID") != "" {
		*vaultRoleId = os.Getenv("VAULT_ROLE_ID")
	}

	if os.Getenv("VAULT_SECRET_ID") != "" {
		*vaultSecretId = os.Getenv("VAULT_SECRET_ID")
	}

	if os.Getenv("VAULT_KV2_PATH") != "" {
		*vaultKv2Path = os.Getenv("VAULT_KV2_PATH")
	}

	if os.Getenv("VAULT_KV2_MOUNT_PATH") != "" {
		*vaultKv2MountPath = os.Getenv("VAULT_KV2_MOUNT_PATH")
	}

	if os.Getenv("VAULT_KV2_USER_FIELD") != "" {
		*vaultKv2UserField = os.Getenv("VAULT_KV2_USER_FIELD")
	}

	if os.Getenv("VAULT_KV2_PASS_FIELD") != "" {
		*vaultKv2PasswordField = os.Getenv("VAULT_KV2_PASS_FIELD")
	}

	// validate logPath exists and is a directory
	fd, err := os.Stat(*logPath)
	if os.IsNotExist(err) {
		panic(err)
	}
	if !fd.IsDir() {
		panic(fmt.Errorf("%s is not a directory", *logPath))
	}

	logger.Initialize(app, hostname, *logPath)
	log = zap.L()
	defer log.Sync()

	// configure vault client if vaultRoleId & vaultSecretId are set
	if *vaultRoleId != "" && *vaultSecretId != "" {
		var err error
		vault, err = cm_vault.NewVaultAppRoleClient(
			ctx,
			cm_vault.VaultParameters{
				Address:          *vaultAddr,
				ApproleRoleID:    *vaultRoleId,
				ApproleSecretID:  *vaultSecretId,
				Kv2Path:          *vaultKv2Path,
				Kv2MountPath:     *vaultKv2MountPath,
				Kv2UserField:     *vaultKv2UserField,
				Kv2PasswordField: *vaultKv2PasswordField,
			},
		)
		if err != nil {
			log.Error("failed initializing vault client", zap.Error(err))
		}

		// we add this here so we can update credentials once we detect they are rotated
		common.ChassisCreds.Vault = vault

		// start go routine to continuously renew vault token
		wg.Add(1)
		go vault.RenewToken(ctx, doneRenew, tokenLifecycle, &wg)
	}

	config.NewConfig(&config.Config{
		OOBScheme: *oobScheme,
		User:      *username,
		Pass:      *password,
	})

	mux := mux.NewRouter()

	instrumentation := muxprom.NewDefaultInstrumentation()
	mux.Use(instrumentation.Middleware)

	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(buildinfo.Info)
	}).Methods("GET")

	mux.Handle("/metrics", promhttp.Handler()).Methods("GET")

	mux.HandleFunc("/scrape", func(w http.ResponseWriter, r *http.Request) {
		handler(ctx, w, r)
	}).Methods("GET")

	tmplIndex := template.Must(template.New("index").Parse(indexTmpl))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := tmplIndex.Execute(w, buildinfo.Info)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	tmplIgnored := template.Must(template.New("ignored").Parse(ignoredTmpl))
	mux.HandleFunc("/ignored", func(w http.ResponseWriter, r *http.Request) {
		err := tmplIgnored.Execute(w, common.IgnoredDevices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	mux.HandleFunc("/ignored/test-conn", common.TestConn).Methods("POST")
	mux.HandleFunc("/ignored/remove", common.RemoveHost).Methods("POST")

	mux.HandleFunc("/verbosity", logger.Verbosity).Methods("GET")
	mux.HandleFunc("/verbosity", logger.SetVerbosity).Methods("PUT")

	srv := &http.Server{
		Addr:    ":" + *exporterPort,
		Handler: loggingHandler(app, mux),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("starting "+app+" service failed", zap.Error(err))
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := <-signals
		log.Info(s.String() + " signal caught, stopping app")
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("http server shutdown failed", zap.Error(err))
		}

		if vault != nil && vault.IsLoggedIn() {
			// send signal to stop token watcher if we were able to successfully login
			tokenLifecycle <- true
		}
		doneRenew <- true
	}()

	log.Info("started " + app + " service")

	wg.Wait()
}

// statusResponseWriter wraps an http.ResponseWriter, recording
// the status code for logging.
type statusResponseWriter struct {
	http.ResponseWriter
	status int // the http.ResponseWriter updates this value
}

// WriteHeader writes the header and saves the status for inspection.
func (r *statusResponseWriter) WriteHeader(status int) {
	r.ResponseWriter.WriteHeader(status)
	r.status = status
}

// loggingHandler accepts an http.Handler and wraps it with a
// handler that logs the request and response information.
func loggingHandler(ctx interface{}, h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		newCtx := context.WithValue(req.Context(), "traceID", uuid.New().String())
		req = req.WithContext(newCtx)
		srw := statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		query := req.URL.Query()

		defer func(start time.Time) {
			log.Info("finished handling",
				zap.String("module", query.Get("module")),
				zap.String("target", query.Get("target")),
				zap.String("sourceAddr", req.RemoteAddr),
				zap.String("method", req.Method),
				zap.String("url", req.URL.String()),
				zap.String("proto", req.Proto),
				zap.Int("status", srw.status),
				zap.Float64("elapsed_time_sec", time.Since(start).Seconds()),
				zap.Any("trace_id", req.Context().Value("traceID")),
			)
		}(time.Now())

		h.ServeHTTP(&srw, req)
	})
}
