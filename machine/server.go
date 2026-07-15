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
// Author: andes
// Email: email.tata@qq.com

package machine

import (
	"context"
	"errors"
	"fmt"

	"github.com/andesli/gossh/auth"
	_ "github.com/andesli/gossh/auth/db"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	//_ "github.com/andesli/gossh/auth/web"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/andesli/gossh/logs"
	"github.com/andesli/gossh/scp"
	"github.com/andesli/gossh/tools"
)

var (
	PASSWORD_SOURCE = "db"
	//PASSWORD_SOURCE   = "web"

	NO_PASSWORD = "GET PASSWORD ERROR\n"

	log = logs.NewLogger()

	hostKeyConfig = HostKeyConfig{}
)

const (
	NO_EXIST = "0"
	IS_FILE  = "1"
	IS_DIR   = "2"
)

type Server struct {
	Ip         string
	Port       string
	User       string
	Psw        string
	Action     string
	Cmd        string
	FileName   string
	RemotePath string
	Force      bool
	Timeout    int
}

type ScpConfig struct {
	Src string
	Dst string
}

type Result struct {
	Ip       string
	Cmd      string
	Result   string
	Err      error
	Bytes    int64
	Duration time.Duration
}

type HostKeyConfig struct {
	KnownHostsPath        string
	InsecureIgnoreHostKey bool
}

// ConfigureHostKey 设置全局 SSH host key 校验策略，默认使用 known_hosts 校验。
func ConfigureHostKey(knownHostsPath string, insecureIgnoreHostKey bool) {
	hostKeyConfig = HostKeyConfig{
		KnownHostsPath:        knownHostsPath,
		InsecureIgnoreHostKey: insecureIgnoreHostKey,
	}
}

func NewCmdServer(ip, port, user, psw, action, cmd string, force bool, timeout int) *Server {
	server := &Server{
		Ip:      ip,
		Port:    port,
		User:    user,
		Action:  action,
		Cmd:     cmd,
		Psw:     psw,
		Force:   force,
		Timeout: timeout,
	}
	if psw == "" {
		server.SetPsw()
		//log.Debug("server.Psw=%s", server.Psw)
	}
	return server
}

func NewScpServer(ip, port, user, psw, action, file, rpath string, force bool, timeout int) *Server {
	rfile := path.Join(rpath, path.Base(file))
	cmd := createShell(rfile)
	server := &Server{
		Ip:         ip,
		Port:       port,
		User:       user,
		Psw:        psw,
		Action:     action,
		FileName:   file,
		RemotePath: rpath,
		Cmd:        cmd,
		Force:      force,
		Timeout:    timeout,
	}
	if psw == "" {
		server.SetPsw()
	}
	return server
}
func NewPullServer(ip, port, user, psw, action, file, rpath string, force bool, timeout int) *Server {
	cmd := createShell(rpath)
	server := &Server{
		Ip:         ip,
		Port:       port,
		User:       user,
		Psw:        psw,
		Action:     action,
		FileName:   file,
		RemotePath: rpath,
		Cmd:        cmd,
		Force:      force,
		Timeout:    timeout,
	}
	if psw == "" {
		server.SetPsw()
	}
	return server
}

/*
func NewScp(src, dst string) ScpConfig {
	scp := ScpConfig{
		Src: src,
		Dst: dst,
	}
	return scp
}
*/

// query password from password plugin
// PASSWORD_SOURCE: db|web
func (server *Server) SetPsw() {
	psw, err := auth.GetPassword(PASSWORD_SOURCE, server.Ip, server.User)
	if err != nil {
		server.Psw = NO_PASSWORD
		return
	}
	server.Psw = psw
}

// run command for parallel
func (server *Server) PRunCmd(crs chan Result) {
	rs := server.SRunCmdContext(context.Background())
	crs <- rs
}

// PRunCmdContext 在指定上下文内并行执行命令，超时或取消时会尽快关闭 SSH 会话。
func (server *Server) PRunCmdContext(ctx context.Context, crs chan Result) {
	rs := server.SRunCmdContext(ctx)
	crs <- rs
}

