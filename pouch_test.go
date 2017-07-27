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
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/tuenti/pouch/pkg/vault"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func newTestState() (state *PouchState, cleanup func()) {
	f, _ := ioutil.TempFile("", "pouch-state-test")
	f.Close()
	return NewState(f.Name()), func() { os.Remove(f.Name()) }
}

func TestPouchRun(t *testing.T) {
	v := &vault.DummyVault{
		T: t,

		ExpectedToken:    "token",
		ExpectedSecretID: "secret",

		RoleID:   "roleid",
		SecretID: "secret",

		Responses: map[string]*api.Secret{
			"GET/v1/foo1": &api.Secret{
				Data: map[string]interface{}{"foo": "secretfoo", "bar": "secretbar"},
			},
			"GET/v1/foo2": &api.Secret{
				Data: map[string]interface{}{"baz": "secretbaz", "stuff": "secretstuff"},
			},
		},
	}
	tmpdir, err := ioutil.TempDir("", "pouch-test")
	if err != nil {
		t.Fatalf("couldn't create temporal directory")
	}
	defer os.RemoveAll(tmpdir)
	secrets := map[string]SecretConfig{
		"foo1": {
			VaultURL:   "/v1/foo1",
			HTTPMethod: "GET",
		},
		"foo2": {
			VaultURL:   "/v1/foo2",
			HTTPMethod: "GET",
		},
	}

	templateFile := path.Join(tmpdir, "template")
	err = ioutil.WriteFile(templateFile, []byte(`{{ secret "foo1" "foo" }}`), 0444)
	if err != nil {
		t.Fatalf("couldn't write template: %s", err)
	}

	files := []FileConfig{
		{Path: path.Join(tmpdir, "foo"), Template: `{{ secret "foo1" "foo" }}`},
		{Path: path.Join(tmpdir, "bar"), Template: `{{ secret "foo1" "foo" }} {{ secret "foo2" "baz"}}`},
		{Path: path.Join(tmpdir, "foo-template"), TemplateFile: templateFile},
	}

	state, cleanup := newTestState()
	defer cleanup()
	pouch := NewPouch(state, v, secrets, files, nil)

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error)
	go func() {
		finished <- pouch.Run(ctx)
	}()
	cancel()
	err = <-finished
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadFile(path.Join(tmpdir, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretfoo", "File content should be the secret")

	d, err = ioutil.ReadFile(path.Join(tmpdir, "bar"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretfoo secretbaz", "File content should be the secret")

	d, err = ioutil.ReadFile(path.Join(tmpdir, "foo-template"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretfoo", "File content should be the secret")
}

func TestPouchWatch(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "pouch-test")
	if err != nil {
		t.Fatalf("couldn't create temporal directory")
	}
	defer os.RemoveAll(tmpdir)

	secretWrapPath, err := ioutil.TempFile("", "pouch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(secretWrapPath.Name())

	v := &vault.DummyVault{
		T: t,

		ExpectedToken:    "token",
		ExpectedSecretID: "secret",
		WrappedSecretID:  "wrap",

		RoleID: "roleid",

		Responses: map[string]*api.Secret{
			"GET/v1/foo": &api.Secret{
				Data: map[string]interface{}{"foo": "secretfoo"},
			},
		},
	}
	secrets := map[string]SecretConfig{
		"foo": {
			VaultURL:   "/v1/foo",
			HTTPMethod: "GET",
		},
	}

	files := []FileConfig{
		{Path: path.Join(tmpdir, "foo"), Template: `{{ secret "foo" "foo" }}`},
	}

	state, cleanup := newTestState()
	defer cleanup()
	pouch := NewPouch(state, v, secrets, files, nil)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	err = w.Add(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	finished := make(chan error)
	go func() {
		finished <- pouch.Watch(secretWrapPath.Name())
	}()

	secretWrapPath.Write([]byte("wrap"))
	secretWrapPath.Close()

	err = <-finished
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, v.SecretID, v.ExpectedSecretID)
}

var dirModeCases = []struct {
	mode    os.FileMode
	dirMode os.FileMode
}{
	{os.FileMode(0000), os.FileMode(0000)},
	{os.FileMode(0004), os.FileMode(0005)},
	{os.FileMode(0640), os.FileMode(0750)},
	{os.FileMode(0400), os.FileMode(0500)},
	{os.FileMode(0666), os.FileMode(0777)},
	{os.FileMode(0444), os.FileMode(0555)},
	{os.FileMode(0777), os.FileMode(0777)},
	{os.FileMode(0640) | os.ModeSetuid, os.FileMode(0750)},
	{os.FileMode(0640) | os.ModeSticky, os.FileMode(0750)},
	{DefaultFileMode, os.FileMode(0700)},
}

func TestDirPerms(t *testing.T) {
	for _, c := range dirModeCases {
		d := dirMode(c.mode)
		assert.Equal(t, c.dirMode.String(), d.String())
	}
}

func TestResolveDataTemplates(t *testing.T) {
	env := "TESTENV"
	envValue := "foo"
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	hostname, _ := os.Hostname()

	data := map[string]interface{}{
		"env":      "{{ env \"TESTENV\" }}",
		"hostname": "{{ hostname }}",
	}

	resolvedData := resolveData(data)

	assert.Equal(t, envValue, resolvedData["env"])
	assert.Equal(t, hostname, resolvedData["hostname"])
}
