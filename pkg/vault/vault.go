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

	TokenHeader   = "X-Vault-Token"
	WrapTTLHeader = "X-Vault-Wrap-Ttl"

	SelfTokenURL      = "/v1//auth/token/lookup-self"
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
	Request(method, urlPath string, options *RequestOptions) (*api.Secret, error)
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
	if v.Address != "" {
		config.Address = v.Address
	}
	return api.NewClient(config)
}

func (v *vaultApi) tokenTTL() (int64, error) {
	resp, err := v.Request(http.MethodGet, SelfTokenURL, nil)
	if err != nil || resp == nil {
		return 0, fmt.Errorf("couldn't obtain self token information: %v", err)
	}
	ttlNumber, ok := resp.Data["ttl"].(json.Number)
	if !ok {
		return 0, fmt.Errorf("couldn't obtain token TTL")
	}
	return ttlNumber.Int64()
}

func (v *vaultApi) autoRenewToken() {
	var ttl int64
	var err error

	retry := func(attempts int, t time.Duration, f func() error) {
		for attempt := 0; attempt < attempts; attempt++ {
			err = f()
			if err == nil {
				<-time.After(t)
				break
			}
		}
	}

	for {
		retry(5, 1*time.Second, func() error {
			ttl, err = v.tokenTTL()
			return err
		})
		if err != nil {
			log.Printf("Couldn't obtain token TTL: %s\n", err)
			break
		}
		if ttl == 0 {
			log.Println("Using token without expiration")
			return
		}
		period := time.Duration(float64(ttl)*AutoRenewPeriodRatio) * time.Second
		log.Printf("Next token renewal in %s", period)
		<-time.After(period)
		retry(5, 1*time.Second, func() error {
			_, err = v.Request(http.MethodPost, SelfTokenRenewURL, nil)
			return err
		})
		if err != nil {
			log.Printf("Failed to renew token: %s", err)
			break
		}
		log.Println("Token succesfuly renewed")
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
	resp, err := v.Request(http.MethodPost, AppRoleLoginURL, &options)
	if err != nil {
		return err
	}

	v.Token = resp.Auth.ClientToken
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

func (v *vaultApi) Request(method, urlPath string, options *RequestOptions) (*api.Secret, error) {
	c, err := v.getClient()
	if err != nil {
		return nil, err
	}
	if v.Token != "" {
		c.SetToken(v.Token)
	}

	r := c.NewRequest(method, urlPath)
	if options != nil {
		if len(options.Data) > 0 {
			err = r.SetJSONBody(options.Data)
			if err != nil {
				return nil, err
			}
		}

		if options.WrapTTL != "" {
			r.WrapTTL = options.WrapTTL
		}
	}

	resp, err := c.RawRequest(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return api.ParseSecret(resp.Body)
}

func (v *vaultApi) GetToken() string {
	return v.Token
}
