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
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tuenti/pouch"
	"github.com/tuenti/pouch/pkg/systemd"
	"github.com/tuenti/pouch/pkg/vault"
)

var version = "dev"

const defaultPouchfilePath = "Pouchfile"

func main() {
	var pouchfilePath string
	var showVersion bool
	flag.StringVar(&pouchfilePath, "pouchfile", defaultPouchfilePath, "Path to Pouchfile")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	pouchfile, err := pouch.LoadPouchfile(pouchfilePath)
	if err != nil {
		log.Fatalf("Couldn't load Pouchfile: %v", err)
	}

	state, err := pouch.LoadState(pouchfile.StatePath)
	if err == nil {
		log.Printf("Using state stored in %s", state.Path)
		pouchfile.Vault.Token = state.Token
	} else {
		log.Printf("Couldn't load state: %s, starting from scratch", err)
		state = pouch.NewState(pouchfile.StatePath)
	}

	vault := vault.New(pouchfile.Vault)

	p := pouch.NewPouch(state, vault, pouchfile.Secrets, pouchfile.Notifiers)

	systemd := systemd.New(pouchfile.Systemd.Configurer())
	if systemd.IsAvailable() && systemd.CanNotify() {
		p.AddStatusNotifier(systemd)
	}
	defer systemd.Close()

	if path := pouchfile.WrappedSecretIDPath; state.Token == "" && path != "" {
		log.Printf("Waiting for a wrapped secret ID in %s", path)
		err = p.Watch(path)
		if err != nil {
			log.Fatalf("Couldn't obtain secret ID from %s: %v", path, err)
		}
	}

	err = p.Run(context.Background())
	if err != nil {
		log.Fatalf("Pouch failed: %v", err)
	}
}
