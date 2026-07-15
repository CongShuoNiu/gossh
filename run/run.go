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

package run

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/andesli/gossh/config"
	"github.com/andesli/gossh/logs"
	"github.com/andesli/gossh/machine"
	"github.com/andesli/gossh/output"
)

var (
	log = logs.NewLogger()
)

type CommonUser struct {
	user    string
	port    string
	psw     string
	force   bool
	encflag bool
}

type hostRunner func(context.Context, config.Host) machine.Result

type batchSummary struct {
	total       int
	success     int
	failedHosts []string
}

// NewUser 构造批量任务复用的 SSH 用户配置。
func NewUser(user, port, psw string, force, encflag bool) *CommonUser {
	return &CommonUser{
		user:    user,
		port:    port,
		psw:     psw,
		force:   force,
		encflag: encflag,
	}

}

// SingleRun 在单台机器上执行命令并输出结果，返回该主机是否执行成功。
func SingleRun(host, cmd string, cu *CommonUser, force bool, timeout int) bool {
	server := machine.NewCmdServer(host, cu.port, cu.user, cu.psw, "cmd", cmd, force, timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
	defer cancel()

	r := server.SRunCmdContext(ctx)
	output.Print(r)
	return r.Err == nil
}

// ServersRun 批量执行远端命令，并为每台机器设置独立超时和取消上下文。
func ServersRun(cmd string, cu *CommonUser, ipFile string, ccons chan struct{}, safe bool, timeout int) bool {
	hosts, err := parseIpfile(ipFile, cu)
	if err != nil {
		log.Error("Parse %s error, error=%s", ipFile, err)
		return false
	}

	ips := config.GetIps(hosts)

	//config.PrintHosts(hosts)
	log.Info("[servers]=%v", ips)
	fmt.Printf("[servers]=%v\n", ips)

	//ccons==1 串行执行,可以暂停
	if concurrency(ccons) == 1 {
		log.Debug("串行执行")
		summary := newBatchSummary(len(hosts))
		for _, h := range hosts {
			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
			server := machine.NewCmdServer(h.Ip, h.Port, h.User, h.Psw, "cmd", cmd, cu.force, timeout)
			r := server.SRunCmdContext(ctx)
			cancel()
			summary.addResult(r)
			if r.Err != nil && safe {
				log.Debug("%s执行出错", h.Ip)
				output.Print(r)
				output.PrintBatchSummary(summary.total, summary.success, summary.skipped(), summary.failedHosts)
				return summary.ok()
			} else {
				output.Print(r)
			}
		}
		output.PrintBatchSummary(summary.total, summary.success, summary.skipped(), summary.failedHosts)
		return summary.ok()
	} else {
		log.Debug("并行执行")
		return runParallelHosts(hosts, concurrency(ccons), timeout, func(ctx context.Context, h config.Host) machine.Result {
			server := machine.NewCmdServer(h.Ip, h.Port, h.User, h.Psw, "cmd", cmd, cu.force, timeout)
			return server.SRunCmdContext(ctx)
		})
	}
}

// SinglePush 在单台机器上执行文件或目录推送，返回该主机是否执行成功。
func SinglePush(ip, src, dst string, cu *CommonUser, f bool, timeout int) bool {
	server := machine.NewScpServer(ip, cu.port, cu.user, cu.psw, "scp", src, dst, f, timeout)
	cmd := "push " + server.FileName + " to " + server.Ip + ":" + server.RemotePath

	rs := machine.Result{
		Ip:  server.Ip,
		Cmd: cmd,
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
	defer cancel()

	stats, err := server.RunScpDirContext(ctx)
	rs.Bytes = stats.Bytes
	rs.Duration = stats.Duration
	if err != nil {
		rs.Err = err
	} else {
		rs.Result = cmd + " ok\n"
	}
	output.Print(rs)
	return rs.Err == nil
}

// ServersPush 批量推送本地文件或目录，并为每台机器设置独立超时和取消上下文。
func ServersPush(src, dst string, cu *CommonUser, ipFile string, ccons chan struct{}, timeout int) bool {
	hosts, err := parseIpfile(ipFile, cu)
	if err != nil {
		log.Error("Parse %s error, error=%s", ipFile, err)
		return false
	}

	ips := config.GetIps(hosts)
	log.Info("[servers]=%v", ips)
	fmt.Printf("[servers]=%v\n", ips)

	return runParallelHosts(hosts, concurrency(ccons), timeout, func(ctx context.Context, h config.Host) machine.Result {
		server := machine.NewScpServer(h.Ip, h.Port, h.User, h.Psw, "scp", src, dst, cu.force, timeout)
		cmd := "push " + server.FileName + " to " + server.Ip + ":" + server.RemotePath
		rs := machine.Result{
			Ip:  server.Ip,
			Cmd: cmd,
		}
		stats, err := server.RunScpDirContext(ctx)
		rs.Bytes = stats.Bytes
		rs.Duration = stats.Duration
		if err != nil {
			rs.Err = err
			return rs
		}
		rs.Result = cmd + " ok\n"
		return rs
	})
}

// SinglePull 在单台机器上拉取远端文件，返回该主机是否执行成功。
func SinglePull(host string, cu *CommonUser, src, dst string, force bool, timeout int) bool {
	server := machine.NewPullServer(host, cu.port, cu.user, cu.psw, "scp", src, dst, force, timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
	defer cancel()

	stats, err := server.PullScpContext(ctx)
	output.PrintPullResult(host, src, dst, err, stats.Bytes, stats.Duration)
	return err == nil
}

// ServersPull 批量拉取远端文件，并为每台机器设置独立超时和取消上下文。
func ServersPull(src, dst string, cu *CommonUser, ipFile string, force bool, timeout int) bool {
	hosts, err := parseIpfile(ipFile, cu)
	if err != nil {
		log.Error("Parse %s error, error=%s", ipFile, err)
		return false
	}
	ips := config.GetIps(hosts)
	log.Info("[servers]=%v", ips)
	fmt.Printf("[servers]=%v\n", ips)

	summary := newBatchSummary(len(hosts))
	for _, h := range hosts {
		ip := h.Ip

		localPath := filepath.Join(src, ip)
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
		server := machine.NewPullServer(h.Ip, h.Port, h.User, h.Psw, "scp", localPath, dst, cu.force, timeout)
		stats, pullErr := server.PullScpContext(ctx)
		cancel()
		output.PrintPullResult(ip, localPath, dst, pullErr, stats.Bytes, stats.Duration)
		summary.add(ip, pullErr)
	}
	output.PrintBatchSummary(summary.total, summary.success, summary.skipped(), summary.failedHosts)
	return summary.ok()
}

// parseIpfile 解析批量机器文件，并用命令行通用参数补齐缺省字段。
func parseIpfile(ipFile string, cu *CommonUser) ([]config.Host, error) {
	hosts, err := config.ParseIps(ipFile, cu.encflag)
	if err != nil {
		log.Error("Parse Ip File %s error,%s\n", ipFile, err)
		return hosts, err
	}

	if len(hosts) == 0 {
		return hosts, errors.New(ipFile + " is null")
	}
	hosts = config.PaddingHosts(hosts, cu.port, cu.user, cu.psw)
	return hosts, nil

}

// runParallelHosts 以固定并发度执行每台机器任务，并在任务完成或超时后释放并发槽。
func runParallelHosts(hosts []config.Host, cons int, timeout int, runner hostRunner) bool {
	results := make(chan machine.Result, len(hosts))
	sem := make(chan struct{}, cons)
	wg := sync.WaitGroup{}
	summary := newBatchSummary(len(hosts))

	for _, h := range hosts {
		host := h
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration(timeout))
			defer cancel()

			results <- runner(ctx, host)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for rs := range results {
		output.Print(rs)
		summary.addResult(rs)
	}
	output.PrintBatchSummary(summary.total, summary.success, summary.skipped(), summary.failedHosts)
	return summary.ok()
}

// concurrency 从旧的并发控制 channel 中提取并发度，并兜底为 1。
func concurrency(ccons chan struct{}) int {
	if cap(ccons) <= 0 {
		return 1
	}
	return cap(ccons)
}

// timeoutDuration 将命令行秒级超时转换为 Duration，0 值沿用历史兜底值。
func timeoutDuration(timeout int) time.Duration {
	if timeout <= 0 {
		timeout = output.TIMEOUT
	}
	return time.Duration(timeout) * time.Second
}

// newBatchSummary 初始化批量操作汇总。
func newBatchSummary(total int) *batchSummary {
	return &batchSummary{
		total:       total,
		failedHosts: make([]string, 0),
	}
}

// addResult 将一条主机执行结果计入批量汇总。
func (s *batchSummary) addResult(rs machine.Result) {
	s.add(rs.Ip, rs.Err)
}

// add 将一台目标主机的成功或失败计入批量汇总。
func (s *batchSummary) add(host string, err error) {
	if err != nil {
		s.failedHosts = append(s.failedHosts, host)
		return
	}
	s.success++
}

// ok 返回批量操作是否全部成功。
func (s *batchSummary) ok() bool {
	return len(s.failedHosts) == 0
}

// skipped 返回因 fail-fast 等原因未执行的目标主机数量。
func (s *batchSummary) skipped() int {
	return s.total - s.success - len(s.failedHosts)
}
