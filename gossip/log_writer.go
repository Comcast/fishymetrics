/*
 * Copyright 2026 Comcast Cable Communications Management, LLC
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

package gossip

import (
	"strings"

	"go.uber.org/zap"
)

// zapLogWriter adapts memberlist's io.Writer-based logging (via log.Logger)
// to zap so all gossip logs flow through the application's structured logger.
type zapLogWriter struct {
	log *zap.Logger
}

func newZapLogWriter(log *zap.Logger) *zapLogWriter {
	return &zapLogWriter{log: log.Named("memberlist")}
}

func (w *zapLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")

	switch {
	case strings.Contains(msg, "[ERR]"):
		w.log.Error(msg)
	case strings.Contains(msg, "[WARN]"):
		w.log.Warn(msg)
	case strings.Contains(msg, "[DEBUG]"):
		w.log.Debug(msg)
	default:
		w.log.Info(msg)
	}

	return len(p), nil
}
