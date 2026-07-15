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

//go:build cgo
// +build cgo

package machine

/*
#cgo LDFLAGS: -framework GSS
#include <gssapi/gssapi.h>
#include <gssapi/gssapi_krb5.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	gossh "golang.org/x/crypto/ssh"
)

// nativeGSSClient 使用系统 GSSAPI 库实现 ssh.GSSAPIClient 接口。
type nativeGSSClient struct {
	ctx     C.gss_ctx_id_t
	cred    C.gss_cred_id_t
	name    C.gss_name_t
	service string
}

// newGSSAPIClient 创建基于系统 GSSAPI 的认证客户端。
func newGSSAPIClient(targetHost string) (*nativeGSSClient, error) {
	var minor C.OM_uint32
	var major C.OM_uint32

	gc := &nativeGSSClient{
		ctx:     C.GSS_C_NO_CONTEXT,
		cred:    C.GSS_C_NO_CREDENTIAL,
		name:    C.GSS_C_NO_NAME,
		service: fmt.Sprintf("host@%s", targetHost),
	}

	// 导入服务名称
	cservice := C.CString(gc.service)
	defer C.free(unsafe.Pointer(cservice))

	buf := C.gss_buffer_desc{
		length: C.size_t(len(gc.service)),
		value:  unsafe.Pointer(cservice),
	}

	// macOS 上 GSS.framework 不导出 GSS_C_NT_HOSTBASED_SERVICE 符号，
	// 使用硬编码的 OID 值 {1, 3, 6, 1, 5, 6, 2}（DER 编码）构造。
	oidData := C.CBytes([]byte{0x2b, 0x06, 0x01, 0x05, 0x06, 0x02})
	defer C.free(oidData)
	hostBasedOID := C.gss_OID_desc{
		length:   6,
		elements: oidData,
	}

	major = C.gss_import_name(&minor, &buf, &hostBasedOID, &gc.name)
	if major != C.GSS_S_COMPLETE {
		return nil, fmt.Errorf("gssapi: gss_import_name failed: %s", gssError(major, minor))
	}

	return gc, nil
}

// InitSecContext 发起 GSS-API 安全上下文。
func (g *nativeGSSClient) InitSecContext(target string, token []byte, isGSSDelegCreds bool) ([]byte, bool, error) {
	var minor C.OM_uint32
	var major C.OM_uint32
	var outBuf C.gss_buffer_desc
	var inBuf C.gss_buffer_desc

	outBuf.length = 0
	outBuf.value = nil

	// CGo 不允许将 Go 指针嵌入 C 结构体传入 C 函数，需要拷贝到 C 内存。
	var cToken unsafe.Pointer
	if token != nil && len(token) > 0 {
		cToken = C.CBytes(token)
		defer C.free(cToken)
		inBuf.length = C.size_t(len(token))
		inBuf.value = cToken
	} else {
		inBuf.length = 0
		inBuf.value = nil
	}

	// RFC 4462 第 3.4 节要求：必须设置 mutual_req_flag 和 integ_req_flag。
	// GSS_C_MUTUAL_FLAG = 2, GSS_C_INTEG_FLAG = 32, GSS_C_DELEG_FLAG = 1
	flags := C.OM_uint32(C.GSS_C_MUTUAL_FLAG | C.GSS_C_INTEG_FLAG)
	if isGSSDelegCreds {
		flags |= C.GSS_C_DELEG_FLAG
	}

	major = C.gss_init_sec_context(
		&minor,
		g.cred,
		&g.ctx,
		g.name,
		C.GSS_C_NO_OID,
		flags,
		0,
		C.GSS_C_NO_CHANNEL_BINDINGS,
		&inBuf,
		nil,
		&outBuf,
		nil,
		nil,
	)

	if major != C.GSS_S_COMPLETE && major != C.GSS_S_CONTINUE_NEEDED {
		return nil, false, fmt.Errorf("gssapi: gss_init_sec_context failed: %s", gssError(major, minor))
	}

	var result []byte
	if outBuf.length > 0 {
		result = C.GoBytes(outBuf.value, C.int(outBuf.length))
		C.gss_release_buffer(&minor, &outBuf)
	}

	return result, major == C.GSS_S_CONTINUE_NEEDED, nil
}

// GetMIC 生成 GSS-API MIC token。
func (g *nativeGSSClient) GetMIC(micField []byte) ([]byte, error) {
	var minor C.OM_uint32
	var major C.OM_uint32
	var micBuf C.gss_buffer_desc

	// CGo 不允许将 Go 指针嵌入 C 结构体传入 C 函数，需要拷贝到 C 内存。
	cMicField := C.CBytes(micField)
	defer C.free(cMicField)

	inBuf := C.gss_buffer_desc{
		length: C.size_t(len(micField)),
		value:  cMicField,
	}

	major = C.gss_get_mic(&minor, g.ctx, C.GSS_C_QOP_DEFAULT, &inBuf, &micBuf)
	if major != C.GSS_S_COMPLETE {
		return nil, fmt.Errorf("gssapi: gss_get_mic failed: %s", gssError(major, minor))
	}

	result := C.GoBytes(micBuf.value, C.int(micBuf.length))
	C.gss_release_buffer(&minor, &micBuf)
	return result, nil
}

// DeleteSecContext 释放 GSS-API 安全上下文。
func (g *nativeGSSClient) DeleteSecContext() error {
	var minor C.OM_uint32

	if g.ctx != C.GSS_C_NO_CONTEXT {
		C.gss_delete_sec_context(&minor, &g.ctx, C.GSS_C_NO_BUFFER)
		g.ctx = C.GSS_C_NO_CONTEXT
	}
	if g.name != C.GSS_C_NO_NAME {
		C.gss_release_name(&minor, &g.name)
		g.name = C.GSS_C_NO_NAME
	}
	if g.cred != C.GSS_C_NO_CREDENTIAL {
		C.gss_release_cred(&minor, &g.cred)
		g.cred = C.GSS_C_NO_CREDENTIAL
	}

	return nil
}

// tryGSSAPIAuth 尝试使用系统 GSSAPI 认证，返回 ssh.AuthMethod。
func tryGSSAPIAuth(targetHost string) gossh.AuthMethod {
	gc, err := newGSSAPIClient(targetHost)
	if err != nil {
		return nil
	}
	return gossh.GSSAPIWithMICAuthMethod(gc, targetHost)
}

// canAuthWithGSSAPI 检查系统 GSSAPI 是否可用。
func canAuthWithGSSAPI(targetHost string) bool {
	return tryGSSAPIAuth(targetHost) != nil
}

// gssError 将 GSS-API 错误码转换为可读错误信息。
func gssError(major, minor C.OM_uint32) string {
	var msgBuf C.gss_buffer_desc
	var minorStat C.OM_uint32
	var msgCtx C.OM_uint32

	C.gss_display_status(&minorStat, major, C.GSS_C_GSS_CODE, C.GSS_C_NO_OID, &msgCtx, &msgBuf)
	msg := C.GoStringN((*C.char)(msgBuf.value), C.int(msgBuf.length))
	C.gss_release_buffer(&minorStat, &msgBuf)

	return msg
}
