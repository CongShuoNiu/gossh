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

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	blackList = []string{"rm", "mkfs", "mkfs.ext3", "make.ext2", "make.ext4", "make2fs", "shutdown", "reboot", "init", "dd"}
	cmds      = []string{"rm -f /", "mkfs /dev/fioa", "shutdown now", "reboot"}
)

func TestCheckSafe(t *testing.T) {
	for _, cmd := range cmds {
		if CheckSafe(cmd, blackList) {
			t.Errorf("CheckSafe fail")
		}
	}

}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0600); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	if !FileExists(tmpFile) {
		t.Errorf("FileExists should return true for a regular file")
	}
	if FileExists(tmpDir) {
		t.Errorf("FileExists should return false for a directory")
	}
	if FileExists(filepath.Join(tmpDir, "missing.txt")) {
		t.Errorf("FileExists should return false for a missing file")
	}
}

func TestPathExists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0600); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	if !PathExists(tmpDir) {
		t.Errorf("PathExists should return true for a directory")
	}
	if PathExists(tmpFile) {
		t.Errorf("PathExists should return false for a regular file")
	}
}