// set Server.Cmd
func (s *Server) SetCmd(cmd string) {
	s.Cmd = cmd
}

// run command in sequence
func (server *Server) RunCmd() (result string, err error) {
	return server.RunCmdContext(context.Background())
}

// RunCmdContext 在指定上下文内执行远端命令，取消时会关闭会话释放连接资源。
func (server *Server) RunCmdContext(ctx context.Context) (result string, err error) {
	if server.Psw == NO_PASSWORD && !canAuthWithGSSAPI(server.Ip) {
		return NO_PASSWORD, nil
	}
	client, err := server.getSshClient()
	if err != nil {
		return "getSSHClient error", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "newSession error", err
	}
	defer session.Close()

	cmd := tools.EnhanceListCommand(server.Cmd)
	bs, err := combinedOutputContext(ctx, client, session, cmd)
	if err != nil {
		return string(bs), err
	}
	return string(bs), nil
}

// run command in sequence
func (server *Server) SRunCmd() Result {
	return server.SRunCmdContext(context.Background())
}

// SRunCmdContext 顺序执行远端命令，并将上下文取消转换为主机级失败结果。
func (server *Server) SRunCmdContext(ctx context.Context) Result {
	rs := Result{
		Ip:  server.Ip,
		Cmd: server.Cmd,
	}

	if server.Psw == NO_PASSWORD && !canAuthWithGSSAPI(server.Ip) {
		rs.Err = errors.New(NO_PASSWORD)
		return rs
	}

	client, err := server.getSshClient()
	if err != nil {
		rs.Err = err
		return rs
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		rs.Err = err
		return rs
	}
	defer session.Close()

	cmd := tools.EnhanceListCommand(server.Cmd)
	bs, err := combinedOutputContext(ctx, client, session, cmd)
	if err != nil {
		rs.Err = err
		return rs
	}
	rs.Result = string(bs)
	return rs
}

// execute a single command on remote server
func (server *Server) checkRemoteFile() (result string) {
	re, _ := server.RunCmd()
	return re
}

// checkRemoteFileContext 在指定上下文内检查远端文件类型，供可取消的 SCP 流程使用。
func (server *Server) checkRemoteFileContext(ctx context.Context) (string, error) {
	return server.RunCmdContext(ctx)
}

// PRunScp() can transport  file or path to remote host
func (server *Server) PRunScp(crs chan Result) {
	server.PRunScpContext(context.Background(), crs)
}

// PRunScpContext 在指定上下文内并行执行 SCP 推送，避免超时任务长期占用并发槽。
func (server *Server) PRunScpContext(ctx context.Context, crs chan Result) {
	cmd := "push " + server.FileName + " to " + server.Ip + ":" + server.RemotePath
	rs := Result{
		Ip:  server.Ip,
		Cmd: cmd,
	}
	stats, err := server.RunScpDirContext(ctx)
	rs.Bytes = stats.Bytes
	rs.Duration = stats.Duration
	if err != nil {
		rs.Err = err
	} else {
		rs.Result = cmd + " ok\n"
	}
	crs <- rs
}

func (server *Server) RunScpDir() (err error) {
	_, err = server.RunScpDirContext(context.Background())
	return err
}

// RunScpDirContext 将文件或目录推送到远端，并在上下文取消时尽快返回错误。
func (server *Server) RunScpDirContext(ctx context.Context) (scp.TransferStats, error) {
	result, err := server.checkRemoteFileContext(ctx)
	if err != nil {
		return scp.TransferStats{}, err
	}
	re := strings.TrimSpace(result)
	log.Debug("server.checkRemoteFile()=%s\n", re)

	//远程机器存在同名文件
	if re == IS_FILE && server.Force == false {
		errString := "<ERROR>\nRemote Server's " + server.RemotePath + " has the same file " + server.FileName + "\nYou can use `-f` option force to cover the remote file.\n</ERROR>\n"
		return scp.TransferStats{}, errors.New(errString)
	}

	rfile := server.RemotePath
	cmd := createShell(rfile)
	server.SetCmd(cmd)
	result, err = server.checkRemoteFileContext(ctx)
	if err != nil {
		return scp.TransferStats{}, err
	}
	re = strings.TrimSpace(result)
	log.Debug("server.checkRemoteFile()=%s\n", re)

	//远程目录不存在
	if re != IS_DIR {
		errString := "[" + server.Ip + ":" + server.RemotePath + "] does not exist or not a dir\n"
		return scp.TransferStats{}, errors.New(errString)
	}

	client, err := server.getSshClient()
	if err != nil {
		return scp.TransferStats{}, err
	}
	defer client.Close()
	go closeClientWhenDone(ctx, client)

	filename := server.FileName
	fi, err := os.Stat(filename)
	if err != nil {
		log.Debug("open source file %s error\n", filename)
		return scp.TransferStats{}, err
	}
	scpClient := scp.NewScp(client)
	if fi.IsDir() {
		return scpClient.PushDir(filename, server.RemotePath)
	}
	return scpClient.PushFile(filename, server.RemotePath)
}

