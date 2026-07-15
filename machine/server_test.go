// Copyright 2018 gossh Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Author: andes
// Email: email.tata@qq.com

package machine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initServer 从环境变量构造集成测试使用的 SSH 服务端，避免单元测试依赖固定机器。
func initServer() *Server {
	s := &Server{
		Ip:     os.Getenv("GOSSH_TEST_SSH_HOST"),
		Port:   os.Getenv("GOSSH_TEST_SSH_PORT"),
		User:   os.Getenv("GOSSH_TEST_SSH_USER"),
		Action: "cmd",
		Cmd:    "uname",
		Psw:    os.Getenv("GOSSH_TEST_SSH_PASSWORD"),
	}

	if s.Port == "" {
		s.Port = "22"
	}

	return s
}

/*
func TestSetPsw(t *testing.T) {
	s := initServer()
	s.SetPsw()
	if s.Psw != "NO_PASSWORD" {
		t.Error("get password fail")
	}
}
*/

func TestRunCmd(t *testing.T) {
	if os.Getenv("GOSSH_TEST_SSH_HOST") == "" {
		t.Skip("set GOSSH_TEST_SSH_HOST to enable SSH integration test")
	}

	s := initServer()
	//	s.SetPsw()

	_, err := s.RunCmd()
	if err != nil {
		t.Fail()
	}
}

func TestCreateShellQuotesRemotePath(t *testing.T) {
	cmd := createShell("/tmp/a path/it's.txt")

	if want := `'/tmp/a path/it'\''s.txt'`; !strings.Contains(cmd, want) {
		t.Fatalf("createShell should quote remote path, want %q in %q", want, cmd)
	}
}

func TestNewHostKeyCallbackRequiresKnownHosts(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "known_hosts")

	if _, err := newHostKeyCallback(HostKeyConfig{KnownHostsPath: missing}); err == nil {
		t.Fatalf("newHostKeyCallback should fail when known_hosts is missing")
	}
}

func TestNewHostKeyCallbackAllowsExplicitInsecureMode(t *testing.T) {
	callback, err := newHostKeyCallback(HostKeyConfig{InsecureIgnoreHostKey: true})
	if err != nil {
		t.Fatalf("newHostKeyCallback insecure mode returned error: %v", err)
	}
	if callback == nil {
		t.Fatalf("newHostKeyCallback insecure mode returned nil callback")
	}
}

func TestDefaultKnownHostsPath(t *testing.T) {
	path := defaultKnownHostsPath()
	if !strings.HasSuffix(path, filepath.Join(".ssh", "known_hosts")) {
		t.Fatalf("defaultKnownHostsPath() = %q, want suffix .ssh/known_hosts", path)
	}
}
