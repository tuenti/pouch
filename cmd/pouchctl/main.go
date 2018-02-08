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
	"net/url"
	"os"
	"path"

	"github.com/tuenti/pouch/pkg/vault"
)

var version = "dev"

type Sender interface {
	Send(secret string) error
}

type StdoutSender struct{}

func (*StdoutSender) Send(secret string) error {
	fmt.Println(secret)
	return nil
}

func getSender(destination string) (Sender, error) {
	if destination == "" {
		return &StdoutSender{}, nil
	}

	u, err := url.Parse(destination)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "ssh", "scp", "sftp":
		return NewSSHSender(u), nil
	}

	return nil, fmt.Errorf("destination not supported")
}

func main() {
	var destination string
	var role, roleId, wrappedSecretId, wrapTTL string
	var address, token string
	var showVersion, genSecret, showRoleId bool

	flag.StringVar(&destination, "copy-to", "", "Destination for the wrapped secret")
	flag.StringVar(&role, "role", "", "Role to request a secret from")
	flag.StringVar(&wrapTTL, "wrap-ttl", "60s", "TTL for the wrapped secret ID")
	flag.StringVar(&address, "address", "", "Address of vault server, VAULT_ADDR can be used instead")
	flag.StringVar(&token, "token", "", "Token for authentication on vault, VAULT_TOKEN can be used instead")
	flag.BoolVar(&genSecret, "gen-secret", false, "Generates a wrapped secret")
	flag.BoolVar(&showRoleId, "show-role-id", false, "Shows role ID")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if role == "" {
		fmt.Println("Flag -role is required")
		os.Exit(-1)
	}

	v := vault.New(vault.Config{
		Address: address,
		Token:   token,
	})

	if showRoleId {
		s, _, err := v.Request("GET", path.Join(vault.AppRoleURL, role, "role-id"), nil)
		if err != nil {
			fmt.Printf("Couldn't get role ID: %s\n", err)
			os.Exit(-1)
		}
		roleId = s.Data["role_id"].(string)

		fmt.Printf("RoleID: %s\n", roleId)
	}

	if !genSecret {
		fmt.Println("Use -gen-secret to obtain a wrapped secret")
		return
	}

	sender, err := getSender(destination)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	options := vault.RequestOptions{WrapTTL: wrapTTL}
	s, _, err := v.Request("POST", path.Join(vault.AppRoleURL, role, "secret-id"), &options)
	if err != nil {
		fmt.Printf("Couldn't get wrapped secret ID: %s\n", err)
		os.Exit(-1)
	}
	wrappedSecretId = s.WrapInfo.Token

	err = sender.Send(wrappedSecretId)
	if err != nil {
		fmt.Println("Couldn't send secret:", err)
		os.Exit(-1)
	}
}
