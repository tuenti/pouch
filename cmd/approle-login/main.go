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
	"io/ioutil"
	"os"
	"strings"

	"github.com/tuenti/pouch"
)

var version = "dev"

func main() {
	var address, roleId, secretId, wrappedSecretIdPath string
	var output string
	var showVersion bool

	flag.StringVar(&address, "address", "", "Address of vault server, VAULT_ADDR can be used instead")
	flag.StringVar(&roleId, "role-id", "", "Role ID to use for login")
	flag.StringVar(&secretId, "secret-id", "", "Secret ID to use for login")
	flag.StringVar(&output, "output", "", "Path to write the token")
	flag.StringVar(&wrappedSecretIdPath, "wrapped-secret-id-path", "", "Path to file containing a wrapped secret ID")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if roleId == "" {
		fmt.Println("-role-id is needed")
		os.Exit(-1)
	}

	vault := pouch.NewVault(pouch.VaultConfig{
		Address:  address,
		RoleID:   roleId,
		SecretID: secretId,
	})

	if wrappedSecretIdPath != "" {
		d, err := ioutil.ReadFile(wrappedSecretIdPath)
		if err != nil {
			fmt.Println("Couldn't read wrapped secret Id")
			os.Exit(-1)
		}
		err = vault.UnwrapSecretID(strings.TrimSpace(string(d)))
		if err != nil {
			fmt.Printf("Couldn't unwrap secret ID: %s\n", err)
			os.Exit(-1)
		}
	}

	err := vault.Login()
	if err != nil {
		fmt.Printf("Couldn't login to vault with provided credentials: %s\n", err)
		os.Exit(-1)
	}

	token := vault.GetToken()
	f := os.Stdout
	if output != "" {
		f, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			fmt.Printf("Couldn't open file %s: %s\n", output, err)
		}
		defer f.Close()
	}
	fmt.Fprintf(f, "VAULT_TOKEN=%s\n", token)
}
