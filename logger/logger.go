/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
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

package logger

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger      *zap.Logger
	path        string
	atomicLevel zap.AtomicLevel
)

type lumberjackSink struct {
	*lumberjack.Logger
}

type LoggerConfig struct {
	LogMethod      string
	LogFile        LogFile
	VectorEndpoint string
}

type LogFile struct {
	Path       string
	MaxSize    int
	MaxBackups int
	MaxAge     int
}

func (lumberjackSink) Sync() error {
	return nil
}

func Initialize(svc, hostname string, config LoggerConfig) {

	atomicLevel = zap.NewAtomicLevel()

	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(ProdEncoderConf()),
		os.Stdout,
		atomicLevel,
	), zap.AddCaller(),
		zap.Fields(
			zap.Field{
				Key:    "app",
				Type:   zapcore.StringType,
				String: svc,
			},
			zap.Field{
				Key:    "host",
				Type:   zapcore.StringType,
				String: hostname,
			},
		))

	if config.LogMethod == "file" {
		ljWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.LogFile.Path + "/" + svc + ".log",
			MaxSize:    config.LogFile.MaxSize, // megabytes
			MaxBackups: config.LogFile.MaxBackups,
			MaxAge:     config.LogFile.MaxAge, // days
		})

		ljCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(ProdEncoderConf()),
			ljWriteSyncer,
			atomicLevel)

		logger = logger.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core {
			return zapcore.NewTee(logger.Core(), ljCore)
		}))
	}

	if config.LogMethod == "vector" {
		url, err := url.Parse(config.VectorEndpoint)
		if err != nil {
			panic(err)
		}

		err = zap.RegisterSink(url.Scheme, initVectorSink)
		if err != nil {
			panic(err)
		}

		vecWriteSyncer, _, err := zap.Open(url.String())
		if err != nil {
			panic(err)
		}

		ws := zapcore.Lock(vecWriteSyncer)

		vectorCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(ProdEncoderConf()),
			ws,
			atomicLevel)

		logger = logger.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core {
			return zapcore.NewTee(logger.Core(), vectorCore)
		}))
	}

	zap.ReplaceGlobals(logger)
}

func Flush() {
	if logger != nil {
		logger.Sync()
	}
}

func SetLevel(l string) {
	atomicLevel.SetLevel(parseLevel(l))
}

func GetLevel() string {
	return atomicLevel.Level().String()
}

func parseLevel(l string) zapcore.Level {
	switch l {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func ProdEncoderConf() zapcore.EncoderConfig {
	encConf := zap.NewProductionEncoderConfig()
	encConf.EncodeTime = zapcore.RFC3339TimeEncoder

	return encConf
}

func Verbosity(w http.ResponseWriter, r *http.Request) {
	type Results struct {
		Level string `json:"verbosity"`
	}

	log := zap.L()
	level := GetLevel()
	log.Info("current logging level", zap.String("level", level))

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "{\"verbosity\": \"%s\"}", level)
}

func SetVerbosity(w http.ResponseWriter, r *http.Request) {
	log := zap.L()
	query := r.URL.Query()

	level := query.Get("v")
	if level == "" {
		http.Error(w, "'v' parameter is not set", http.StatusBadRequest)
		return
	}

	SetLevel(level)

	log.Info("updating logging level", zap.String("level", level))

	w.WriteHeader(http.StatusNoContent)
	fmt.Fprint(w, "")
}
