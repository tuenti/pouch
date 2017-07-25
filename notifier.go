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
	"fmt"
	"log"
	"os/exec"
	"time"
)

const (
	DefaultNotifyTimeout = 5 * time.Minute
)

type NotifierRunner interface {
	Run(context.Context) (string, error)
}

type ServiceNotifier struct {
	Reloader

	Service string
}

func (n *ServiceNotifier) Run(ctx context.Context) (string, error) {
	err := n.Reload(ctx, n.Service)
	return "", err
}

type CommandNotifier struct {
	Command string
}

func (n *CommandNotifier) Run(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", n.Command)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (p *pouch) notifierRunner(config NotifierConfig) (NotifierRunner, error) {
	var runner NotifierRunner

	count := 0
	if config.Service != "" {
		if p.Reloader == nil {
			return nil, fmt.Errorf("service set for notifier, but not service reloader available")
		}
		runner = &ServiceNotifier{Reloader: p.Reloader, Service: config.Service}
		count++
	}

	if config.Command != "" {
		runner = &CommandNotifier{Command: config.Command}
		count++
	}

	if count != 1 {
		return nil, fmt.Errorf("one and only one notifier option can be set")
	}

	return runner, nil
}

func (p *pouch) Notify(name string) {
	notifier, found := p.Notifiers[name]
	if !found {
		log.Printf("Couldn't find notifier for '%s'", name)
		return
	}

	runner, err := p.notifierRunner(notifier)
	if err != nil {
		log.Printf("Couldn't configure notifier for '%s': %v", name, err)
		return
	}

	timeout := DefaultNotifyTimeout
	if notifier.Timeout != "" {
		t, err := time.ParseDuration(notifier.Timeout)
		if err == nil {
			timeout = t
		} else {
			log.Printf("Incorrect timeout: %s", err)
		}
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)

	out, err := runner.Run(ctx)
	if err != nil {
		log.Printf("Notification to '%s' failed: %s", name, err)
		if len(out) > 0 {
			log.Println(string(out))
		}
	}
}
