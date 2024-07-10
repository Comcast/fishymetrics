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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	fishy_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
)

var (
	ChassisCreds = ChassisCredentials{
		Creds:    make(map[string]*Credential),
		Profiles: make(map[string]*fishy_vault.SecretProperties),
	}

	log *zap.Logger
)

type ChassisCredentials struct {
	mu             sync.Mutex
	Creds          map[string]*Credential
	Profiles       map[string]*fishy_vault.SecretProperties
	DefaultProfile string
	Vault          *fishy_vault.Vault
}

type Credential struct {
	User string
	Pass string
}

type ProfileFlag struct {
	Name          string `json:"name" yaml:"name"`
	MountPath     string `json:"mountPath" yaml:"mountPath"`
	Path          string `json:"path" yaml:"path"`
	UserField     string `json:"userField,omitempty" yaml:"userField,omitempty"`
	PasswordField string `json:"passwordField" yaml:"passwordField"`
	SecretName    string `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	UserName      string `json:"userName,omitempty" yaml:"userName,omitempty"`
}

type CredentialProfilesFlag struct {
	Profiles []ProfileFlag `json:"profiles" yaml:"profiles"`
}

func (cp *CredentialProfilesFlag) Set(value string) error {
	err := yaml.Unmarshal([]byte(value), cp)
	if err != nil {
		// if json was passed in we will attempt to unmarshal differently
		err := json.Unmarshal([]byte(value), cp)
		if err != nil {
			panic(fmt.Errorf("error parsing argument flag \"--credentials.profiles\" - %s", err.Error()))
		}
	}
	ChassisCreds.populateProfiles(cp)
	return nil
}

func (c *CredentialProfilesFlag) String() string {
	return fmt.Sprintf("%+v\n", *c)
}

func CredentialProf(s *kingpin.FlagClause) (target *CredentialProfilesFlag) {
	target = &CredentialProfilesFlag{}
	s.SetValue((*CredentialProfilesFlag)(target))
	return
}

func (c *ChassisCredentials) Get(key string) (*Credential, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	val, ok := c.Creds[key]
	return val, ok
}

func (c *ChassisCredentials) Set(key string, value *Credential) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Creds[key] = value
}

func (c *ChassisCredentials) populateProfiles(profiles *CredentialProfilesFlag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// default profile is the first one in the list
	c.DefaultProfile = profiles.Profiles[0].Name

	for _, v := range profiles.Profiles {
		c.Profiles[v.Name] = &fishy_vault.SecretProperties{
			MountPath:     v.MountPath,
			Path:          v.Path,
			UserField:     v.UserField,
			PasswordField: v.PasswordField,
			SecretName:    v.SecretName,
			UserName:      v.UserName,
		}
	}
}

func (c *ChassisCredentials) GetCredentials(ctx context.Context, profile, target string) (*Credential, error) {
	var credential *Credential
	var ok bool
	var user, pass string
	var credProf *fishy_vault.SecretProperties

	log = zap.L()

	if c.Vault == nil {
		return nil, fmt.Errorf("vault client not configured")
	}

	// check that atleast 1 profile is present
	if len(c.Profiles) < 1 {
		return nil, fmt.Errorf("no credential profiles configured")
	}

	// if profile is set but not in hashmap we will error
	if profile != "" {
		credProf, ok = c.Profiles[profile]
		if !ok {
			return nil, fmt.Errorf("profile \"%s\" not found", profile)
		}
	} else {
		// if profile is empty string we use the default profile
		credProf = c.Profiles[c.DefaultProfile]
	}

	secret, err := c.Vault.GetKVSecret(ctx, credProf, target)
	if err != nil {
		return nil, err
	}

	if credProf.UserName != "" {
		user = credProf.UserName
	} else {
		if user, ok = secret.Data[credProf.UserField].(string); !ok {
			return nil, fmt.Errorf("missing the \"%q\" user field", credProf.UserField)
		}
	}

	if pass, ok = secret.Data[credProf.PasswordField].(string); !ok {
		return nil, fmt.Errorf("missing the \"%q\" password field", credProf.PasswordField)
	}
	credential = &Credential{
		User: user,
		Pass: pass,
	}

	return credential, nil
}
