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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/hashicorp/vault/api"
)

const (
	TokenHeader   = "X-Vault-Token"
	WrapTTLHeader = "X-Vault-Wrap-Ttl"

	SysHealthURL = "/v1/sys/health"

	AuthAppRoleURL  = "/v1/sys/auth/approle"
	AppRoleLoginURL = "/v1/auth/approle/login"
	AppRoleURL      = "/v1/auth/approle/role"

	UnwrapURL = "/v1/sys/wrapping/unwrap"
)

type VaultRequestOptions struct {
	WrapTTL string

	Parameters map[string]string
}

type VaultErrorResponse struct {
	Errors []string
}

func (r VaultErrorResponse) String() string {
	return strings.Join(r.Errors, ", ")
}

type Vault interface {
	Login() error
	Request(method, urlPath string, options *VaultRequestOptions) (*api.Secret, error)
	UnwrapSecretID(token string) error
}

type vaultApi struct {
	VaultConfig
}

func (v *vaultApi) Login() error {
	if v.Token != "" {
		return nil
	}
	if v.RoleID == "" {
		return fmt.Errorf("role ID needed")
	}
	params := make(map[string]string)
	params["role_id"] = v.RoleID
	if v.SecretID != "" {
		params["secret_id"] = v.SecretID
	}
	options := VaultRequestOptions{Parameters: params}
	resp, err := v.Request(http.MethodPost, AppRoleLoginURL, &options)
	if err != nil {
		return err
	}

	v.Token = resp.Auth.ClientToken

	return nil
}

func (v *vaultApi) UnwrapSecretID(token string) error {
	c := vaultApi{v.VaultConfig}
	c.Token = token
	resp, err := c.Request(http.MethodPost, UnwrapURL, nil)
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
	u, err := url.Parse(v.Address)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, urlPath)

	reqBody := bytes.NewBuffer([]byte{})
	if options != nil && len(options.Parameters) > 0 {
		d, err := json.Marshal(options.Parameters)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(d)
	}

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}
	if v.Token != "" {
		req.Header.Add(TokenHeader, v.Token)
	}
	if options != nil && options.WrapTTL != "" {
		req.Header.Add(WrapTTLHeader, options.WrapTTL)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var secret api.Secret
		err = json.Unmarshal(body, &secret)
		if err != nil {
			return nil, err
		}

		return &secret, nil
	case http.StatusNoContent:
		return nil, nil
	default:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf(resp.Status)
		}

		var errResp VaultErrorResponse
		err = json.Unmarshal(body, &errResp)
		if err != nil {
			return nil, fmt.Errorf(resp.Status)
		}

		return nil, fmt.Errorf("%s (%s)", resp.Status, errResp)

	}
}

func NewVault(config VaultConfig) Vault {
	return &vaultApi{config}
}
