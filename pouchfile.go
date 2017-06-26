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
	"io"
	"io/ioutil"
	"os"

	"github.com/tuenti/pouch/pkg/vault"

	"github.com/ghodss/yaml"
)

type Pouchfile struct {
	WrappedSecretIDPath string `json:"wrapped_secret_id_path,omitempty"`
	StatePath           string `json:"state_path,omitempty"`

	Vault   vault.Config   `json:"vault,omitempty"`
	Systemd SystemdConfig  `json:"systemd,omitempty"`
	Secrets []SecretConfig `json:"secrets,omitempty"`
}

type SystemdConfig struct {
	// If pouch should enable systemd support. Defaults to true
	// if systemd is available
	Enabled *bool `json:"enabled,omitempty"`
}

type systemdConfigurer struct {
	enabled bool
}

func (c *systemdConfigurer) Enabled() bool {
	return c.enabled
}

func (s *SystemdConfig) Configurer() *systemdConfigurer {
	return &systemdConfigurer{
		enabled: s.Enabled == nil || *s.Enabled,
	}
}

type SecretConfig struct {
	Name       string                 `json:"name,omitempty"`
	VaultURL   string                 `json:"vault_url,omitempty"`
	HTTPMethod string                 `json:"http_method,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Files      []FileConfig           `json:"files,omitempty"`
}

type FileConfig struct {
	Path         string `json:"path,omitempty"`
	Template     string `json:"template,omitempty"`
	TemplateFile string `json:"template_file,omitempty"`
}

func LoadPouchfile(path string) (*Pouchfile, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return loadPouchfile(r)
}

func loadPouchfile(r io.Reader) (*Pouchfile, error) {
	d, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var p Pouchfile
	err = yaml.Unmarshal(d, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
