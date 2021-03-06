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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	Path string `json:"-"`
}

func NewState(path string) *PouchState {
	return &PouchState{Path: path}
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
	state.Path = path
	return &state, nil
}

func (s *PouchState) Save() error {
	path := s.Path
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

// Sources of TTUs
var secretTTUSources = []func(*SecretState) (*time.Time, error){
	ttuFromTTLOrLeaseDuration,
	ttuFromCertificateValidity,
}

func ttuFromTTLOrLeaseDuration(s *SecretState) (*time.Time, error) {
	ttl, ttlKnown := s.TTL()

	var duration int
	switch {
	case ttlKnown && s.LeaseDuration > 0:
		if ttl < s.LeaseDuration {
			duration = ttl
		} else {
			duration = s.LeaseDuration
		}
	case ttlKnown:
		duration = ttl
	case s.LeaseDuration > 0:
		duration = s.LeaseDuration
	default:
		return nil, nil
	}

	ttu := s.Timestamp.Add(time.Duration(float64(duration)*s.Ratio()) * time.Second)
	return &ttu, nil
}

func ttuFromCertificateValidity(s *SecretState) (*time.Time, error) {
	if s.Data == nil {
		return nil, nil
	}

	data, ok := s.Data["certificate"].(string)
	if !ok {
		return nil, nil
	}

	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	ttl := certificate.NotAfter.Sub(certificate.NotBefore)
	ttu := certificate.NotBefore.Add(time.Duration(float64(ttl) * s.Ratio()))
	return &ttu, nil
}

func (s *PouchState) SetSecret(name string, secret *api.Secret) {
	if s.Secrets == nil {
		s.Secrets = make(map[string]*SecretState)
	}
	state := &SecretState{
		Name:          name,
		Timestamp:     time.Now(),
		LeaseDuration: secret.LeaseDuration,
		Data:          secret.Data,
	}

	if _, known := state.TimeToUpdate(); !known {
		// Without a known TTU, we don't know when to update
		state.DisableAutoUpdate = true
	}

	if oldState, found := s.Secrets[name]; found {
		state.FilesUsing = oldState.FilesUsing
	}
	s.Secrets[name] = state
}

func (s *PouchState) DeleteSecret(name string) {
	delete(s.Secrets, name)
}

func (s *PouchState) NextUpdate() (secret *SecretState, minTTU time.Time) {
	for name := range s.Secrets {
		if s.Secrets[name].DisableAutoUpdate {
			continue
		}
		ttu, known := s.Secrets[name].TimeToUpdate()
		if known && (secret == nil || ttu.Before(minTTU)) {
			secret = s.Secrets[name]
			minTTU = ttu
		}
	}
	return
}

type PriorityFileSortedList []PriorityFile

type PriorityFile struct {
	Priority int    `json:"-"`
	Path     string `json:"path,omitempty"`
}

func (pf *PriorityFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(pf.Path)
}

func (s *PriorityFileSortedList) UnmarshalJSON(data []byte) error {
	var priorityFiles []string

	if err := json.Unmarshal(data, &priorityFiles); err != nil {
		return err
	}

	// To keep the same state as when it was written, each file is assigned a
	// priority according to the order that they appear in the state file,
	for i, pf := range priorityFiles {
		*s = append(*s, PriorityFile{Path: pf, Priority: i * 10})
	}

	return nil
}

func (p PriorityFileSortedList) Len() int      { return len(p) }
func (p PriorityFileSortedList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p PriorityFileSortedList) Less(i, j int) bool {
	if p[i].Priority != p[j].Priority {
		return p[i].Priority < p[j].Priority
	}
	return p[i].Path < p[j].Path
}

type SecretData map[string]interface{}

type SecretState struct {
	// Secret name
	Name string `json:"name,omitempty"`

	// Time when the secret was read
	Timestamp time.Time `json:"creation_time,omitempty"`

	// Lease duration, in seconds, if any when the secret was read
	LeaseDuration int `json:"lease_duration,omitempty"`

	// Secret will be renewed after this portion of its life has passed
	DurationRatio float64 `json:"duration_ratio,omitempty"`

	// If the secret has no expiration data, don't try to update it
	DisableAutoUpdate bool `json:"disable_auto_uptdate,omitempty"`

	// Actual secret
	Data SecretData `json:"data,omitempty"`

	// Files using this secret
	FilesUsing PriorityFileSortedList `json:"files_using,omitempty"`
}

func (s *SecretState) Ratio() float64 {
	ratio := s.DurationRatio
	if ratio == 0 {
		ratio = DefaultSecretDurationRatio
	}
	return ratio
}

func (s *SecretState) TTL() (int, bool) {
	if s.Data == nil {
		return 0, false
	}
	switch ttlNumber := s.Data["ttl"].(type) {
	case json.Number:
		ttl, err := ttlNumber.Int64()
		if err != nil {
			return 0, false
		}
		return int(ttl), true
	case int:
		return ttlNumber, true
	case int64:
		return int(ttlNumber), true
	}
	return 0, false
}

func (s *SecretState) TimeToUpdate() (minTTU time.Time, known bool) {
	for _, source := range secretTTUSources {
		ttu, err := source(s)
		if err != nil {
			log.Printf("Error trying to obtain TTU for secret '%s': %s", s.Name, err)
			continue
		}
		if ttu != nil && (!known || ttu.Before(minTTU)) {
			minTTU = *ttu
			known = true
		}
	}
	return
}

func (s *SecretState) RegisterUsage(path string, priority int) {
	for _, f := range s.FilesUsing {
		if f.Path == path {
			// Already registered
			return
		}
	}
	s.FilesUsing = append(s.FilesUsing, PriorityFile{Priority: priority, Path: path})
	sort.Sort(s.FilesUsing)
}
