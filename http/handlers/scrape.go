/*
 * Copyright 2025 Comcast Cable Communications Management, LLC
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

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/exporter"
	"github.com/comcast/fishymetrics/exporter/moonshot"
	"github.com/comcast/fishymetrics/middleware/logging"
	"github.com/comcast/fishymetrics/plugins/nuova"
	fishy_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ScrapeConfig holds configuration for scrape handlers
type ScrapeConfig struct {
	Vault              *fishy_vault.Vault
	Excludes           map[string]interface{}
	URLExtraParamsMap  map[string]string
	ExtraParamsAliases map[string]string
}

// ScrapeHandler handles GET /scrape requests
func ScrapeHandler(cfg *ScrapeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(r.Context(), w, r, cfg)
	}
}

// PartialScrapeHandler handles GET /scrape/partial requests
func PartialScrapeHandler(cfg *ScrapeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		partialHandler(r.Context(), w, r, cfg)
	}
}

func handler(ctx context.Context, w http.ResponseWriter, r *http.Request, cfg *ScrapeConfig) {
	log := zap.L()
	query := r.URL.Query()
	var uri string
	var exp prometheus.Collector
	var err error

	target := query.Get("target")
	if len(query["target"]) != 1 || target == "" {
		log.Error("'target' parameter not set correctly", zap.String("target", target), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
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
			log.Debug("nuova plugin added", zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		}
	}

	log.Info("started scrape",
		zap.String("model", model),
		zap.String("target", target),
		zap.String("credential_profile", credProf),
		zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))

	// extract value of extra url param(s) from url query string if they are present.
	// we'll want to assign the key of the kv pair as the variable alias and the value as the
	// unique identifier for the alias
	extraParamsAliases := make(map[string]string)
	if len(cfg.URLExtraParamsMap) > 0 {
		for k, v := range cfg.URLExtraParamsMap {
			// check if url contains a valid key
			param := query.Get(k)
			if param != "" {
				extraParamsAliases[v] = param
			}
		}
		// Merge with existing aliases from config
		for k, v := range cfg.ExtraParamsAliases {
			extraParamsAliases[k] = v
		}
	}

	// Set configurations in common package for use in credential retrieval
	common.ExtraParamsAliases = extraParamsAliases

	// check if vault is configured
	if cfg.Vault != nil {
		// check if ChassisCredentials hashmap contains the credentials we need otherwise get them from vault
		if _, ok := common.ChassisCreds.Get(target); !ok {
			credential, err := common.ChassisCreds.GetCredentials(ctx, credProf, target, common.UpdateCredProfilePath(extraParamsAliases))
			if err != nil {
				log.Error("issue retrieving credentials from vault using target "+target, zap.Error(err), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			common.ChassisCreds.Set(target, credential)
		}
	}

	registry := prometheus.NewRegistry()

	if proxyHost := query.Get("proxy_host"); proxyHost != "" {
		if !strings.Contains(proxyHost, "://") {
			proxyHost = "http://" + proxyHost
		}
		// Basic validation: if malformed, return 400
		if _, err := url.Parse(proxyHost); err != nil {
			log.Error("invalid proxy_host parameter", zap.Error(err), zap.String("proxy_host", proxyHost),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			http.Error(w, "invalid proxy_host parameter", http.StatusBadRequest)
			return
		}
		ctx = exporter.WithProxyURL(ctx, proxyHost)
	}

	if model == "moonshot" {
		uri = "/rest/v1/chassis/1"
		exp, err = moonshot.NewExporter(ctx, target, uri, credProf)
	} else {
		uri = "/redfish/v1"
		exp, err = exporter.NewExporter(ctx, target, uri, credProf, model, cfg.Excludes, plugs...)
	}

	if err != nil {
		log.Error("failed to create chassis exporter", zap.Error(err), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, fmt.Sprintf("failed to create chassis exporter - %s", err.Error()), http.StatusInternalServerError)
		return
	}

	registry.MustRegister(exp)
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func partialHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, cfg *ScrapeConfig) {
	log := zap.L()
	query := r.URL.Query()
	var uri string
	var exp prometheus.Collector
	var err error

	// Get target parameter
	target := query.Get("target")
	if len(query["target"]) != 1 || target == "" {
		log.Error("'target' parameter not set correctly", zap.String("target", target),
			zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, "'target' parameter not set correctly", http.StatusBadRequest)
		return
	}

	// Get components parameter
	componentsStr := query.Get("components")
	if componentsStr == "" {
		log.Error("'components' parameter not set",
			zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, "'components' parameter is required. Valid components are: thermal, power, memory, processor, drives, storage_controller, firmware, system",
			http.StatusBadRequest)
		return
	}

	// Parse and validate components
	components, err := exporter.ParseComponents(componentsStr)
	if err != nil {
		log.Error("invalid components parameter", zap.Error(err), zap.String("components", componentsStr),
			zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	model := query.Get("model")

	// optional query param for credential profile
	credProf := query.Get("credential_profile")

	// optional query param for external plugins
	plugins := strings.Split(query.Get("plugins"), ",")
	var plugs []exporter.Plugin
	for _, p := range plugins {
		if p == "nuova" {
			plugs = append(plugs, &nuova.NuovaPlugin{})
			log.Debug("nuova plugin added", zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		}
	}

	log.Info("started partial scrape",
		zap.String("model", model),
		zap.String("target", target),
		zap.String("credential_profile", credProf),
		zap.Strings("components", func() []string {
			s := make([]string, len(components))
			for i, c := range components {
				s[i] = string(c)
			}
			return s
		}()),
		zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))

	// extract value of extra url param(s) from url query string if they are present
	extraParamsAliases := make(map[string]string)
	if len(cfg.URLExtraParamsMap) > 0 {
		for k, v := range cfg.URLExtraParamsMap {
			param := query.Get(k)
			if param != "" {
				extraParamsAliases[v] = param
			}
		}
		// Merge with existing aliases from config
		for k, v := range cfg.ExtraParamsAliases {
			extraParamsAliases[k] = v
		}
	}

	// check if vault is configured
	if cfg.Vault != nil {
		// check if ChassisCredentials hashmap contains the credentials we need otherwise get them from vault
		if _, ok := common.ChassisCreds.Get(target); !ok {
			credential, err := common.ChassisCreds.GetCredentials(ctx, credProf, target,
				common.UpdateCredProfilePath(extraParamsAliases))
			if err != nil {
				log.Error("issue retrieving credentials from vault using target "+target, zap.Error(err),
					zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			common.ChassisCreds.Set(target, credential)
		}
	}

	registry := prometheus.NewRegistry()

	// Optional per-request proxy override
	if proxyHost := query.Get("proxy_host"); proxyHost != "" {
		if !strings.Contains(proxyHost, "://") {
			proxyHost = "http://" + proxyHost
		}
		if _, err := url.Parse(proxyHost); err != nil {
			log.Error("invalid proxy_host parameter", zap.Error(err), zap.String("proxy_host", proxyHost),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			http.Error(w, "invalid proxy_host parameter", http.StatusBadRequest)
			return
		}
		ctx = exporter.WithProxyURL(ctx, proxyHost)
	}

	// Moonshot model doesn't support partial scraping yet
	if model == "moonshot" {
		log.Error("moonshot model does not support partial scraping",
			zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, "moonshot model does not support partial scraping", http.StatusBadRequest)
		return
	} else {
		uri = "/redfish/v1"
		exp, err = exporter.NewPartialExporter(ctx, target, uri, credProf, model, cfg.Excludes, components, plugs...)
	}

	if err != nil {
		log.Error("failed to create partial chassis exporter", zap.Error(err),
			zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		http.Error(w, fmt.Sprintf("failed to create partial chassis exporter - %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	registry.MustRegister(exp)
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
