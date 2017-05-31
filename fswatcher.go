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
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func (p *pouch) handleWrapped(path string) error {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = p.Vault.UnwrapSecretID(strings.TrimSpace(string(d)))
	if err != nil {
		return err
	}
	err = p.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *pouch) Watch(path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	dir := filepath.Dir(path)

	p.handleWrapped(path)

	errors := make(chan error)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Name == path && event.Op&fsnotify.Write != 0 {
					p.NotifyReload()
					err := p.handleWrapped(path)
					if err != nil {
						errors <- err
						return
					}
					p.AutoRestart()
				}
			case err := <-watcher.Errors:
				errors <- err
				return
			}
		}
	}()

	if !p.PendingSecrets() {
		log.Println("No pending secrets, we are ready")
		p.NotifyReady()
	}

	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	return <-errors
}
