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

package pouch

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/vault/api"
)

const (
	TokenHeader   = "X-Vault-Token"
	WrapTTLHeader = "X-Vault-Wrap-Ttl"

	SysHealthURL = "/v1/sys/health"

	AuthAppRoleURL  = "/v1/sys/auth/approle"
	AppRoleLoginURL = "/v1/auth/approle/login"
	AppRoleURL      = "/v1/auth/approle/role"
)

type VaultRequestOptions struct {
	WrapTTL string

	Data map[string]interface{}
}

type Vault interface {
	Login() error
	Request(method, urlPath string, options *VaultRequestOptions) (*api.Secret, error)
	UnwrapSecretID(token string) error
}

type vaultApi struct {
	Address  string
	RoleID   string
	SecretID string
	Token    string
}

func NewVault(c VaultConfig) Vault {
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

func (v *vaultApi) Login() error {
	if v.Token != "" {
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
	options := VaultRequestOptions{Data: data}
	resp, err := v.Request(http.MethodPost, AppRoleLoginURL, &options)
	if err != nil {
		return err
	}

	v.Token = resp.Auth.ClientToken

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

func (v *vaultApi) Request(method, urlPath string, options *VaultRequestOptions) (*api.Secret, error) {
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
