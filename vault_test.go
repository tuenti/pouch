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
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"
)

func devVaultServer(t *testing.T, token, address string) *os.Process {
	url, _ := url.Parse(address)
	cmd := exec.Command("vault", "server", "-dev",
		"-dev-root-token-id", token,
		"-dev-listen-address", url.Host)
	if testing.Verbose() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	if err != nil {
		t.Fatalf("couldn't start vault dev server: %v", err)
	}
	v := vaultApi{VaultConfig{
		Address: address,
		Token:   token,
	}}
	for retries := 5; retries > 0; retries-- {
		_, err = v.Request(http.MethodGet, SysHealthURL, nil)
		if err == nil {
			break
		}
		<-time.After(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("timeout while waiting for vault server to be ready: %v", err)
	}
	return cmd.Process
}

func setupAppRole(t *testing.T, token, address string, secret bool) string {
	v := vaultApi{
		VaultConfig{
			Address: address,
			Token:   token,
		},
	}
	options := VaultRequestOptions{
		Parameters: map[string]string{"type": "approle"},
	}
	_, err := v.Request(http.MethodPost, AuthAppRoleURL, &options)
	if err != nil {
		t.Fatalf("couldn't enable approle auth method: %s", err)
	}

	roleURL := path.Join(AppRoleURL, "testrole")
	roleParams := make(map[string]string)
	if !secret {
		roleParams["bind_secret_id"] = "false"
		roleParams["bound_cidr_list"] = "127.0.0.0/8"
	}
	options = VaultRequestOptions{Parameters: roleParams}
	_, err = v.Request(http.MethodPost, roleURL, &options)
	if err != nil {
		t.Fatalf("couldn't create approle testrole: %s ", err)
	}

	roleIDURL := path.Join(roleURL, "role-id")
	s, err := v.Request(http.MethodGet, roleIDURL, nil)
	if err != nil {
		t.Fatalf("couldn't obtain role id: %s", err)
	}

	roleID := s.Data["role_id"].(string)

	return roleID
}

func TestLoginWithoutSecret(t *testing.T) {
	token := "dev"
	address := "http://127.0.0.1:8201"
	server := devVaultServer(t, token, address)
	defer server.Kill()

	v := vaultApi{
		VaultConfig{
			Address: address,
		},
	}

	v.RoleID = setupAppRole(t, token, address, false)

	err := v.Login()
	if err != nil {
		t.Fatalf("couldn't login: %v", err)
	}
}

func TestLogin(t *testing.T) {
	token := "dev"
	address := "http://127.0.0.1:8201"
	server := devVaultServer(t, token, address)
	defer server.Kill()

	roleID := setupAppRole(t, token, address, true)

	admin := vaultApi{
		VaultConfig{
			Address: address,
			Token:   token,
		},
	}
	secretIDURL := path.Join(AppRoleURL, "testrole", "secret-id")
	s, err := admin.Request(http.MethodPost, secretIDURL, nil)
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
	token := "dev"
	address := "http://127.0.0.1:8201"
	server := devVaultServer(t, token, address)
	defer server.Kill()

	roleID := setupAppRole(t, token, address, true)

	admin := vaultApi{
		VaultConfig{
			Address: address,
			Token:   token,
		},
	}
	secretIDURL := path.Join(AppRoleURL, "testrole", "secret-id")
	s, err := admin.Request(http.MethodPost, secretIDURL, &VaultRequestOptions{
		WrapTTL: "10s",
	})
	if err != nil {
		t.Fatalf("couldn't obtain wrapped secret-id: %v", err)
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
