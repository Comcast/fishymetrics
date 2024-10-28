/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	logg "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/comcast/fishymetrics/buildinfo"
	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/comcast/fishymetrics/exporter"
	"github.com/comcast/fishymetrics/exporter/moonshot"
	"github.com/comcast/fishymetrics/logger"
	"github.com/comcast/fishymetrics/middleware/logging"
	"github.com/comcast/fishymetrics/middleware/muxprom"
	"github.com/comcast/fishymetrics/plugins/nuova"
	fishy_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	app           = "fishymetrics"
	EnvVaultToken = "VAULT_TOKEN"
)

var (
	a                  = kingpin.New(app, "redfish api exporter with all the bells and whistles")
	username           = a.Flag("user", "BMC static username").Default("").Envar("BMC_USERNAME").String()
	password           = a.Flag("password", "BMC static password").Default("").Envar("BMC_PASSWORD").String()
	bmcTimeout         = a.Flag("timeout", "BMC scrape timeout").Default("15s").Envar("BMC_TIMEOUT").Duration()
	bmcScheme          = a.Flag("scheme", "BMC Scheme to use").Default("https").Envar("BMC_SCHEME").String()
	insecureSkipVerify = a.Flag("insecure-skip-verify", "Skip TLS verification").Default("false").Envar("INSECURE_SKIP_VERIFY").Bool()
	logLevel           = a.Flag("log.level", "log level verbosity").PlaceHolder("[debug|info|warn|error]").Default("info").Envar("LOG_LEVEL").String()
	logMethod          = a.Flag("log.method", "alternative method for logging in addition to stdout").PlaceHolder("[file|vector]").Default("").Envar("LOG_METHOD").String()
	logFilePath        = a.Flag("log.file-path", "directory path where log files are written if log-method is file").Default("/var/log/fishymetrics").Envar("LOG_FILE_PATH").String()
	logFileMaxSize     = a.Flag("log.file-max-size", "max file size in megabytes if log-method is file").Default("256").Envar("LOG_FILE_MAX_SIZE").String()
	logFileMaxBackups  = a.Flag("log.file-max-backups", "max file backups before they are rotated if log-method is file").Default("1").Envar("LOG_FILE_MAX_BACKUPS").String()
	logFileMaxAge      = a.Flag("log.file-max-age", "max file age in days before they are rotated if log-method is file").Default("1").Envar("LOG_FILE_MAX_AGE").String()
	vectorEndpoint     = a.Flag("vector.endpoint", "vector endpoint to send structured json logs to").Default("http://0.0.0.0:4444").Envar("VECTOR_ENDPOINT").String()
	exporterPort       = a.Flag("port", "exporter port").Default("10023").Envar("EXPORTER_PORT").String()
	vaultAddr          = a.Flag("vault.addr", "Vault instance address to get chassis credentials from").Default("https://vault.com").Envar("VAULT_ADDRESS").String()
	vaultRoleId        = a.Flag("vault.role-id", "Vault Role ID for AppRole").Default("").Envar("VAULT_ROLE_ID").String()
	vaultSecretId      = a.Flag("vault.secret-id", "Vault Secret ID for AppRole").Default("").Envar("VAULT_SECRET_ID").String()
	driveModExclude    = a.Flag("collector.drives.modules-exclude", "regex of drive module(s) to exclude from the scrape").Default("").Envar("COLLECTOR_DRIVES_MODULE_EXCLUDE").String()
	firmwareModExclude = a.Flag("collector.firmware.modules-exclude", "regex of firmware module(s) to exclude from the scrape").Default("").Envar("COLLECTOR_FIRMWARE_MODULE_EXCLUDE").String()
	credProfiles       = common.CredentialProf(a.Flag("credentials.profiles",
		`profile(s) with all necessary parameters to obtain BMC credential from secrets backend, i.e.
  --credentials.profiles="
    profiles:
      - name: profile1
        mountPath: "kv2"
        path: "path/to/secret"
        userField: "user"
        passwordField: "password"
      ...
  "
--credentials.profiles='{"profiles":[{"name":"profile1","mountPath":"kv2","path":"path/to/secret","userField":"user","passwordField":"password"},...]}'`))

	log *zap.Logger

	vault    *fishy_vault.Vault
	excludes = make(map[string]interface{})
	counter  int
)

