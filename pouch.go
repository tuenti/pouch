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
	"fmt"
	"io/ioutil"
	"path"
)

type Pouch interface {
	Run() error
	Watch(path string) error
}

type pouch struct {
	Vault   Vault
	Secrets []SecretConfig
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
		for k, fm := range c.FileMap {
			v, found := s.Data[k]
			if !found {
				return fmt.Errorf("secret '%s' not found in '%s'", k, c.VaultURL)
			}
			vStr, ok := v.(string)
			if !ok {
				return fmt.Errorf("secret '%s' from '%s' couldn't be converted to string", k, c.VaultURL)
			}
			p := path.Join(c.LocalDir, fm.Name)
			err := ioutil.WriteFile(p, []byte(vStr), 0600)
			if err != nil {
				return fmt.Errorf("couldn't write secret in '%s': %s", p, err)
			}
		}
	}
	return nil
}

func NewPouch(v Vault, s []SecretConfig) Pouch {
	return &pouch{v, s}
}