// pull file from remote to local server
func (server *Server) PullScp() (err error) {
	_, err = server.PullScpContext(context.Background())
	return err
}

// PullScpContext 将远端普通文件拉取到本地目录，并支持通过上下文取消传输。
func (server *Server) PullScpContext(ctx context.Context) (scp.TransferStats, error) {

	//判断远程源文件情况
	result, err := server.checkRemoteFileContext(ctx)
	if err != nil {
		return scp.TransferStats{}, err
	}
	re := strings.TrimSpace(result)
	log.Debug("server.checkRemoteFile()=%s\n", re)

	//不存在报错
	if re == NO_EXIST {
		errString := "Remote Server's " + server.RemotePath + " doesn't exist.\n"
		return scp.TransferStats{}, errors.New(errString)
	}

	//不支持拉取目录
	if re == IS_DIR {
		errString := "Remote Server's " + server.RemotePath + " is a directory ,not support.\n"
		return scp.TransferStats{}, errors.New(errString)
	}

	//仅仅支持普通文件
	if re != IS_FILE {
		errString := "Get info from Remote Server's " + server.RemotePath + " error.\n"
		return scp.TransferStats{}, errors.New(errString)
	}

	//本地目录
	dst := server.FileName
	//远程文件
	src := server.RemotePath

	log.Debug("src=%s", src)
	log.Debug("dst=%s", dst)

	//本地路径不存在，自动创建
	err = tools.MakePath(dst)
	if err != nil {
		return scp.TransferStats{}, err
	}

	//检查本地是否有同名文件
	fileName := path.Base(src)
	localFile := filepath.Join(dst, fileName)

	flag := tools.FileExists(localFile)
	log.Debug("flag=%v", flag)
	log.Debug("localFile=%s", localFile)

	//-f 可以强制覆盖
	if flag && !server.Force {
		return scp.TransferStats{}, errors.New(localFile + " is exist, use -f to cover the old file")
	}

	//执行pull
	client, err := server.getSshClient()
	if err != nil {
		return scp.TransferStats{}, err
	}
	defer client.Close()
	go closeClientWhenDone(ctx, client)

	scpClient := scp.NewScp(client)
	return scpClient.PullFile(dst, src)
}

// RunScp1() only can transport  file to remote host
func (server *Server) RunScpFile() (result string, err error) {
	client, err := server.getSshClient()
	if err != nil {
		return "GetSSHClient Error\n", err
	}
	defer client.Close()

	filename := server.FileName
	session, err := client.NewSession()
	if err != nil {
		return "Create SSHSession Error", err
	}
	defer session.Close()

	go func() {
		Buf := make([]byte, 1024)
		w, _ := session.StdinPipe()
		defer w.Close()
		//File, err := os.Open(filepath.Abs(filename))
		File, err := os.Open(filename)
		if err != nil {
			log.Debug("open scp source file %s error\n", filename)
			return
		}
		defer File.Close()

		info, _ := File.Stat()
		newname := filepath.Base(filename)
		fmt.Fprintln(w, "C0644", info.Size(), newname)
		for {
			n, err := File.Read(Buf)
			fmt.Fprint(w, string(Buf[:n]))
			if err != nil {
				if err == io.EOF {
					// transfer end with \x00
					fmt.Fprint(w, "\x00")
					return
				} else {
					fmt.Println("read scp source file error")
					return
				}
			}
		}
	}()

	cmd := "/usr/bin/scp -qt " + server.RemotePath
	bs, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(bs), err
	}
	return string(bs), nil
}

