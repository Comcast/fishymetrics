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

package vault

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"go.uber.org/zap"
)

var (
	log *zap.Logger

	ErrBadTLSConfig = errors.New("bad TLS configuration")
)

type Parameters struct {
	// connection and credential parameters
	Address         string
	ApproleRoleID   string
	ApproleSecretID string
	CACertBytes     []byte
}

// the locations / field names of kv2 secrets
type SecretProperties struct {
	MountPath     string
	Path          string
	UserField     string
	PasswordField string
	SecretName    string
	UserName      string
}

type Vault struct {
	mu         sync.RWMutex
	client     *vault.Client
	Parameters Parameters
	isLoggedIn bool
}

// NewVaultAppRoleClient logs in to Vault using the AppRole authentication
// method, returning an authenticated client and the auth token itself, which
// can be periodically renewed.
func NewVaultAppRoleClient(ctx context.Context, parameters Parameters) (*Vault, error) {
	config := vault.DefaultConfig()
	config.Address = parameters.Address
	if len(parameters.CACertBytes) > 0 {
		if err := config.ConfigureTLS(&vault.TLSConfig{
			CACertBytes: parameters.CACertBytes,
		}); err != nil {
			return nil, fmt.Errorf("unable to configure TLS: %w", err)
		}
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize vault client: %w", err)
	}

	vault := &Vault{
		client:     client,
		Parameters: parameters,
	}

	return vault, nil
}

// A combination of a RoleID and a SecretID is required to log into Vault
// with AppRole authentication method.
func (v *Vault) login(ctx context.Context) (*vault.Secret, error) {
	var roleId, secretId string
	v.mu.RLock()
	roleId = v.Parameters.ApproleRoleID
	secretId = v.Parameters.ApproleSecretID
	v.mu.RUnlock()

	approleSecretID := &approle.SecretID{
		FromString: secretId,
	}

	appRoleAuth, err := approle.NewAppRoleAuth(
		roleId,
		approleSecretID,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize approle authentication method: %w", err)
	}

	authInfo, err := v.client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to login using approle auth method: %w", err)
	}

	return authInfo, nil
}

// GetKVSecret fetches the latest version of secret api key from kv-v1 or kv-v2
func (v *Vault) GetKVSecret(ctx context.Context, props *SecretProperties, secret string) (*vault.KVSecret, error) {
	var kvSecret *vault.KVSecret
	var err error
	var secretPath string

	if props.Path != "" {
		if props.SecretName != "" {
			secretPath = fmt.Sprintf("%s/%s", props.Path, props.SecretName)
		} else {
			secretPath = fmt.Sprintf("%s/%s", props.Path, secret)
		}
	} else {
		if props.SecretName != "" {
			secretPath = props.SecretName
		} else {
			secretPath = secret
		}
	}

	// perform more checks based on profile
	if props.MountPath != "kv2" {
		kvSecret, err = v.client.KVv1(props.MountPath).Get(ctx, secretPath)
	} else {
		kvSecret, err = v.client.KVv2(props.MountPath).Get(ctx, secretPath)
	}

	if err != nil {
		return kvSecret, fmt.Errorf("unable to read secret: %w", err)
	}

	return kvSecret, nil
}

func wait(sleepTime time.Duration, c chan bool) {
	time.Sleep(sleepTime)
	c <- true
}

func (v *Vault) IsLoggedIn() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isLoggedIn
}

func (v *Vault) setLoggedIn(b bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.isLoggedIn = b
}

func (v *Vault) RenewToken(ctx context.Context, doneRenew, tokenLifecycle chan bool, wg *sync.WaitGroup) {
	log = zap.L()
	retry := make(chan bool, 1)
	defer wg.Done()
	retry <- true

	for {
		select {
		case <-doneRenew:
			log.Info("stopping renew token go routine")
			return
		case <-retry:
			vaultLoginResp, err := v.login(ctx)
			if err != nil {
				log.Error("unable to authenticate to vault", zap.Error(err))
				v.setLoggedIn(false)
				go wait(10*time.Second, retry)
			} else {
				wg.Add(1)
				v.setLoggedIn(true)
				tokenErr := v.manageTokenLifecycle(ctx, vaultLoginResp, tokenLifecycle, wg)
				if tokenErr != nil {
					log.Error("unable to start managing token lifecycle", zap.Error(tokenErr))
				}
			}
		}
	}
}

// Starts token lifecycle management. Returns only fatal errors as errors,
// otherwise returns nil so we can attempt login again.
func (v *Vault) manageTokenLifecycle(ctx context.Context, token *vault.Secret, done chan bool, wg *sync.WaitGroup) error {
	var renewal *vault.RenewOutput

	log = zap.L()

	if token.Auth != nil {
		renew := token.Auth.Renewable
		if !renew {
			log.Info("token is not configured to be renewable. re-attempting login")
			return nil
		}
	}

	watcher, err := v.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret:    token,
		Increment: token.LeaseDuration / 2,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize new lifetime watcher for renewing auth token: %w", err)
	}

	go watcher.Start()
	defer wg.Done()
	defer func() {
		log.Info("revoking token before app shutdown")
		err := v.client.Auth().Token().RevokeSelfWithContext(ctx, v.client.Token())
		if err != nil {
			log.Error("unable to revoke token", zap.Error(err))
		}
	}()
	defer watcher.Stop()

	for {
		select {
		case <-done:
			log.Info("stopping token watcher go routine")
			return nil
		// `DoneCh` will return if renewal fails, or if the remaining lease
		// duration is under a built-in threshold and either renewing is not
		// extending it or renewing is disabled.
		case err := <-watcher.DoneCh():
			if err != nil {
				log.Error("failed to renew token. re-attempting login", zap.Error(err))
				return nil
			}
			// This occurs once the token has reached max TTL.
			log.Info("token can no longer be renewed. re-attempting login")
			return nil

		case renewal = <-watcher.RenewCh():
			v.client.SetToken(renewal.Secret.Auth.ClientToken)
			log.Info(fmt.Sprintf("successfully renewed: %#v", renewal))
		}
	}
}
