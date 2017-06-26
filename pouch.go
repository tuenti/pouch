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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/tuenti/pouch/pkg/vault"
)

type Pouch interface {
	Run(context.Context) error
	Watch(path string) error
	AddStatusNotifier(StatusNotifier)
	AddReloader(Reloader)
}

type StatusNotifier interface {
	NotifyReady() error
}

type Reloader interface {
	Reload(string) error
}

type pouch struct {
	State *PouchState

	Vault   vault.Vault
	Secrets []SecretConfig

	statusNotifiers []StatusNotifier
	reloaders       []Reloader
}

func getFileContent(fc FileConfig, data interface{}) (string, error) {
	if fc.Template != "" && fc.TemplateFile != "" {
		return "", fmt.Errorf("inline template and template file specified")
	}
	var t *template.Template
	var err error
	switch {
	case fc.Template != "":
		t, err = template.New("file").Parse(fc.Template)
		if err != nil {
			return "", err
		}
	case fc.TemplateFile != "":
		t, err = template.ParseFiles(fc.TemplateFile)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("no content defined for file")
	}
	var b bytes.Buffer
	err = t.Execute(&b, data)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func (p *pouch) resolveSecret(c SecretConfig) error {
	options := &vault.RequestOptions{Data: c.Data}
	s, err := p.Vault.Request(c.HTTPMethod, c.VaultURL, options)
	if err != nil {
		return err
	}
	p.State.SetSecret(c.Name, s)
	for _, fc := range c.Files {
		dir := path.Dir(fc.Path)
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}

		content, err := getFileContent(fc, s.Data)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(fc.Path, []byte(content), 0600)
		if err != nil {
			return fmt.Errorf("couldn't write secret in '%s': %s", p, err)
		}
	}
	return nil
}

func (p *pouch) Run(ctx context.Context) error {
	err := p.Vault.Login()
	if err != nil {
		return err
	}
	p.State.Token = p.Vault.GetToken()

	secretConfigs := make(map[string]SecretConfig)
	for _, c := range p.Secrets {
		secretConfigs[c.Name] = c
		if _, found := p.State.Secrets[c.Name]; !found {
			err = p.resolveSecret(c)
			if err != nil {
				return err
			}
		}
	}

	for name := range p.State.Secrets {
		if _, found := secretConfigs[name]; !found {
			p.State.DeleteSecret(name)
		}
	}

	p.NotifyReady()

	for {
		err = p.State.Save()
		if err != nil {
			log.Printf("Couldn't save state: %s", err)
		}

		var nextUpdate <-chan time.Time
		s, ttu := p.State.NextUpdate()
		if s != nil {
			nextUpdate = time.After(ttu)
		} else {
			log.Printf("No secret to update")
		}

		select {
		case <-nextUpdate:
			log.Printf("Updating secret '%s'", s.Name)
			err = p.resolveSecret(secretConfigs[s.Name])
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func NewPouch(s *PouchState, vc vault.Vault, sc []SecretConfig) Pouch {
	return &pouch{State: s, Vault: vc, Secrets: sc}
}

func (p *pouch) AddStatusNotifier(n StatusNotifier) {
	p.statusNotifiers = append(p.statusNotifiers, n)
}

func (p *pouch) NotifyReady() {
	for _, n := range p.statusNotifiers {
		err := n.NotifyReady()
		if err != nil {
			log.Println(err)
		}
	}
}

func (p *pouch) AddReloader(n Reloader) {
	p.reloaders = append(p.reloaders, n)
}

func (p *pouch) Reload(name string) {
	for _, n := range p.reloaders {
		err := n.Reload(name)
		if err != nil {
			log.Println(err)
		}
	}
}
