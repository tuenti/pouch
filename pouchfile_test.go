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
	"strings"
	"testing"
)

var casePouchfile = `
wrappedSecretIDPath: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  roleID: kubelet
  secretID: ""
  token: ""
systemd:
  enabled: true
  autoRestart: false
secrets:
- vaultURL: /v1/kubernetes-pki/issue/kubelet
  httpMethod: POST
  localDir: /etc/kubernetes/ssl
  fileMap:
    private_key:
      name: client.key
    certificate:
      name: client.crt
    issuing_ca:
      name: ca.cr
`

var wrongPouchfile = `
wrappedSecretIDPath: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  roleID: kubelet
  secretID: ""
  token: ""
  unknownField: "wrong"
systemd:
  enabled: true
  autoRestart: false
secrets:
- vaultURL: /v1/kubernetes-pki/issue/kubelet
  httpMethod: POST
  localDir: /etc/kubernetes/ssl
  fileMap:
    private_key:
      name: client.key
    certificate:
      name: client.crt
    issuing_ca:
      name: ca.crt
`

func TestLoadPouchfile(t *testing.T) {
	_, err := loadPouchfile(strings.NewReader(casePouchfile))
	if err != nil {
		t.Fatal(err)
	}
}

func TestWrongPouchfile(t *testing.T) {
	// TODO: Detect unexpected fields (https://github.com/golang/go/issues/15314)
	t.SkipNow()
	_, err := loadPouchfile(strings.NewReader(casePouchfile))
	if err == nil {
		t.Fatal("Pouchfile load should have failed")
	}
}
