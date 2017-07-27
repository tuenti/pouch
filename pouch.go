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

const (
	DefaultFileMode = os.FileMode(0600)
)

type Pouch interface {
	Run(context.Context) error
	Watch(path string) error
	AddStatusNotifier(StatusNotifier)
	ServiceReloader(Reloader)
}

type StatusNotifier interface {
	NotifyReady() error
}

type Reloader interface {
	Reload(context.Context, string) error
}

type pouch struct {
	State *PouchState

	Vault     vault.Vault
	Secrets   map[string]SecretConfig
	Files     map[string]FileConfig
	Notifiers map[string]NotifierConfig
	Reloader  Reloader

	statusNotifiers  []StatusNotifier
	pendingNotifiers map[string]bool
}

func getFileContent(fc FileConfig, data interface{}, secretFunc interface{}) (string, error) {
	if fc.Template != "" && fc.TemplateFile != "" {
		return "", fmt.Errorf("inline template and template file specified")
	}
	var t *template.Template
	funcMap := template.FuncMap{
		"secret": secretFunc,
	}
	var err error
	switch {
	case fc.Template != "":
		t, err = template.New("inline-template").Funcs(funcMap).Parse(fc.Template)
		if err != nil {
			return "", err
		}
	case fc.TemplateFile != "":
		d, err := ioutil.ReadFile(fc.TemplateFile)
		if err != nil {
			return "", err
		}
		t, err = template.New(fc.TemplateFile).Funcs(funcMap).Parse(string(d))
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("no content defined for file %s", fc.Path)
	}
	var b bytes.Buffer
	err = t.Execute(&b, data)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func dirMode(mode os.FileMode) os.FileMode {
	result := os.FileMode(0)
	for i := 01; i <= 0777; i *= 010 {
		mask := os.FileMode(7 * i)
		if mode&mask > 0 {
			result |= (mode & mask) | os.FileMode(i)
		}
	}
	return result
}

var dataFuncMap = template.FuncMap{
	"env":      os.Getenv,
	"hostname": os.Hostname,
}

func resolveData(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, d := range data {
		resolved, err := func() (interface{}, error) {
			v, ok := d.(string)
			if !ok {
				return d, nil
			}
			t, err := template.New("secret-data").Funcs(dataFuncMap).Parse(v)
			if err != nil {
				return d, err
			}
			var b bytes.Buffer
			err = t.Execute(&b, nil)
			if err != nil {
				return d, err
			}
			return b.String(), nil
		}()
		if err != nil {
			log.Printf("When resolving data template '%s' for '%s': %v", d, k, err)
		}
		result[k] = resolved
	}
	return result
}

func (p *pouch) resolveSecret(name string, c SecretConfig) error {
	options := &vault.RequestOptions{Data: resolveData(c.Data)}
	s, err := p.Vault.Request(c.HTTPMethod, c.VaultURL, options)
	if err != nil {
		return err
	}
	p.State.SetSecret(name, s)
	return nil
}

func (p *pouch) resolveFile(fc FileConfig) error {
	mode := os.FileMode(fc.Mode)
	if mode == 0 {
		mode = DefaultFileMode
	}
	dir := path.Dir(fc.Path)
	err := os.MkdirAll(dir, dirMode(mode))
	if err != nil {
		return err
	}

	secretFunc := func(name, key string) (interface{}, error) {
		secret, found := p.State.Secrets[name]
		if !found {
			return nil, fmt.Errorf("unknown secret: %s", name)
		}
		value, found := secret.Data[key]
		if !found {
			return nil, fmt.Errorf("unkown key in secret '%s': %s", name, key)
		}
		secret.RegisterUsage(fc.Path)
		return value, nil
	}

	content, err := getFileContent(fc, nil, secretFunc)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fc.Path, []byte(content), mode)
	if err != nil {
		return fmt.Errorf("couldn't write secret in '%s': %s", fc.Path, err)
	}

	p.addForNotify(fc.Notify...)
	return nil
}

func (p *pouch) Run(ctx context.Context) error {
	err := p.Vault.Login()
	if err != nil {
		return err
	}
	p.State.Token = p.Vault.GetToken()
	err = p.State.Save()
	if err != nil {
		log.Printf("Couldn't save state: %s", err)
	}

	for name, c := range p.Secrets {
		if s, found := p.State.Secrets[name]; found {
			// Clean files using this secret, we'll process templates in case
			// someone has changed
			s.FilesUsing = nil
		} else {
			err = p.resolveSecret(name, c)
			if err != nil {
				return err
			}
		}
	}

	for name := range p.State.Secrets {
		if _, found := p.Secrets[name]; !found {
			p.State.DeleteSecret(name)
		}
	}

	for _, fc := range p.Files {
		err := p.resolveFile(fc)
		if err != nil {
			return err
		}
	}

	p.NotifyReady()

	for {
		p.notifyPending()

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
			err = p.resolveSecret(s.Name, p.Secrets[s.Name])
			if err != nil {
				return err
			}
			for _, f := range p.State.Secrets[s.Name].FilesUsing {
				err = p.resolveFile(p.Files[f])
				if err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func NewPouch(s *PouchState, vc vault.Vault, sc map[string]SecretConfig, fc []FileConfig, nc map[string]NotifierConfig) Pouch {
	fileMap := make(map[string]FileConfig)
	for _, f := range fc {
		fileMap[f.Path] = f
	}
	return &pouch{State: s, Vault: vc, Secrets: sc, Files: fileMap, Notifiers: nc}
}

func (p *pouch) ServiceReloader(r Reloader) {
	p.Reloader = r
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

func (p *pouch) addForNotify(names ...string) {
	if p.pendingNotifiers == nil {
		p.pendingNotifiers = make(map[string]bool)
	}
	for _, name := range names {
		p.pendingNotifiers[name] = true
	}
}

func (p *pouch) notifyPending() {
	for pending := range p.pendingNotifiers {
		p.Notify(pending)
		delete(p.pendingNotifiers, pending)
	}
}
