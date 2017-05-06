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
	"path"
	"testing"

	"github.com/hashicorp/vault/http"
)

func TestLoginWithoutSecret(t *testing.T) {
	core, _, token := NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	v := vaultApi{
		VaultConfig{
			Address: address,
		},
	}
	v.RoleID = setupAppRole(t, "test", token, address, false)

	err := v.Login()
	if err != nil {
		t.Fatalf("couldn't login: %v", err)
	}
}

func TestLogin(t *testing.T) {
	core, _, token := NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	roleName := "test"
	roleID := setupAppRole(t, roleName, token, address, true)

	admin := vaultApi{
		VaultConfig{
			Address: address,
			Token:   token,
		},
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, err := admin.Request("POST", secretIDURL, nil)
	if err != nil {
		t.Fatalf("couldn't obtain secret-id: %v", err)
	}
	secretID, _ := s.Data["secret_id"].(string)

	v := vaultApi{
		VaultConfig{
			Address:  address,
			RoleID:   roleID,
			SecretID: secretID,
		},
	}

	err = v.Login()
	if err != nil {
		t.Fatalf("couldn't login: %v", err)
	}
}

func TestLoginWithWrappedSecret(t *testing.T) {
	core, _, token := NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	roleName := "test"
	roleID := setupAppRole(t, roleName, token, address, true)

	admin := vaultApi{
		VaultConfig{
			Address: address,
			Token:   token,
		},
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, err := admin.Request("POST", secretIDURL, &VaultRequestOptions{
		WrapTTL: "10s",
	})
	if err != nil {
		t.Fatalf("couldn't obtain wrapped secret-id: %v", err)
	}
	if s.WrapInfo == nil {
		t.Fatalf("no wrapped information in secret: %+v", s)
	}
	wrappedSecretID := s.WrapInfo.Token

	v := vaultApi{
		VaultConfig{
			Address: address,
			RoleID:  roleID,
		},
	}
	err = v.UnwrapSecretID(wrappedSecretID)
	if err != nil {
		t.Fatalf("couldn't unwrap secret-id: %v", err)
	}

	err = v.Login()
	if err != nil {
		t.Fatalf("couldn't login: %v", err)
	}
}
