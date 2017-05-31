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
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"text/template"
)

type Pouch interface {
	Run() error
	Watch(path string) error
	AddReadyNotifier(ReadyNotifier)
	AddReloadNotifier(ReloadNotifier)
}

type ReadyNotifier interface {
	NotifyReady() error
}

type ReloadNotifier interface {
	NotifyReload() error
}

type pouch struct {
	Vault   Vault
	Secrets []SecretConfig

	readyNotifiers  []ReadyNotifier
	reloadNotifiers []ReloadNotifier
}

func (p *pouch) Run() error {
	err := p.Vault.Login()
	if err != nil {
		return err
	}
	for _, c := range p.Secrets {
		options := &VaultRequestOptions{Data: c.Data}
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

			var content string
			if fc.Template != "" {
				t, err := template.New("file").Parse(fc.Template)
				if err != nil {
					return err
				}
				var b bytes.Buffer
				err = t.Execute(&b, s.Data)
				if err != nil {
					return err
				}
				content = b.String()
			}

			err = ioutil.WriteFile(fc.Path, []byte(content), 0600)
			if err != nil {
				return fmt.Errorf("couldn't write secret in '%s': %s", p, err)
			}
		}
	}
	p.NotifyReady()
	return nil
}

func NewPouch(v Vault, s []SecretConfig) Pouch {
	return &pouch{Vault: v, Secrets: s}
}

func (p *pouch) AddReadyNotifier(n ReadyNotifier) {
	p.readyNotifiers = append(p.readyNotifiers, n)
}

func (p *pouch) NotifyReady() {
	for _, n := range p.readyNotifiers {
		err := n.NotifyReady()
		if err != nil {
			log.Println(err)
		}
	}
}

func (p *pouch) AddReloadNotifier(n ReloadNotifier) {
	p.reloadNotifiers = append(p.reloadNotifiers, n)
}

func (p *pouch) NotifyReload() {
	for _, n := range p.reloadNotifiers {
		err := n.NotifyReload()
		if err != nil {
			log.Println(err)
		}
	}
}
