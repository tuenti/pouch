/*
Copyright 2018 Tuenti Technologies S.L. All rights reserved.

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

package test

import (
	"testing"

	log "github.com/mgutz/logxi/v1"

	"github.com/hashicorp/vault/builtin/credential/approle"
	"github.com/hashicorp/vault/helper/logformat"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/physical"
	v "github.com/hashicorp/vault/vault"
)

// Based on TestCores on github.com/hashicorp/vault
func NewTestCoreAppRole(t *testing.T) (*v.Core, [][]byte, string) {
	logLevel := log.LevelError
	if testing.Verbose() {
		logLevel = log.LevelTrace
	}
	logger := logformat.NewVaultLogger(logLevel)
	physicalBackend := physical.NewInmem(logger)

	credentialBackends := make(map[string]logical.Factory)
	credentialBackends["approle"] = approle.Factory

	conf := &v.CoreConfig{
		Physical:           physicalBackend,
		CredentialBackends: credentialBackends,
		DisableMlock:       true,
		Logger:             logger,
	}
	core, err := v.NewCore(conf)

	keys, token := v.TestCoreInit(t, core)
	for _, key := range keys {
		if _, err := v.TestCoreUnseal(core, v.TestKeyCopy(key)); err != nil {
			t.Fatalf("unseal err: %s", err)
		}
	}

	sealed, err := core.Sealed()
	if err != nil {
		t.Fatalf("err checking seal status: %s", err)
	}
	if sealed {
		t.Fatal("should not be sealed")
	}

	if err != nil {
		t.Fatalf("couldn't start core: %v", err)
	}

	return core, keys, token
}
