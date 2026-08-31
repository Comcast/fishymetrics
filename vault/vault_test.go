/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/helper/testcluster/docker"
	"github.com/stretchr/testify/assert"
)

// waitForCondition waits until the expected condition is true or the timeout
// expires. This avoids unnecessary fixed sleeps in token lifecycle tests.
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(250 * time.Millisecond)
	}

	t.Fatal(message)
}

func createVaultTestCluster(t *testing.T) (*docker.DockerCluster, string, string) {
	t.Helper()

	opts := &docker.DockerClusterOptions{
		ImageRepo: "hashicorp/vault",
		ImageTag:  "1.13.3",
	}
	opts.Logger = hclog.NewNullLogger()
	cluster := docker.NewTestDockerCluster(t, opts)

	client := cluster.Nodes()[0].APIClient()

	// create KV V1 mount
	if err := client.Sys().Mount("secret", &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "1",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// create KV V2 mount
	if err := client.Sys().Mount("kv2", &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// create KV V2 mount with a custom mount name
	if err := client.Sys().Mount("custom-kv2", &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// enable approle
	if err := client.Sys().EnableAuthWithOptions("approle", &vaultapi.EnableAuthOptions{
		Type: "approle",
	}); err != nil {
		t.Fatal(err)
	}

	// create an approle
	//
	// Use short token TTL values so the test can verify the same lifecycle
	// used in production: renew the token until max TTL, then authenticate
	// again using AppRole and obtain a new token.
	if _, err := client.Logical().Write("auth/approle/role/testrole", map[string]interface{}{
		"token_policies":     []string{"testrole"},
		"token_ttl":          "5s",
		"token_max_ttl":      "12s",
		"secret_id_ttl":      "0",
		"secret_id_num_uses": 0,
	}); err != nil {
		t.Fatal(err)
	}

	// get the role ID
	res, err := client.Logical().Read("auth/approle/role/testrole/role-id")
	if err != nil {
		t.Fatal(err)
	}

	roleID, ok := res.Data["role_id"].(string)
	if !ok {
		t.Fatal("Could not read the approle")
	}

	// create a secretID
	res, err = client.Logical().Write("auth/approle/role/testrole/secret-id", nil)
	if err != nil {
		t.Fatal(err)
	}

	secretID, ok := res.Data["secret_id"].(string)
	if !ok {
		t.Fatal("Could not generate the secret id")
	}

	// Create a broad policy to allow the approle to do whatever
	err = client.Sys().PutPolicy("testrole", `
    path "*" {
        capabilities = ["create", "read", "list", "update", "delete"]
    }`)
	if err != nil {
		t.Fatal(err)
	}

	// Create KV2 secrets
	if _, err := client.Logical().Write("kv2/data/testkv2secret", map[string]interface{}{
		"data": map[string]interface{}{
			"value": "testkv2value",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Logical().Write("kv2/data/morepath/testkv2secret", map[string]interface{}{
		"data": map[string]interface{}{
			"value": "testkv2value",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Create KV2 secret under custom mount
	if _, err := client.Logical().Write("custom-kv2/data/ipmi/testkv2secret", map[string]interface{}{
		"data": map[string]interface{}{
			"value": "testkv2value",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Create KV1 secret
	if _, err := client.Logical().Write("secret/testkv1secret", map[string]interface{}{
		"data": map[string]interface{}{
			"value": "testkv1value",
		},
	}); err != nil {
		t.Fatal(err)
	}

	return cluster, roleID, secretID
}

func Test_Vault_Auth(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	cluster, roleID, secretID := createVaultTestCluster(t)
	defer cluster.Cleanup()

	type testcase struct {
		name              string
		ctx               context.Context
		vaultParams       Parameters
		appRoleClientFunc func(t *testing.T, ctx context.Context, parameters Parameters) (*Vault, error)
		vaultClient       *Vault
		loginFunc         func(t *testing.T, ctx context.Context, v *Vault) (*vaultapi.Secret, error)
		getSecretFunc     func(t *testing.T, ctx context.Context, v *Vault, props *SecretProperties, secret string) (*vaultapi.KVSecret, error)
		secretProps       *SecretProperties
		validateFunc      func(t *testing.T, tc testcase) error
		cleanUpFunc       func(t *testing.T, client *vaultapi.Client) error
		expectErr         bool
	}

	goodParams := Parameters{
		Address:         cluster.Nodes()[0].APIClient().Address(),
		ApproleRoleID:   roleID,
		ApproleSecretID: secretID,
		CACertBytes:     cluster.CACertPEM,
	}

	createAppRoleClient := func(t *testing.T, ctx context.Context, parameters Parameters) (*Vault, error) {
		appRoleClient, err := NewVaultAppRoleClient(ctx, parameters)
		if err != nil {
			return nil, err
		}
		return appRoleClient, nil
	}

	login := func(t *testing.T, ctx context.Context, v *Vault) (*vaultapi.Secret, error) {
		authInfo, err := v.login(ctx)
		if err != nil {
			return nil, err
		}
		return authInfo, nil
	}

	getSecret := func(t *testing.T, ctx context.Context, v *Vault, props *SecretProperties, secret string) (*vaultapi.KVSecret, error) {
		sec, err := v.GetKVSecret(ctx, props, secret)
		if err != nil {
			return nil, err
		}
		return sec, nil
	}

	cleanUp := func(t *testing.T, client *vaultapi.Client) error {
		err := client.Auth().Token().RevokeSelfWithContext(ctx, client.Token())
		if err != nil {
			return err
		}
		return nil
	}

	tests := []testcase{
		{
			name: "Bad CACertBytes",
			ctx:  ctx,
			vaultParams: Parameters{
				Address:         goodParams.Address,
				ApproleRoleID:   goodParams.ApproleRoleID,
				ApproleSecretID: goodParams.ApproleSecretID,
				CACertBytes:     []byte("bad cert"),
			},
			appRoleClientFunc: createAppRoleClient,
			expectErr:         true,
		},
		{
			name:        "Bad Client Config",
			ctx:         ctx,
			vaultParams: goodParams,
			appRoleClientFunc: func(t *testing.T, ctx context.Context, parameters Parameters) (*Vault, error) {
				os.Setenv("VAULT_MAX_RETRIES", "badnumber")
				v, err := createAppRoleClient(t, ctx, parameters)
				if err != nil {
					os.Setenv("VAULT_MAX_RETRIES", "")
					return nil, err
				}
				return v, nil
			},
			expectErr: true,
		},
		{
			name:              "Good Client",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			expectErr:         false,
		},
		{
			name: "Bad Login Empty Secret ID",
			ctx:  ctx,
			vaultParams: Parameters{
				Address:         goodParams.Address,
				ApproleRoleID:   goodParams.ApproleRoleID,
				ApproleSecretID: "",
				CACertBytes:     goodParams.CACertBytes,
			},
			loginFunc:         login,
			appRoleClientFunc: createAppRoleClient,
			expectErr:         true,
		},
		{
			name: "Bad Login Incorrect Secret ID",
			ctx:  ctx,
			vaultParams: Parameters{
				Address:         goodParams.Address,
				ApproleRoleID:   goodParams.ApproleRoleID,
				ApproleSecretID: "bad-secret-id",
				CACertBytes:     goodParams.CACertBytes,
			},
			loginFunc:         login,
			appRoleClientFunc: createAppRoleClient,
			expectErr:         true,
		},
		{
			name:              "Good Login",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			cleanUpFunc:       cleanUp,
			expectErr:         false,
		},
		{
			name:              "Missing Secret",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath:  "kv2",
				KVVersion:  2,
				SecretName: "missing",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     true,
		},
		{
			name:              "Get Secret Path and SecretName",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath:  "kv2",
				Path:       "morepath",
				KVVersion:  2,
				SecretName: "testkv2secret",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Get Secret Path and no SecretName",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath: "kv2",
				KVVersion: 2,
				Path:      "morepath",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Get Secret SecretName and no Path",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath:  "kv2",
				KVVersion:  2,
				SecretName: "testkv2secret",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Get Secret no Path and no SecretName",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath: "kv2",
				KVVersion: 2,
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Get KVv1 Secret",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath:  "secret",
				SecretName: "testkv1secret",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Get KVv2 Secret Custom Mount",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			loginFunc:         login,
			secretProps: &SecretProperties{
				MountPath:  "custom-kv2",
				Path:       "ipmi",
				KVVersion:  2,
				SecretName: "testkv2secret",
			},
			getSecretFunc: getSecret,
			cleanUpFunc:   cleanUp,
			expectErr:     false,
		},
		{
			name:              "Token Reauthentication After Max TTL",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			secretProps: &SecretProperties{
				MountPath: "kv2",
				KVVersion: 2,
			},
			validateFunc: func(t *testing.T, tc testcase) error {
				var wg = sync.WaitGroup{}
				doneRenew := make(chan bool, 1)
				tokenLifecycle := make(chan bool, 1)

				wg.Add(1)
				go tc.vaultClient.RenewToken(ctx, doneRenew, tokenLifecycle, &wg)

				defer func() {
					select {
					case tokenLifecycle <- true:
					default:
					}

					select {
					case doneRenew <- true:
					default:
					}

					wg.Wait()
				}()

				// wait for the initial AppRole login
				waitForCondition(
					t,
					5*time.Second,
					func() bool {
						return tc.vaultClient.IsLoggedIn() &&
							tc.vaultClient.client.Token() != ""
					},
					"vault client did not authenticate",
				)

				initialToken := tc.vaultClient.client.Token()
				assert.NotEmpty(initialToken)

				// wait for the token to reach max TTL and a fresh AppRole
				// login to replace the old token
				waitForCondition(
					t,
					25*time.Second,
					func() bool {
						currentToken := tc.vaultClient.client.Token()

						return tc.vaultClient.IsLoggedIn() &&
							currentToken != "" &&
							currentToken != initialToken
					},
					"vault token was not replaced after reaching max TTL",
				)

				newToken := tc.vaultClient.client.Token()

				assert.NotEqual(initialToken, newToken)
				assert.True(tc.vaultClient.IsLoggedIn())

				// verify Vault can still read secrets using the new token
				_, err := getSecret(
					t,
					tc.ctx,
					tc.vaultClient,
					tc.secretProps,
					"testkv2secret",
				)
				if err != nil {
					return err
				}

				return nil
			},
			expectErr: false,
		},
		{
			name:              "Token Revoke",
			ctx:               ctx,
			vaultParams:       goodParams,
			appRoleClientFunc: createAppRoleClient,
			secretProps: &SecretProperties{
				MountPath: "kv2",
				KVVersion: 2,
			},
			validateFunc: func(t *testing.T, tc testcase) error {
				var wg = sync.WaitGroup{}
				doneRenew := make(chan bool, 1)
				tokenLifecycle := make(chan bool, 1)

				wg.Add(1)
				go tc.vaultClient.RenewToken(ctx, doneRenew, tokenLifecycle, &wg)

				// wait for token to setup
				waitForCondition(
					t,
					5*time.Second,
					tc.vaultClient.IsLoggedIn,
					"vault client did not authenticate",
				)

				assert.True(tc.vaultClient.IsLoggedIn())

				tokenLifecycle <- true
				doneRenew <- true

				wg.Wait()

				assert.False(tc.vaultClient.IsLoggedIn())

				// the current token should be revoked after shutdown
				_, err := getSecret(
					t,
					tc.ctx,
					tc.vaultClient,
					tc.secretProps,
					"testkv2secret",
				)

				assert.Error(err)

				return nil
			},
			expectErr: false,
		},
		{
			name: "Token Retry",
			ctx:  ctx,
			vaultParams: Parameters{
				Address:         goodParams.Address,
				ApproleRoleID:   goodParams.ApproleRoleID,
				ApproleSecretID: "bad-secret-id",
				CACertBytes:     goodParams.CACertBytes,
			},
			appRoleClientFunc: createAppRoleClient,
			validateFunc: func(t *testing.T, tc testcase) error {
				var wg = sync.WaitGroup{}
				doneRenew := make(chan bool, 1)
				tokenLifecycle := make(chan bool, 1)

				wg.Add(1)
				go tc.vaultClient.RenewToken(ctx, doneRenew, tokenLifecycle, &wg)

				defer func() {
					select {
					case tokenLifecycle <- true:
					default:
					}

					select {
					case doneRenew <- true:
					default:
					}

					wg.Wait()
				}()

				time.Sleep(2 * time.Second)
				assert.False(tc.vaultClient.IsLoggedIn())

				tc.vaultClient.mu.Lock()
				tc.vaultClient.Parameters.ApproleSecretID = goodParams.ApproleSecretID
				tc.vaultClient.mu.Unlock()

				// wait up to 15 seconds for token retry to happen
				waitForCondition(
					t,
					15*time.Second,
					tc.vaultClient.IsLoggedIn,
					"vault client did not authenticate after retry",
				)

				assert.True(tc.vaultClient.IsLoggedIn())

				return nil
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var vault *Vault
			var err error

			if test.appRoleClientFunc != nil {
				vault, err = test.appRoleClientFunc(t, ctx, test.vaultParams)
				if err != nil {
					if !test.expectErr {
						t.Error(err)
					}
				}
				test.vaultClient = vault
			}

			if test.loginFunc != nil {
				_, err := test.loginFunc(t, ctx, vault)
				if err != nil {
					if !test.expectErr {
						t.Error(err)
					}
				}
			}

			if test.getSecretFunc != nil {
				if test.secretProps.SecretName != "" {
					_, err = test.getSecretFunc(t, ctx, vault, test.secretProps, "")
				} else {
					_, err = test.getSecretFunc(t, ctx, vault, test.secretProps, "testkv2secret")
				}
				if err != nil {
					if !test.expectErr {
						t.Error(err)
					}
				}
			}

			if test.validateFunc != nil {
				err = test.validateFunc(t, test)
				if err != nil {
					if !test.expectErr {
						t.Error(err)
					}
				}
			}

			if test.cleanUpFunc != nil {
				err := test.cleanUpFunc(t, vault.client)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
