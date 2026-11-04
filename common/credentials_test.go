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

package common

import (
	"maps"
	"strings"
	"testing"
)

func TestCredentialProfilesFlagRejectsInvalidInput(t *testing.T) {
	preserveCredentialProfileState(t)
	for _, input := range []string{"", "profiles: []", "{}", "profiles: [", "profiles: invalid"} {
		t.Run(input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Set panicked instead of returning an error: %v", r)
				}
			}()
			var profiles CredentialProfilesFlag
			if err := profiles.Set(input); err == nil {
				t.Error("expected invalid profiles to return an error")
			}
		})
	}
}

func TestCredentialProfilesFlagPreservesStateOnError(t *testing.T) {
	preserveCredentialProfileState(t)
	var profiles CredentialProfilesFlag
	if err := profiles.Set("profiles:\n  - name: existing\n    path: old/path\n"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Set panicked instead of returning an error: %v", r)
		}
	}()
	if err := profiles.Set("profiles: []"); err == nil {
		t.Fatal("expected empty profiles error")
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].Name != "existing" {
		t.Fatalf("invalid input changed profiles: %+v", profiles)
	}
	if !strings.Contains(ChassisCreds.Profiles["existing"].Path, "old/path") {
		t.Fatal("invalid input changed credential state")
	}
}

func preserveCredentialProfileState(t *testing.T) {
	t.Helper()
	profiles := maps.Clone(ChassisCreds.Profiles)
	defaultProfile := ChassisCreds.DefaultProfile
	t.Cleanup(func() {
		ChassisCreds.Profiles = profiles
		ChassisCreds.DefaultProfile = defaultProfile
	})
}
