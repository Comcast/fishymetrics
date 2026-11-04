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

package main

import (
	"fmt"
	"os"

	"github.com/comcast/fishymetrics/common"
)

func loadCredentialProfilesFile(profiles *common.CredentialProfilesFlag, path string) error {
	if path == "" {
		return nil
	}
	if len(profiles.Profiles) != 0 {
		return fmt.Errorf("cannot combine --credentials.profiles and --credentials.profiles-file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read credential profiles file %q: %w", path, err)
	}
	if err := profiles.Set(string(data)); err != nil {
		return fmt.Errorf("parse credential profiles file %q: %w", path, err)
	}
	return nil
}
