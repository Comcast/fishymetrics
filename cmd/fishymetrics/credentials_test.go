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
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/comcast/fishymetrics/common"
)

func TestLoadCredentialProfilesFile(t *testing.T) {
	preserveCredentialProfileState(t)
	for _, tc := range []struct{ name, content string }{
		{"yaml", "profiles:\n  - name: primary\n    mountPath: kv2\n    path: \"devices/%target%\"\n    userField: username\n    passwordField: password\n    kvVersion: 2\n  - name: secondary\n    path: other/path\n"},
		{"json", `{"profiles":[{"name":"primary","mountPath":"kv2","path":"devices/%target%","userField":"username","passwordField":"password","kvVersion":2},{"name":"secondary","path":"other/path"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles config")
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			var profiles common.CredentialProfilesFlag
			if err := loadCredentialProfilesFile(&profiles, path); err != nil {
				t.Fatal(err)
			}
			if len(profiles.Profiles) != 2 || profiles.Profiles[0].Name != "primary" {
				t.Fatalf("incorrect profiles: %+v", profiles)
			}
			if common.ChassisCreds.DefaultProfile != "primary" {
				t.Fatalf("incorrect default: %s", common.ChassisCreds.DefaultProfile)
			}
			got := common.ChassisCreds.Profiles["primary"]
			if got.Path != "devices/%target%" || got.KVVersion != 2 || got.MountPath != "kv2" || got.UserField != "username" || got.PasswordField != "password" {
				t.Fatalf("incorrect secret properties: %+v", got)
			}
		})
	}
}

func TestLoadCredentialProfilesFileErrors(t *testing.T) {
	preserveCredentialProfileState(t)
	for _, tc := range []struct{ name, content string }{
		{"empty", ""}, {"empty profiles", "profiles: []"}, {"malformed", "profiles: ["},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			var profiles common.CredentialProfilesFlag
			if err := loadCredentialProfilesFile(&profiles, path); err == nil {
				t.Error("expected error")
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		var profiles common.CredentialProfilesFlag
		if err := loadCredentialProfilesFile(&profiles, filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Error("expected missing file error")
		}
	})
	t.Run("directory", func(t *testing.T) {
		var profiles common.CredentialProfilesFlag
		if err := loadCredentialProfilesFile(&profiles, t.TempDir()); err == nil {
			t.Error("expected directory error")
		}
	})
	t.Run("conflicting flags", func(t *testing.T) {
		var profiles common.CredentialProfilesFlag
		if err := profiles.Set("profiles:\n  - name: inline\n"); err != nil {
			t.Fatal(err)
		}
		err := loadCredentialProfilesFile(&profiles, filepath.Join(t.TempDir(), "unused"))
		if err == nil || !strings.Contains(err.Error(), "cannot combine") {
			t.Fatalf("expected mutually exclusive flags error, got %v", err)
		}
		if profiles.Profiles[0].Name != "inline" {
			t.Fatal("conflict changed inline profile")
		}
	})
	t.Run("omitted file", func(t *testing.T) {
		var profiles common.CredentialProfilesFlag
		if err := profiles.Set("profiles:\n  - name: inline\n"); err != nil {
			t.Fatal(err)
		}
		if err := loadCredentialProfilesFile(&profiles, ""); err != nil {
			t.Fatal(err)
		}
		if len(profiles.Profiles) != 1 || profiles.Profiles[0].Name != "inline" {
			t.Error("omitted file changed profiles")
		}
	})
}

func preserveCredentialProfileState(t *testing.T) {
	t.Helper()
	profiles := maps.Clone(common.ChassisCreds.Profiles)
	defaultProfile := common.ChassisCreds.DefaultProfile
	t.Cleanup(func() {
		common.ChassisCreds.Profiles = profiles
		common.ChassisCreds.DefaultProfile = defaultProfile
	})
}
