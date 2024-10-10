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

package logging

import (
	"context"
	"net/http"
	"time"

	"github.com/nrednav/cuid2"
	"go.uber.org/zap"
)

var (
	log         *zap.Logger
	generate, _ = cuid2.Init(
		cuid2.WithLength(32),
	)
)

// LoggingHandler accepts an http.Handler and wraps it with a
// handler that logs the request and response information.
func LoggingHandler(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	log = zap.L()

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		newCtx := context.WithValue(req.Context(), "traceID", generate())
		req = req.WithContext(newCtx)
		srw := statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		query := req.URL.Query()

		defer func(start time.Time) {
			log.Info("finished handling",
				zap.String("model", query.Get("model")),
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
