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

package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/andesli/gossh/machine"
)

const (
	TIMEOUT = 4500
)

// new print result
func Print(res machine.Result) {
	fmt.Printf("ip=%s\n", res.Ip)
	//index := strings.Index(cmd, ";")
	//newcmd := cmd[index+1:]
	//fmt.Printf("ip=%s|command=%s\n", ip, cmd)
	fmt.Printf("command=%s\n", res.Cmd)
	if res.Err != nil {
		fmt.Printf("return=1\n")
		fmt.Printf("%s\n", res.Err)
	} else {
		fmt.Printf("return=0\n")
		fmt.Printf("%s\n", res.Result)
	}
	printTransferStats(res.Bytes, res.Duration)
	fmt.Println("----------------------------------------------------------")
}

// PrintBatchSummary 输出批量操作的聚合结果，帮助运维人员快速判断影响面。
func PrintBatchSummary(total, success, skipped int, failedHosts []string) {
	fmt.Printf("summary_total=%d\n", total)
	fmt.Printf("summary_success=%d\n", success)
	fmt.Printf("summary_failed=%d\n", len(failedHosts))
	fmt.Printf("summary_skipped=%d\n", skipped)
	if len(failedHosts) > 0 {
		fmt.Printf("summary_failed_hosts=%s\n", strings.Join(failedHosts, ","))
	} else {
		fmt.Printf("summary_failed_hosts=\n")
	}
	fmt.Println("----------------------------------------------------------")
}

// print push file result
func PrintPushResult(ip, src, dst string, err error) {
	fmt.Println("ip=", ip)
	fmt.Println("command=", "scp "+src+" root@"+ip+":"+dst)
	if err != nil {
		fmt.Printf("return=1\n")
		fmt.Println(err)
	} else {
		fmt.Printf("return=0\n")
		fmt.Printf("Push %s to %s ok.\n", src, dst)
	}
	fmt.Println("----------------------------------------------------------")
}

// print pull result
func PrintPullResult(ip, src, dst string, err error, bytes int64, duration time.Duration) {
	fmt.Println("ip=", ip)
	fmt.Println("command=", "scp "+" root@"+ip+":"+dst+" "+src)
	if err != nil {
		fmt.Printf("return=1\n")
		fmt.Println(err)
	} else {
		fmt.Printf("return=0\n")
		fmt.Printf("Pull from %s to %s ok.\n", dst, src)
	}
	printTransferStats(bytes, duration)
	fmt.Println("----------------------------------------------------------")
}

// printTransferStats 输出 SCP 传输字节数、耗时和平均吞吐，便于观察大文件传输表现。
func printTransferStats(bytes int64, duration time.Duration) {
	if bytes <= 0 && duration <= 0 {
		return
	}

	fmt.Printf("transfer_bytes=%d\n", bytes)
	fmt.Printf("transfer_duration=%s\n", duration.Truncate(time.Millisecond))
	fmt.Printf("transfer_throughput=%s/s\n", formatBytesPerSecond(bytes, duration))
}

// formatBytesPerSecond 将字节数和耗时格式化为人类可读的平均吞吐。
func formatBytesPerSecond(bytes int64, duration time.Duration) string {
	if bytes <= 0 || duration <= 0 {
		return "0 B"
	}
	return formatBytes(float64(bytes) / duration.Seconds())
}

// formatBytes 将字节值格式化为 B/KB/MB/GB/TB。
func formatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := bytes
	unit := units[0]
	for i := 1; i < len(units) && value >= 1024; i++ {
		value = value / 1024
		unit = units[i]
	}
	if unit == "B" {
		return fmt.Sprintf("%.0f %s", value, unit)
	}
	return fmt.Sprintf("%.2f %s", value, unit)
}
