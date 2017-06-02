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
	"io"
	"net/url"
	"os/exec"
)

type SSHSender struct {
	URL *url.URL
}

// Sender implemented over ssh, using ssh command and output
// redirection, for two main reasons: to make use of client
// ssh configuration and keys, and to avoid writing the secret
// to disk. It'd be nice to implement it with crypto/ssh.
func NewSSHSender(u *url.URL) *SSHSender {
	return &SSHSender{u}
}

func (s *SSHSender) Send(secret string) error {
	var sshArgs []string
	if s.URL.User != nil {
		sshArgs = append(sshArgs, "-l", s.URL.User.Username())
	}
	if s.URL.Port() != "" {
		sshArgs = append(sshArgs, "-p", s.URL.Port())
	}
	sshArgs = append(sshArgs, s.URL.Hostname())
	sshArgs = append(sshArgs, fmt.Sprintf("cat > '%s'", s.URL.Path))

	cmd := exec.Command("ssh", sshArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, secret)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh error: %s, output: %s", err, out)
	}
	return nil
}
