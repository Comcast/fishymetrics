package common

import (
	"context"
	"fmt"
	"sync"

	cm_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"
)

var (
	ChassisCreds = ChassisCredentials{
		Creds: make(map[string]*Credential),
	}

	log *zap.Logger
)

type ChassisCredentials struct {
	mu    sync.Mutex
	Creds map[string]*Credential
	Vault *cm_vault.Vault
}

type Credential struct {
	User string
	Pass string
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

func (c *ChassisCredentials) GetCredentials(ctx context.Context, target string) (*Credential, error) {
	var credential *Credential
	var ok bool
	var user, pass string

	log = zap.L()

	if c.Vault == nil {
		log.Error("issue retrieving credentials from vault using target "+target, zap.Error(fmt.Errorf("vault client not configured")))
		return credential, fmt.Errorf("issue retrieving credentials from vault using target: %s", target)
	}

	secret, err := c.Vault.GetKV2(ctx, target)
	if err != nil {
		log.Error("issue retrieving credentials from vault using target "+target, zap.Error(err))
		return credential, fmt.Errorf("issue retrieving credentials from vault using target: %s", target)
	}

	if user, ok = secret.Data[c.Vault.Parameters.Kv2UserField].(string); !ok {
		return credential, fmt.Errorf("the secret retrieved from vault using target %s is missing the %q field", target, c.Vault.Parameters.Kv2UserField)
	}

	if pass, ok = secret.Data[c.Vault.Parameters.Kv2PasswordField].(string); !ok {
		return credential, fmt.Errorf("the secret retrieved from vault using target %s is missing the %q field", target, c.Vault.Parameters.Kv2PasswordField)
	}
	credential = &Credential{
		User: user,
		Pass: pass,
	}

	return credential, nil
}
