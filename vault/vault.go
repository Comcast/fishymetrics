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
	KVVersion     int
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

	v := &Vault{
		client:     client,
		Parameters: parameters,
	}

	return v, nil
}

// A combination of a RoleID and a SecretID is required to log into Vault
// with AppRole authentication method.
func (v *Vault) login(ctx context.Context) (*vault.Secret, error) {
	v.mu.RLock()
	roleID := v.Parameters.ApproleRoleID
	secretID := v.Parameters.ApproleSecretID
	v.mu.RUnlock()

	approleSecretID := &approle.SecretID{
		FromString: secretID,
	}

	appRoleAuth, err := approle.NewAppRoleAuth(roleID, approleSecretID)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize approle authentication method: %w", err)
	}

	authInfo, err := v.client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to login using approle auth method: %w", err)
	}

	return authInfo, nil
}

// GetKVSecret retrieves a secret from Vault using KV v1 or KV v2.
// MountPath is the actual Vault mount (e.g., "testing-path"),
// while KVVersion controls which API (v1/v2) is used.
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
	switch props.KVVersion {
	case 2:
		kvSecret, err = v.client.KVv2(props.MountPath).Get(ctx, secretPath)
	default:
		kvSecret, err = v.client.KVv1(props.MountPath).Get(ctx, secretPath)
	}

	if err != nil {
		return kvSecret, fmt.Errorf("unable to read secret: %w", err)
	}

	return kvSecret, nil
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

// waitBeforeRetry waits before retrying AppRole authentication while still
// allowing application shutdown to interrupt the wait.
func waitBeforeRetry(ctx context.Context, doneRenew <-chan bool, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-doneRenew:
		return false
	case <-ctx.Done():
		return false
	}
}

// RenewToken continuously manages Vault authentication.
//
// Once the current token can no longer be renewed, including when it reaches
// its maximum TTL, manageTokenLifecycle returns and the loop performs a fresh
// AppRole login to obtain a new token.
func (v *Vault) RenewToken(
	ctx context.Context,
	doneRenew, tokenLifecycle chan bool,
	wg *sync.WaitGroup,
) {
	log = zap.L()
	defer wg.Done()

	for {
		select {
		case <-doneRenew:
			v.setLoggedIn(false)
			log.Info("stopping renew token go routine")
			return
		case <-ctx.Done():
			v.setLoggedIn(false)
			log.Info("stopping renew token go routine", zap.Error(ctx.Err()))
			return
		default:
		}

		vaultLoginResp, err := v.login(ctx)
		if err != nil {
			v.setLoggedIn(false)
			log.Error("unable to authenticate to vault", zap.Error(err))

			if !waitBeforeRetry(ctx, doneRenew, 10*time.Second) {
				return
			}

			continue
		}

		v.setLoggedIn(true)

		log.Info(
			"successfully authenticated to vault",
			zap.Int("lease_duration", vaultLoginResp.Auth.LeaseDuration),
			zap.Bool("renewable", vaultLoginResp.Auth.Renewable),
		)

		reauthenticate, err := v.manageTokenLifecycle(
			ctx,
			vaultLoginResp,
			tokenLifecycle,
			doneRenew,
		)

		v.setLoggedIn(false)

		if err != nil {
			log.Error("unable to start managing token lifecycle", zap.Error(err))

			if !waitBeforeRetry(ctx, doneRenew, 10*time.Second) {
				return
			}

			continue
		}

		if !reauthenticate {
			return
		}

		// The current token can no longer be renewed. Loop back and perform
		// another AppRole login to obtain a fresh token.
		log.Info("vault token lifecycle ended. re-attempting login")
	}
}

// Starts token lifecycle management.
//
// Returns true when the current token can no longer be renewed and a fresh
// AppRole login should be attempted. Returns false when the application is
// shutting down.
func (v *Vault) manageTokenLifecycle(
	ctx context.Context,
	token *vault.Secret,
	tokenLifecycle, doneRenew <-chan bool,
) (bool, error) {
	log = zap.L()

	watcher, err := v.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: token,
	})
	if err != nil {
		return true, fmt.Errorf(
			"unable to initialize new lifetime watcher for renewing auth token: %w",
			err,
		)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case <-tokenLifecycle:
			log.Info("stopping token watcher go routine")

			log.Info("revoking token before app shutdown")
			if err := v.client.Auth().Token().RevokeSelfWithContext(ctx, v.client.Token()); err != nil {
				log.Error("unable to revoke token", zap.Error(err))
			}

			return false, nil

		case <-doneRenew:
			log.Info("stopping token watcher go routine")

			log.Info("revoking token before app shutdown")
			if err := v.client.Auth().Token().RevokeSelfWithContext(ctx, v.client.Token()); err != nil {
				log.Error("unable to revoke token", zap.Error(err))
			}

			return false, nil

		case <-ctx.Done():
			log.Info("stopping token watcher go routine", zap.Error(ctx.Err()))
			return false, nil

		// DoneCh will return if renewal fails, or if the remaining lease
		// duration is under a built-in threshold and either renewing is not
		// extending it or renewing is disabled.
		case err := <-watcher.DoneCh():
			if err != nil {
				log.Error("failed to renew token. re-attempting login", zap.Error(err))
				return true, nil
			}

			// This occurs once the token has reached max TTL.
			log.Info("token can no longer be renewed. re-attempting login")
			return true, nil

		case renewal := <-watcher.RenewCh():
			if renewal == nil || renewal.Secret == nil || renewal.Secret.Auth == nil {
				log.Warn("received incomplete token renewal response")
				continue
			}

			if renewal.Secret.Auth.ClientToken != "" {
				v.client.SetToken(renewal.Secret.Auth.ClientToken)
			}

			log.Info(
				"successfully renewed vault token",
				zap.Int("lease_duration", renewal.Secret.Auth.LeaseDuration),
				zap.Bool("renewable", renewal.Secret.Auth.Renewable),
			)
		}
	}
}
