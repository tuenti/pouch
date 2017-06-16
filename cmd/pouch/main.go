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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tuenti/pouch"
	"github.com/tuenti/pouch/pkg/systemd"
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

	vault := pouch.NewVault(pouchfile.Vault)

	p := pouch.NewPouch(vault, pouchfile.Secrets)

	systemd := systemd.New(pouchfile.Systemd.Configurer())
	if systemd.IsAvailable() {
		p.AddAutoReloader(systemd)
		if systemd.CanNotify() {
			p.AddStatusNotifier(systemd)
		}
	}
	defer systemd.Close()

	if pouchfile.WrappedSecretIDPath != "" {
		err = p.Watch(pouchfile.WrappedSecretIDPath)
	} else {
		err = p.Run()
	}
	if err != nil {
		log.Fatalf("Pouch failed: %v", err)
	}
}
