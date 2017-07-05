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
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var isEmpty = errors.New("Empty wrapped secret ID file")

func (p *pouch) handleWrapped(path string) error {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if len(d) == 0 {
		return isEmpty
	}
	err = p.Vault.UnwrapSecretID(strings.TrimSpace(string(d)))
	if err != nil {
		return err
	}
	return nil
}

func (p *pouch) Watch(path string) error {
	// If the file is here, we are done, try before watching
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		err = p.handleWrapped(path)
		if err != isEmpty {
			return err
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	dir := filepath.Dir(path)
	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Name == path && event.Op&fsnotify.Write != 0 {
				err = p.handleWrapped(path)
				if err == isEmpty {
					continue
				}
				return err
			}
		case err := <-watcher.Errors:
			return err
		}
	}

	return nil
}
