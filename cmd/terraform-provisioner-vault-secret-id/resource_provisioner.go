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
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/tuenti/pouch/pkg/vault"

	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provisioner() terraform.ResourceProvisioner {
	return &schema.Provisioner{
		Schema: map[string]*schema.Schema{
			"role": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Role for what to generate a secret ID.",
			},

			"destination": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path of the file in resource where secret will be stored",
			},

			"wrap_ttl": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "60s",
			},

			"address": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_ADDR", nil),
				Description: "URL of the root of the target Vault server.",
			},

			"token": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_TOKEN", ""),
				Description: "Token to use to authenticate to Vault.",
			},
		},

		ApplyFunc: applyFn,
	}
}

func retryFn(ctx context.Context, timeout time.Duration, f func() error) error {
	ctx, done := context.WithTimeout(ctx, timeout)
	defer done()

	for {
		err := f()
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			switch ctx.Err() {
			case context.Canceled:
				return fmt.Errorf("interrupted")
			case context.DeadlineExceeded:
				return fmt.Errorf("timeout, last error: %s", err)
			}
		case <-time.After(3 * time.Second):
		}
	}
}

func applyFn(ctx context.Context) error {
	connState := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)
	data := ctx.Value(schema.ProvConfigDataKey).(*schema.ResourceData)

	comm, err := communicator.New(connState)
	if err != nil {
		return err
	}

	err = retryFn(ctx, comm.Timeout(), func() error {
		return comm.Connect(nil)
	})
	if err != nil {
		return fmt.Errorf("couldn't connect: %s", err)
	}
	defer comm.Disconnect()

	address := data.Get("address").(string)
	token := data.Get("token").(string)
	v := vault.New(vault.Config{
		Address: address,
		Token:   token,
	})

	role := data.Get("role").(string)
	wrapTTL := data.Get("wrap_ttl").(string)
	options := vault.RequestOptions{WrapTTL: wrapTTL}
	s, err := v.Request("POST", path.Join(vault.AppRoleURL, role, "secret-id"), &options)
	if err != nil {
		return fmt.Errorf("couldn't get wrapped secret ID: %s", err)
	}
	wrappedSecretId := s.WrapInfo.Token

	destination := data.Get("destination").(string)

	f := strings.NewReader(wrappedSecretId)
	err = comm.Upload(destination, f)
	if err != nil {
		return fmt.Errorf("couldn't upload secret: %s", err)
	}

	return nil
}
