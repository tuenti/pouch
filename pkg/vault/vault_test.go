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
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/tuenti/pouch/pkg/vault/test"

	"github.com/hashicorp/vault/http"
)

func setupAppRole(name, token, address string, secret bool) (string, error) {
	v := vaultApi{
		Address: address,
		Token:   token,
	}
	options := RequestOptions{
		Data: map[string]interface{}{"type": "approle"},
	}
	_, _, err := v.Request("POST", AuthAppRoleURL, &options)
	if err != nil {
		return "", fmt.Errorf("couldn't enable approle auth method: %s", err)
	}

	roleURL := path.Join(AppRoleURL, name)
	roleParams := make(map[string]interface{})
	if !secret {
		roleParams["bind_secret_id"] = "false"
		roleParams["bound_cidr_list"] = "127.0.0.0/8"
	}
	options = RequestOptions{Data: roleParams}
	_, _, err = v.Request("POST", roleURL, &options)
	if err != nil {
		return "", fmt.Errorf("couldn't create approle testrole: %s ", err)
	}

	roleIDURL := path.Join(roleURL, "role-id")
	s, _, err := v.Request("GET", roleIDURL, nil)
	if err != nil {
		return "", fmt.Errorf("couldn't obtain role id: %s", err)
	}

	roleID := s.Data["role_id"].(string)

	return roleID, nil
}

func TestLogin(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	roleName := "test"
	roleID, err := setupAppRole(roleName, token, address, true)
	if err != nil {
		t.Fatal(err)
	}

	admin := vaultApi{
		Address: address,
		Token:   token,
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, _, err := admin.Request("POST", secretIDURL, nil)
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
	roleID, err := setupAppRole(roleName, token, address, true)
	if err != nil {
		t.Fatal(err)
	}

	admin := vaultApi{
		Address: address,
		Token:   token,
	}
	secretIDURL := path.Join(AppRoleURL, roleName, "secret-id")
	s, _, err := admin.Request("POST", secretIDURL, &RequestOptions{
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
	_, _, err := admin.Request("POST", secretURL, &RequestOptions{
		Data: map[string]interface{}{
			secretName: secret,
		},
	})
	if err != nil {
		t.Fatalf("couldn't create secret with data: %v", err)
	}

	s, _, err := admin.Request("GET", secretURL, nil)
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

func TestTokenRenovation(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	admin := vaultApi{
		Address: address,
		Token:   token,
	}

	s, _, err := admin.Request("POST", TokenCreateURL, &RequestOptions{
		Data: map[string]interface{}{
			"renewable": true,
			"ttl":       "1h",
		},
	})
	if err != nil {
		t.Fatalf("couldn't create new token: %v", err)
	}

	adminWithTTL := vaultApi{
		Address: address,
		Token:   s.Auth.ClientToken,
	}

	renewable, err := adminWithTTL.renewToken()
	if err != nil {
		t.Fatalf("couldn't renew token: %v", err)
	}
	if !renewable {
		t.Fatalf("token should still be renewable")
	}
}

func TestTokenRenovationExpired(t *testing.T) {
	core, _, token := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	admin := vaultApi{
		Address: address,
		Token:   token,
	}

	s, _, err := admin.Request("POST", TokenCreateURL, &RequestOptions{
		Data: map[string]interface{}{
			"renewable": true,
			"ttl":       "0s",
		},
	})
	if err != nil {
		t.Fatalf("couldn't create new token: %v", err)
	}

	adminExpired := vaultApi{
		Address: address,
		Token:   s.Auth.ClientToken,
	}

	renewable, err := adminExpired.renewToken()
	if err == nil {
		t.Fatalf("token renovation should have failed")
	}
	if renewable {
		t.Fatalf("token should have been reported as non renewable")
	}
}

func TestTokenRenovationUnavailableServer(t *testing.T) {
	ln := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.WriteHeader(nethttp.StatusServiceUnavailable)
	}))
	defer ln.Close()

	v := vaultApi{
		Address: ln.URL,
		Token:   "some-token",
	}

	renewable, err := v.renewToken()
	if err == nil {
		t.Fatalf("token renovation should have failed")
	}
	if !renewable {
		t.Fatalf("token should still be considered as renewable")
	}
}

func TestGetTokenTTL(t *testing.T) {
	core, _, _ := test.NewTestCoreAppRole(t)
	ln, address := http.TestServer(t, core)
	defer ln.Close()

	broken := vaultApi{
		Address: address,
		Token:   "bad-token",
	}

	_, invalid, err := broken.tokenTTL()
	if err == nil {
		t.Fatalf("this should have failed")
	}
	if !invalid {
		t.Fatalf("token should be reported as invalid")
	}
}
