/*
Copyright 2018 Tuenti Technologies S.L. All rights reserved.

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
	"path"
	"testing"

	"github.com/hashicorp/vault/api"
)

func setupAppRole(t *testing.T, name, token, address string, secret bool) string {
	v := vaultApi{
		Address: address,
		Token:   token,
	}
	options := RequestOptions{
		Data: map[string]interface{}{"type": "approle"},
	}
	_, err := v.Request("POST", AuthAppRoleURL, &options)
	if err != nil {
		t.Fatalf("couldn't enable approle auth method: %s", err)
	}

	roleURL := path.Join(AppRoleURL, name)
	roleParams := make(map[string]interface{})
	if !secret {
		roleParams["bind_secret_id"] = "false"
		roleParams["bound_cidr_list"] = "127.0.0.0/8"
	}
	options = RequestOptions{Data: roleParams}
	_, err = v.Request("POST", roleURL, &options)
	if err != nil {
		t.Fatalf("couldn't create approle testrole: %s ", err)
	}

	roleIDURL := path.Join(roleURL, "role-id")
	s, err := v.Request("GET", roleIDURL, nil)
	if err != nil {
		t.Fatalf("couldn't obtain role id: %s", err)
	}

	roleID := s.Data["role_id"].(string)

	return roleID
}

type DummyVault struct {
	T *testing.T

	ExpectedToken    string
	ExpectedSecretID string
	WrappedSecretID  string

	Token    string
	RoleID   string
	SecretID string

	Responses map[string]*api.Secret
}

func (v *DummyVault) Login() error {
	if v.Token != "" {
		return nil
	}
	if v.RoleID == "" {
		v.T.Fatalf("unset roleID")
	}
	if v.SecretID != v.ExpectedSecretID {
		v.T.Fatalf("incorrect secretID")
	}
	v.Token = v.ExpectedToken
	return nil
}

func (v *DummyVault) UnwrapSecretID(token string) error {
	if token != v.WrappedSecretID {
		v.T.Fatalf("incorrect wrapped secret ID")
	}
	v.SecretID = v.ExpectedSecretID
	v.WrappedSecretID = ""
	return nil
}

func (v *DummyVault) Request(method, urlPath string, options *RequestOptions) (*api.Secret, error) {
	if v.Token != v.ExpectedToken {
		v.T.Fatalf("incorrect token on request")
	}
	k := method + urlPath
	s, ok := v.Responses[k]
	if !ok {
		v.T.Fatalf("incorrect response")
	}
	return s, nil
}

func (v *DummyVault) GetToken() string {
	return v.Token
}
