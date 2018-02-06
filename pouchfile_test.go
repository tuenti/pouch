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
	"strings"
	"testing"
)

var casePouchfiles = []string{
	`
wrapped_secret_id_path: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  role_id: kubelet
  secret_id: ""
  token: ""
systemd:
  enabled: true
secrets:
  foo:
    vault_url: /v1/kubernetes-pki/issue/kubelet
    http_method: POST
files:
  - path: /etc/kubernetes/ssl/client.key
    template: |
      {{ .private_key }}
  - path: /etc/kubernetes/ssl/client.crt
    template: |
      {{ .certificate }}
  - path: /etc/kubernetes/ssl/ca.crt
    template: |
      {{ .issuing_ca }}
`,
	`
wrapped_secret_id_path: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  role_id: kubelet
  secret_id: ""
  token: ""
systemd:
  enabled: true
secrets:
  nginx:
    vault_url: /v1/pki/issue/nginx
    http_method: POST
files:
  - path: /etc/nginx/ssl/bundle.crt
    template: |
      {{ .certificate }}
      {{ .issuing_ca }}
  - path: /etc/nginx/ssl/server.key
    template: |
      {{ .private_key }}
`,
	`
wrapped_secret_id_path: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  role_id: kubelet
  secret_id: ""
  token: ""
systemd:
  enabled: true
secrets:
  nginx:
    vault_url: /v1/pki/issue/nginx
    http_method: POST
files:
  - path: /etc/kubernetes/ssl/client.key
    priority: 10
    template: |
      {{ .private_key }}
  - path: /etc/kubernetes/ssl/client.crt
    priority: 20
    template: |
      {{ .certificate }}
  - path: /etc/kubernetes/ssl/ca.crt
    priority: 5
    template: |
      {{ .issuing_ca }}
`,
}

var wrongPouchfile = `
wrapped_secret_id_path: /var/run/vault_token
vault:
  address: http://127.0.0.1:8200
  role_id: kubelet
  secret_id: ""
  token: ""
  unknown_field: "wrong"
systemd:
  enabled: true
secrets:
- vault_url: /v1/kubernetes-pki/issue/kubelet
  http_method: POST
`

func TestLoadPouchfile(t *testing.T) {
	for _, c := range casePouchfiles {
		_, err := loadPouchfile(strings.NewReader(c))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWrongPouchfile(t *testing.T) {
	// TODO: Detect unexpected fields (https://github.com/golang/go/issues/15314)
	_, err := loadPouchfile(strings.NewReader(wrongPouchfile))
	if err == nil {
		t.Fatal("Pouchfile load should have failed")
	}
}
