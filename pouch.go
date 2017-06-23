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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"text/template"

	"github.com/tuenti/pouch/pkg/vault"
)

type Pouch interface {
	Run() error
	Watch(path string) error
	AddStatusNotifier(StatusNotifier)
	AddAutoReloader(AutoReloader)
}

type StatusNotifier interface {
	NotifyReady() error
	NotifyReload() error
}

type AutoReloader interface {
	AutoReload() error
}

type pouch struct {
	State *PouchState

	Vault   vault.Vault
	Secrets []SecretConfig

	statusNotifiers []StatusNotifier
	autoReloaders   []AutoReloader
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

func (p *pouch) Run() error {
	err := p.Vault.Login()
	if err != nil {
		return err
	}
	p.State.Token = p.Vault.GetToken()

	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)

	for {
		for _, c := range p.Secrets {
			options := &vault.RequestOptions{Data: c.Data}
			s, err := p.Vault.Request(c.HTTPMethod, c.VaultURL, options)
			if err != nil {
				return err
			}
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
		}
		p.AutoRestart()
		p.NotifyReady()

		err = p.State.Save()
		if err != nil {
			log.Printf("Couldn't save state: %s", err)
		}

		select {
		case <-sigHup:
			p.NotifyReload()
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

func (p *pouch) NotifyReload() {
	for _, n := range p.statusNotifiers {
		err := n.NotifyReload()
		if err != nil {
			log.Println(err)
		}
	}
}

func (p *pouch) AddAutoReloader(n AutoReloader) {
	p.autoReloaders = append(p.autoReloaders, n)
}

func (p *pouch) AutoRestart() {
	for _, n := range p.autoReloaders {
		err := n.AutoReload()
		if err != nil {
			log.Println(err)
		}
	}

}
