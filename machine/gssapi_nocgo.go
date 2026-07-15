// Copyright 2018 github.com/andesli/gossh Author. All Rights Reserved.
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

//go:build !cgo
// +build !cgo

package machine

import gossh "golang.org/x/crypto/ssh"

// tryGSSAPIAuth 非 CGO 环境下 GSSAPI 不可用。
func tryGSSAPIAuth(targetHost string) gossh.AuthMethod {
	return nil
}

// canAuthWithGSSAPI 非 CGO 环境下 GSSAPI 不可用。
func canAuthWithGSSAPI(targetHost string) bool {
	return false
}
