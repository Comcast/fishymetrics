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

package config

import (
	"sync"
	"time"
)

type Config struct {
	BMCScheme  string
	BMCTimeout time.Duration
	User       string
	Pass       string
}

type SSLVerifyConfig struct {
	SSLVerify bool
}

var (
	config        *Config
	sslconfig     *SSLVerifyConfig
	once          sync.Once
	sslverifyonce sync.Once
)

func NewConfig(c *Config) {
	once.Do(func() {
		if c != nil {
			config = c
		} else {
			config = &Config{}
		}
	})
}

func NewSSLVerifyConfig(c *SSLVerifyConfig) {
	sslverifyonce.Do(func() {
		if c != nil {
			sslconfig = c
		} else {
			sslconfig = &SSLVerifyConfig{}
		}
	})
}

func GetConfig() *Config {
	if config != nil {
		return config
	}

	NewConfig(nil)
	return config
}

func GetSSLVerifyConfig() *SSLVerifyConfig {
	if sslconfig != nil {
		return sslconfig
	}

	NewSSLVerifyConfig(nil)
	return sslconfig
}