// getSshClient 建立 SSH 连接，认证优先级为 GSSAPI > keyboard-interactive > password。
func (server *Server) getSshClient() (client *ssh.Client, err error) {
	authMethods := []ssh.AuthMethod{}

	// 优先尝试 GSSAPI（Kerberos）认证
	if gssapiAuth := tryGSSAPIAuth(server.Ip); gssapiAuth != nil {
		authMethods = append(authMethods, gssapiAuth)
	}

	keyboardInteractiveChallenge := func(
		user,
		instruction string,
		questions []string,
		echos []bool,
	) (answers []string, err error) {

		if len(questions) == 0 {
			return []string{}, nil
		}
		/*
			for i, question := range questions {
				log.Debug("SSH Question %d: %s", i+1, question)
			}
		*/

		answers = make([]string, len(questions))
		for i := range questions {
			if strings.Contains(strings.ToLower(questions[i]), "yes") {
				answers[i] = "yes"

			} else {
				answers[i] = server.Psw
			}
		}
		return answers, nil
	}
	authMethods = append(authMethods, ssh.KeyboardInteractive(keyboardInteractiveChallenge))
	authMethods = append(authMethods, ssh.Password(server.Psw))

	hostKeyCallback, err := newHostKeyCallback(hostKeyConfig)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            server.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         (time.Duration(server.Timeout)) * time.Second,
	}
	//psw := []ssh.AuthMethod{ssh.Password(server.Psw)}
	//Conf := ssh.ClientConfig{User: server.User, Auth: psw}
	ip_port := server.Ip + ":" + server.Port
	client, err = ssh.Dial("tcp", ip_port, sshConfig)
	return
}

// combinedOutputContext 执行远端命令，并在 ctx 取消时关闭 session/client 来中断阻塞调用。
func combinedOutputContext(ctx context.Context, client *ssh.Client, session *ssh.Session, cmd string) ([]byte, error) {
	type sessionResult struct {
		output []byte
		err    error
	}

	done := make(chan sessionResult, 1)
	go func() {
		bs, err := session.CombinedOutput(cmd)
		done <- sessionResult{output: bs, err: err}
	}()

	select {
	case result := <-done:
		return result.output, result.err
	case <-ctx.Done():
		_ = session.Close()
		_ = client.Close()
		return nil, ctx.Err()
	}
}

// closeClientWhenDone 在上下文取消后关闭 SSH 连接，用于中断 SCP 传输等阻塞操作。
func closeClientWhenDone(ctx context.Context, client *ssh.Client) {
	done := ctx.Done()
	if done == nil {
		return
	}
	<-done
	_ = client.Close()
}

// newHostKeyCallback 根据配置构建 SSH host key 校验回调。
func newHostKeyCallback(cfg HostKeyConfig) (ssh.HostKeyCallback, error) {
	if cfg.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	path := cfg.KnownHostsPath
	if path == "" {
		path = defaultKnownHostsPath()
	}
	if !tools.FileExists(path) {
		return nil, fmt.Errorf("known_hosts file %s does not exist; create it with ssh-keyscan or use -insecure-ignore-host-key for trusted networks", path)
	}
	return knownhosts.New(path)
}

// defaultKnownHostsPath 返回当前用户默认的 known_hosts 文件路径。
func defaultKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".ssh/known_hosts"
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// create shell script for running on remote server
func createShell(file string) string {
	s1 := "bash << EOF \n"
	s2 := "if [[ -f " + shellQuote(file) + " ]];then \n"
	s3 := "echo '1'\n"
	s4 := "elif [[ -d " + shellQuote(file) + " ]];then \n"
	s5 := `echo "2"
else 
echo "0"
fi
EOF`
	cmd := s1 + s2 + s3 + s4 + s5
	return cmd
}

// shellQuote 对远端 shell 参数做单引号转义，避免路径中的空格或特殊字符被解释。
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
