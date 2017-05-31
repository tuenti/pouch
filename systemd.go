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
	"fmt"
	"log"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/util"
)

type SystemD interface {
	IsAvailable() bool
	UnitName() (string, error)
	Close()

	ReloadNotifier
	ReadyNotifier
}

func NewSystemd(c SystemdConfig) SystemD {
	return &systemd{
		enabled:     c.Enabled,
		autoRestart: c.AutoRestart,
	}
}

type systemd struct {
	enabled     *bool
	autoRestart *bool

	name string
}

func (s *systemd) Close() {
}

func (s *systemd) getName() (string, error) {
	if s.name != "" {
		return s.name, nil
	}
	name, err := util.CurrentUnitName()
	if err != nil {
		return "", err
	}
	s.name = name
	return s.name, nil
}

func (s *systemd) UnitName() (string, error) {
	return s.getName()
}

func (s *systemd) IsAvailable() bool {
	if s.enabled != nil && !*s.enabled {
		return false
	}

	if !util.IsRunningSystemd() {
		log.Printf("systemd is not running")
	}

	name, err := s.getName()
	if err != nil {
		log.Printf("couldn't obtain current unit name: %v", err)
		return false
	}
	if !strings.HasSuffix(name, ".service") {
		log.Printf("process is not started from a service unit, unit name found: %s", name)
		return false
	}

	log.Printf("systemd available, unit name: %s\n", name)

	return true
}

func (s *systemd) NotifyReady() error {
	sent, err := daemon.SdNotify(false, "READY=1")
	if err != nil {
		return fmt.Errorf("couldn't notify ready: %v", err)
	}
	if !sent {
		return fmt.Errorf("ready notification to systemd was not sent")
	}
	return nil
}

func (s *systemd) NotifyReload() error {
	sent, err := daemon.SdNotify(false, "RELOADING=1")
	if err != nil {
		return fmt.Errorf("couldn't notify reload: %v", err)
	}
	if !sent {
		return fmt.Errorf("reload notification to dbus was not sent")
	}
	return nil
}

func (s *systemd) AutoRestart() error {
	if s.autoRestart == nil || !*s.autoRestart {
		return nil
	}

	c, err := dbus.New()
	if err != nil {
		return err
	}

	name, _ := s.getName()
	propagateToProp, err := c.GetUnitProperty(name, "PropagatesReloadTo")
	if err != nil {
		return err
	}

	units, ok := propagateToProp.Value.Value().([]string)
	if !ok {
		return fmt.Errorf("couldn't convert property to units list (value: %+v)", propagateToProp.Value.Value())
	}

	errors := false
	results := make([]chan string, len(units))
	for i, unit := range units {
		results[i] = make(chan string, 1)
		_, err := c.ReloadOrRestartUnit(unit, "replace", results[i])
		if err != nil {
			log.Printf("error reloading %s: %v\n", unit, err)
			errors = true
		}
	}
	for i, result := range results {
		if r := <-result; r != "done" {
			log.Printf("reload job for %s is not done (found: %s)", units[i], r)
			errors = true
		}
	}
	if errors {
		return fmt.Errorf("there were errors when restarting units")
	}
	return nil
}
