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
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	DefaultStatePath           = "/var/lib/pouch/state"
	DefaultStateMode           = os.FileMode(0600)
	DefaultStateDirMode        = os.FileMode(0700)
	DefaultSecretDurationRatio = 0.75

	PreviousStateFilePostfix = "-prev"
)

type PouchState struct {
	// Last known token
	Token string `json:"token,omitempty"`

	// Secrets state
	Secrets map[string]*SecretState `json:"secrets,omitempty"`

	// Path from where this state was read
	path string
}

func NewState(path string) *PouchState {
	return &PouchState{path: path}
}

func LoadState(path string) (*PouchState, error) {
	if path == "" {
		path = DefaultStatePath
	}
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state PouchState
	err = json.Unmarshal(d, &state)
	if err != nil {
		return nil, err
	}
	state.path = path
	return &state, nil
}

func (s *PouchState) Save() error {
	path := s.path
	if path == "" {
		path = DefaultStatePath
	}

	// Create directories if they don't exist
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, DefaultStateDirMode)
		if err != nil {
			return err
		}
	}

	// Copy existing data to reduce risk of corrupting state
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		d, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(path+PreviousStateFilePostfix, d, DefaultStateMode)
		if err != nil {
			return err
		}
	}

	// Finally write the state
	d, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, d, DefaultStateMode)
}

func (s *PouchState) SetSecret(name string, secret *api.Secret) {
	if s.Secrets == nil {
		s.Secrets = make(map[string]*SecretState)
	}
	state := &SecretState{
		Name:          name,
		Timestamp:     time.Now(),
		LeaseDuration: secret.LeaseDuration,
	}
	if secret.Data != nil {
		ttlNumber, ok := secret.Data["ttl"].(json.Number)
		if ok {
			ttl, err := ttlNumber.Int64()
			if err == nil {
				state.TTL = int(ttl)
			}
		}
	}
	s.Secrets[name] = state
}

func (s *PouchState) DeleteSecret(name string) {
	delete(s.Secrets, name)
}

func (s *PouchState) NextUpdate() (secret *SecretState, minTTU time.Duration) {
	for name := range s.Secrets {
		ttu := s.Secrets[name].TimeToUpdate()
		if secret == nil || ttu < minTTU {
			secret = s.Secrets[name]
			minTTU = ttu
		}
	}
	if minTTU < 0 {
		minTTU = 0
	}
	return
}

type SecretState struct {
	// Secret name
	Name string `json:"name,omitempty"`

	// Time when the secret was read
	Timestamp time.Time `json:"creation_time,omitempty"`

	// Lease duration, in seconds, if any when the secret was read
	LeaseDuration int `json:"lease_id,omitempty"`

	// TTL, in seconds, if any when the secret was read
	TTL int `json:"ttl,omitempty"`

	// Secret will be renewed after this portion of its life has passed
	DurationRatio float64 `json:"duration_ratio,omitempty"`
}

func (s *SecretState) TimeToUpdate() time.Duration {
	ratio := s.DurationRatio
	if ratio == 0 {
		ratio = DefaultSecretDurationRatio
	}

	// Next update for the secret will be based on these rules:
	// - If we have both a TTL and a lease duration, we use the minimal of them
	// - If we have only a TTL or a lease duration, we take it
	// - If we don't have anything, we won't try to update this secret
	var duration int
	switch {
	case s.TTL > 0 && s.LeaseDuration > 0:
		if s.TTL < s.LeaseDuration {
			duration = s.TTL
		} else {
			duration = s.LeaseDuration
		}
	case s.TTL > 0:
		duration = s.TTL
	case s.LeaseDuration > 0:
		duration = s.LeaseDuration
	}

	return (time.Duration(float64(duration)*ratio) * time.Second) - time.Now().Sub(s.Timestamp)
}
