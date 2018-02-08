/*
Copyright 2017 Tuenti Technologies S.L. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vault

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	// Token renewal period is its TTL multiplied by this ratio
	AutoRenewPeriodRatio = 0.5
	TokenRetryPeriod     = 5 * time.Second

	TokenHeader   = "X-Vault-Token"
	WrapTTLHeader = "X-Vault-Wrap-Ttl"

	TokenCreateURL    = "/v1/auth/token/create"
	SelfTokenURL      = "/v1/auth/token/lookup-self"
	SelfTokenRenewURL = "/v1/auth/token/renew-self"

	SysHealthURL = "/v1/sys/health"

	AuthAppRoleURL  = "/v1/sys/auth/approle"
	AppRoleLoginURL = "/v1/auth/approle/login"
	AppRoleURL      = "/v1/auth/approle/role"
)

type RequestOptions struct {
	WrapTTL string

	Data map[string]interface{}
}

type Vault interface {
	Login() error
	Request(method, urlPath string, options *RequestOptions) (*api.Secret, *api.Response, error)
	UnwrapSecretID(token string) error
	GetToken() string
}

type Config struct {
	Address  string `json:"address,omitempty"`
	RoleID   string `json:"role_id,omitempty"`
	SecretID string `json:"secret_id,omitempty"`
	Token    string `json:"token,omitempty"`
}

type vaultApi struct {
	Address  string
	RoleID   string
	SecretID string
	Token    string
}

func New(c Config) Vault {
	return &vaultApi{
		Address:  c.Address,
		RoleID:   c.RoleID,
		SecretID: c.SecretID,
		Token:    c.Token,
	}
}

func (v *vaultApi) getClient() (*api.Client, error) {
	config := api.DefaultConfig()
	if err := config.ReadEnvironment(); err != nil {
		return nil, fmt.Errorf("couldn't read config from environment: %v", err)
	}
	if v.Address != "" {
		config.Address = v.Address
	}
	return api.NewClient(config)
}

// A token is considered invalid if we receive 400 status codes
func (v *vaultApi) tokenTTL() (ttl int64, invalid bool, err error) {
	s, resp, err := v.Request(http.MethodGet, SelfTokenURL, nil)
	if resp != nil && (resp.StatusCode == 400 || resp.StatusCode == 403) {
		return 0, true, err
	}
	if err != nil {
		return 0, false, fmt.Errorf("couldn't obtain self token information: %v", err)
	}
	ttlNumber, ok := s.Data["ttl"].(json.Number)
	if !ok {
		return 0, false, fmt.Errorf("couldn't obtain token TTL")
	}
	ttl, err = ttlNumber.Int64()
	return ttl, false, err
}

// A token is not considered renewable if we receive a 400 error
// for any other case we consider that it can still be renewed
// and we are having problems connecting to the server.
func (v *vaultApi) renewToken() (renewable bool, err error) {
	s, resp, err := v.Request(http.MethodPost, SelfTokenRenewURL, nil)
	if resp != nil && (resp.StatusCode == 400 || resp.StatusCode == 403) {
		return false, err
	}
	if s != nil && s.Auth != nil {
		return s.Auth.Renewable, err
	}
	if s != nil {
		return s.Renewable, err
	}
	return true, err
}

func (v *vaultApi) autoRenewToken() {
	const (
		stateUpdateTTL = iota
		stateRenew
	)

	state := stateUpdateTTL
	var next time.Duration

	for {
		switch state {
		case stateUpdateTTL:
			ttl, invalid, err := v.tokenTTL()

			// For any other errors we should continue retryining till we
			// confirm that the token is definitively invalid
			if invalid {
				log.Println("Invalid token")
				return
			}

			if err != nil {
				log.Printf("Couldn't obtain token TTL: %s\n", err)
				next = TokenRetryPeriod
				break
			}

			if ttl == 0 {
				log.Println("Using token without expiration")
				return
			}

			state = stateRenew
			next = time.Duration(float64(ttl)*AutoRenewPeriodRatio) * time.Second
			log.Printf("Next token renewal in %s", next)

		case stateRenew:
			log.Println("Renewing token")
			renewable, err := v.renewToken()

			if err != nil {
				log.Printf("Couldn't renew token: %s\n", err)
				next = TokenRetryPeriod
			} else {
				state = stateUpdateTTL
				next = 0
			}

			if !renewable {
				log.Println("Token cannot be renewed anymore")
				return
			}
		}

		select {
		case <-time.After(next):
		}
	}

	log.Println("Won't autorenew token")
}

func (v *vaultApi) Login() error {
	if v.Token != "" {
		go v.autoRenewToken()
		return nil
	}
	if v.RoleID == "" {
		return fmt.Errorf("role ID needed")
	}
	data := make(map[string]interface{})
	data["role_id"] = v.RoleID
	if v.SecretID != "" {
		data["secret_id"] = v.SecretID
	}
	options := RequestOptions{Data: data}
	s, _, err := v.Request(http.MethodPost, AppRoleLoginURL, &options)
	if err != nil {
		return err
	}

	v.Token = s.Auth.ClientToken
	go v.autoRenewToken()

	return nil
}

func (v *vaultApi) UnwrapSecretID(token string) error {
	c, err := v.getClient()
	if err != nil {
		return err
	}
	c.SetToken(token)
	resp, err := c.Logical().Unwrap(token)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("no response?")
	}
	secretID, ok := resp.Data["secret_id"]
	if !ok {
		return fmt.Errorf("no secret ID found in response")
	}
	v.SecretID, ok = secretID.(string)
	if !ok {
		return fmt.Errorf("secret_id in response is not a string")
	}
	return nil
}

func (v *vaultApi) Request(method, urlPath string, options *RequestOptions) (*api.Secret, *api.Response, error) {
	c, err := v.getClient()
	if err != nil {
		return nil, nil, err
	}
	if v.Token != "" {
		c.SetToken(v.Token)
	}

	r := c.NewRequest(method, urlPath)
	if options != nil {
		if len(options.Data) > 0 {
			err = r.SetJSONBody(options.Data)
			if err != nil {
				return nil, nil, err
			}
		}

		if options.WrapTTL != "" {
			r.WrapTTL = options.WrapTTL
		}
	}

	resp, err := c.RawRequest(r)
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, resp, nil
	}
	s, err := api.ParseSecret(resp.Body)
	return s, resp, err
}

func (v *vaultApi) GetToken() string {
	return v.Token
}
