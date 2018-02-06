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

	"github.com/tuenti/pouch/pkg/vault/test"

	"github.com/hashicorp/vault/http"
)

func TestLogin(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	roleName := "test"
	roleID := setupAppRole(t, roleName, token, address, true)

	admin := vaultApi{
		Address: address,
		Token:   token,
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, err := admin.Request("POST", secretIDURL, nil)
	if err != nil {
		t.Fatalf("couldn't obtain secret-id: %v", err)
	}
	secretID, _ := s.Data["secret_id"].(string)

	v := vaultApi{
		Address:  address,
		RoleID:   roleID,
		SecretID: secretID,
	}

	err = v.Login()
	if err != nil {
		t.Fatalf("couldn't login: %v", err)
	}
}

func TestLoginWithWrappedSecret(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	roleName := "test"
	roleID := setupAppRole(t, roleName, token, address, true)

	admin := vaultApi{
		Address: address,
		Token:   token,
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, err := admin.Request("POST", secretIDURL, &RequestOptions{
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
		Address: address,
		RoleID:  roleID,
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

func TestRequestWithData(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	admin := vaultApi{
		Address: address,
		Token:   token,
	}

	secretURL := "/v1/secret/foo"
	secretName := "foo"
	secret := "this is a secret!"
	_, err := admin.Request("POST", secretURL, &RequestOptions{
		Data: map[string]interface{}{
			secretName: secret,
		},
	})
	if err != nil {
		t.Fatalf("couldn't create secret with data: %v", err)
	}

	s, err := admin.Request("GET", secretURL, nil)
	if err != nil {
		t.Fatalf("couldn't read secret")
	}
	if s.Data == nil {
		t.Fatalf("empty data?")
	}
	foundSecret, found := s.Data[secretName]
	if !found {
		t.Fatalf("secret '%s' not found in %+v", secretName, s.Data)
	}

	if foundSecret != secret {
		t.Fatalf("found: %s, expected: %s", foundSecret, secret)
	}
}