var wg = sync.WaitGroup{}

func handler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var uri string
	var exp prometheus.Collector
	var err error

	target := query.Get("target")
	if len(query["target"]) != 1 || target == "" {
		log.Error("'target' parameter not set correctly", zap.String("target", target), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, "'target' parameter not set correctly", http.StatusBadRequest)
		return
	}

	model := query.Get("model")

	// optional query param is used to tell us which credential profile to use when retrieving that hosts username and password
	credProf := query.Get("credential_profile")

	// optional query param for external plugins which executes non redfish API calls to the device.
	// this is a comma separated list of strings
	plugins := strings.Split(query.Get("plugins"), ",")
	var plugs []exporter.Plugin
	for _, p := range plugins {
		if p == "nuova" {
			plugs = append(plugs, &nuova.NuovaPlugin{})
			log.Debug("nuova plugin added", zap.Any("trace_id", r.Context().Value("traceID")))
		}
	}

	log.Info("started scrape",
		zap.String("model", model),
		zap.String("target", target),
		zap.String("credential_profile", credProf),
		zap.Any("trace_id", r.Context().Value("traceID")))

	// check if vault is configured
	if vault != nil {
		var credential *common.Credential
		var err error
		// check if ChassisCredentials hashmap contains the credentials we need otherwise get them from vault
		if c, ok := common.ChassisCreds.Get(target); ok {
			credential = c
		} else {
			credential, err = common.ChassisCreds.GetCredentials(ctx, credProf, target)
			if err != nil {
				log.Error("issue retrieving credentials from vault using target "+target, zap.Error(err), zap.Any("trace_id", r.Context().Value("traceID")))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			common.ChassisCreds.Set(target, credential)
		}
	}

	registry := prometheus.NewRegistry()

	if model == "moonshot" {
		uri = "/rest/v1/chassis/1"
		exp, err = moonshot.NewExporter(r.Context(), target, uri, credProf)
	} else {
		uri = "/redfish/v1"
		exp, err = exporter.NewExporter(r.Context(), target, uri, credProf, model, excludes, plugs...)
	}

	if err != nil {
		log.Error("failed to create chassis exporter", zap.Error(err), zap.Any("trace_id", r.Context().Value("traceID")))
		http.Error(w, fmt.Sprintf("failed to create chassis exporter - %s", err.Error()), http.StatusInternalServerError)
		return
	}

	registry.MustRegister(exp)
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

	a.HelpFlag.Short('h')

	_, err = a.Parse(os.Args[1:])
	if err != nil {
		panic(fmt.Errorf("error parsing argument flags - %s", err.Error()))
	}

	// populate excludes map
	if *driveModExclude != "" {
		driveModPattern := regexp.MustCompile(*driveModExclude)
		excludes["drive"] = driveModPattern
	}

	if *firmwareModExclude != "" {
		firmwareModPattern := regexp.MustCompile(*firmwareModExclude)
		excludes["firmware"] = firmwareModPattern
	}

	// validate logFilePath exists and is a directory
	if *logMethod == "file" {
		fd, err := os.Stat(*logFilePath)
		if os.IsNotExist(err) {
			panic(err)
		}
		if !fd.IsDir() {
			panic(fmt.Errorf("%s is not a directory", *logFilePath))
		}
	}

	logfileMaxSize, err := strconv.Atoi(*logFileMaxSize)
	if err != nil {
		panic(fmt.Errorf("error converting arg --log.file-max-size to int - %s", err.Error()))
	}

	logfileMaxBackups, err := strconv.Atoi(*logFileMaxBackups)
	if err != nil {
		panic(fmt.Errorf("error converting arg --log.file-max-backups to int - %s", err.Error()))
	}

	logfileMaxAge, err := strconv.Atoi(*logFileMaxAge)
	if err != nil {
		panic(fmt.Errorf("error converting arg --log.file-max-age to int - %s", err.Error()))
	}

	c := &config.Config{
		BMCScheme:  *bmcScheme,
		BMCTimeout: *bmcTimeout,
		SSLVerify:  *insecureSkipVerify,
		User:       *username,
		Pass:       *password,
	}

	config.NewConfig(c)

	// init logger config
	logConfig := logger.LoggerConfig{
		LogLevel:  *logLevel,
		LogMethod: *logMethod,
		LogFile: logger.LogFile{
			Path:       *logFilePath,
			MaxSize:    logfileMaxSize,
			MaxBackups: logfileMaxBackups,
			MaxAge:     logfileMaxAge,
		},
		VectorEndpoint: *vectorEndpoint,
	}

	err = logger.Initialize(app, hostname, logConfig)
	if err != nil {
		panic(fmt.Errorf("error initializing logger - log_method=%s vector_endpoint=%s log_file_path=%s log_file_max_size=%d log_file_max_backups=%d log_file_max_age=%d - err=%s",
			*logMethod, *vectorEndpoint, *logFilePath, logfileMaxSize, logfileMaxBackups, logfileMaxAge, err.Error()))
	}

	log = zap.L()
	defer logger.Flush()

	if *logMethod == "vector" {
		log.Info("successfully initialized logger", zap.String("log_method", *logMethod),
			zap.String("vector_endpoint", *vectorEndpoint))
	} else if *logMethod == "file" {
		log.Info("successfully initialized logger", zap.String("log_method", *logMethod),
			zap.String("log_file_path", *logFilePath),
			zap.Int("log_file_max_size", logfileMaxSize),
			zap.Int("log_file_max_backups", logfileMaxBackups),
			zap.Int("log_file_max_age", logfileMaxAge))
	}

	// configure vault client if vaultRoleId & vaultSecretId are set
	if *vaultRoleId != "" && *vaultSecretId != "" {
		var err error
		vault, err = fishy_vault.NewVaultAppRoleClient(
			ctx,
			fishy_vault.Parameters{
				Address:         *vaultAddr,
				ApproleRoleID:   *vaultRoleId,
				ApproleSecretID: *vaultSecretId,
			},
		)
		if err != nil {
			log.Error("failed initializing vault client", zap.Error(err),
				zap.String("vault_address", *vaultAddr),
				zap.String("vault_role_id", *vaultRoleId))
		} else {
			// we add this here so we can update credentials once we detect they are rotated
			common.ChassisCreds.Vault = vault

			// start go routine to continuously renew vault token
			wg.Add(1)
			go vault.RenewToken(ctx, doneRenew, tokenLifecycle, &wg)
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /info", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(buildinfo.Info)
	})

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("GET /scrape", func(w http.ResponseWriter, r *http.Request) {
		handler(ctx, w, r)
	})

	tmplIndex := template.Must(template.New("index").Parse(indexTmpl))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		err := tmplIndex.Execute(w, buildinfo.Info)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	tmplIgnored := template.Must(template.New("ignored").Parse(ignoredTmpl))
	mux.HandleFunc("GET /ignored", func(w http.ResponseWriter, r *http.Request) {
		err := tmplIgnored.Execute(w, common.IgnoredDevices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("POST /ignored/test-conn", common.TestConn)
	mux.HandleFunc("POST /ignored/remove", common.RemoveHost)

	mux.HandleFunc("GET /verbosity", logger.Verbosity)
	mux.HandleFunc("PUT /verbosity", logger.SetVerbosity)

	instrumentation := muxprom.NewDefaultInstrumentation()
	wrappedmux := logging.LoggingHandler(instrumentation.Middleware(mux))

	srv := &http.Server{
		Addr:    ":" + *exporterPort,
		Handler: wrappedmux,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	listener, err := net.Listen("tcp4", ":"+*exporterPort)
	if err != nil {
		log.Error("starting "+app+" service failed", zap.Error(err))
		signals <- syscall.SIGTERM
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Error("http server received an error", zap.Error(err))
				signals <- syscall.SIGTERM
			}
		}()

		log.Info("started " + app + " service")
	}

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

	wg.Wait()
}
