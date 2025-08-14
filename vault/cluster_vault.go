/*
 * Copyright 2025 Comcast Cable Communications Management, LLC
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
	"fmt"
	"sync"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"go.uber.org/zap"
)

// ClusterVault extends Vault with cluster-aware token management
type ClusterVault struct {
	*Vault
	tokenProvider TokenProvider
}

// TokenProvider interface for getting tokens from cluster state
type TokenProvider interface {
	GetToken() string
	UpdateToken(token string, expiresAt int64) error
	IsLeader() bool
}

// NewClusterVault creates a new cluster-aware Vault client
func NewClusterVault(ctx context.Context, parameters Parameters, tokenProvider TokenProvider) (*ClusterVault, error) {
	v, err := NewVaultAppRoleClient(ctx, parameters)
	if err != nil {
		return nil, err
	}

	cv := &ClusterVault{
		Vault:         v,
		tokenProvider: tokenProvider,
	}

	// Set the token from cluster state if available
	if token := tokenProvider.GetToken(); token != "" {
		v.client.SetToken(token)
		v.setLoggedIn(true)
	}

	return cv, nil
}

// RenewTokenWithCluster manages token lifecycle with cluster awareness
func (cv *ClusterVault) RenewTokenWithCluster(ctx context.Context, done chan bool, wg *sync.WaitGroup) {
	log = zap.L()
	defer wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			log.Info("stopping cluster vault token renewal")
			return
		case <-ticker.C:
			// Get token from cluster state
			token := cv.tokenProvider.GetToken()
			
			if token == "" && cv.tokenProvider.IsLeader() {
				// Leader needs to login and share token
				log.Info("leader logging in to vault")
				if err := cv.loginAndShare(ctx); err != nil {
					log.Error("failed to login to vault", zap.Error(err))
				}
				continue
			}

			if token != "" && token != cv.client.Token() {
				// Update local client with cluster token
				cv.client.SetToken(token)
				cv.setLoggedIn(true)
				log.Info("updated local vault client with cluster token")
			}

			// Only leader renews the token
			if cv.tokenProvider.IsLeader() && token != "" {
				if err := cv.renewSharedToken(ctx); err != nil {
					log.Error("failed to renew shared token", zap.Error(err))
					// Try to login again
					if err := cv.loginAndShare(ctx); err != nil {
						log.Error("failed to re-login to vault", zap.Error(err))
					}
				}
			}
		}
	}
}

func (cv *ClusterVault) loginAndShare(ctx context.Context) error {
	approleSecretID := &approle.SecretID{
		FromString: cv.Parameters.ApproleSecretID,
	}

	appRoleAuth, err := approle.NewAppRoleAuth(
		cv.Parameters.ApproleRoleID,
		approleSecretID,
	)
	if err != nil {
		return fmt.Errorf("unable to initialize approle authentication method: %w", err)
	}

	authInfo, err := cv.client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return fmt.Errorf("unable to login using approle auth method: %w", err)
	}

	if authInfo.Auth == nil {
		return fmt.Errorf("no auth info returned from login")
	}

	// Set the token in the client
	cv.client.SetToken(authInfo.Auth.ClientToken)
	cv.setLoggedIn(true)

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(authInfo.Auth.LeaseDuration) * time.Second).Unix()

	// Share the token with the cluster
	if err := cv.tokenProvider.UpdateToken(authInfo.Auth.ClientToken, expiresAt); err != nil {
		return fmt.Errorf("failed to share token with cluster: %w", err)
	}

	log.Info("successfully logged in to vault and shared token with cluster")
	return nil
}

func (cv *ClusterVault) renewSharedToken(ctx context.Context) error {
	// Create a renewer for the current token
	renewer, err := cv.client.Auth().Token().RenewSelfWithContext(ctx, 0)
	if err != nil {
		return fmt.Errorf("failed to renew token: %w", err)
	}

	// Calculate new expiration time (assume 1 hour extension)
	newExpiresAt := time.Now().Add(1 * time.Hour).Unix()

	// Update the shared token with new expiration
	if err := cv.tokenProvider.UpdateToken(cv.client.Token(), newExpiresAt); err != nil {
		return fmt.Errorf("failed to update shared token expiration: %w", err)
	}

	log.Info("successfully renewed shared vault token", zap.Any("response", renewer))
	return nil
}

// GetKVSecretWithCluster gets a secret using the cluster-shared token
func (cv *ClusterVault) GetKVSecretWithCluster(ctx context.Context, props *SecretProperties, secret string) (*vault.KVSecret, error) {
	// Ensure we have a valid token from cluster
	if token := cv.tokenProvider.GetToken(); token != "" && token != cv.client.Token() {
		cv.client.SetToken(token)
		cv.setLoggedIn(true)
	}

	if !cv.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to vault")
	}

	return cv.GetKVSecret(ctx, props, secret)
}