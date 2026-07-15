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

package run

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andesli/gossh/config"
	"github.com/andesli/gossh/machine"
	"github.com/andesli/gossh/output"
)

var (
	user = "root"
	port = "22"
	psw  = ""
	cmd  = "uname"
)

func TestSingleRun(t *testing.T) {
	user := NewUser(user, port, psw, true, false)
	t.Log(user)
}

func TestConcurrency(t *testing.T) {
	if got := concurrency(make(chan struct{}, 3)); got != 3 {
		t.Fatalf("concurrency() = %d, want 3", got)
	}
	if got := concurrency(make(chan struct{})); got != 1 {
		t.Fatalf("concurrency() = %d, want fallback 1", got)
	}
}

func TestTimeoutDuration(t *testing.T) {
	if got := timeoutDuration(2); got != 2*time.Second {
		t.Fatalf("timeoutDuration() = %s, want 2s", got)
	}
	if got := timeoutDuration(0); got != time.Duration(outputTimeoutForTest())*time.Second {
		t.Fatalf("timeoutDuration() = %s, want default timeout", got)
	}
}

func TestRunParallelHostsRespectsConcurrency(t *testing.T) {
	hosts := []config.Host{{Ip: "1"}, {Ip: "2"}, {Ip: "3"}, {Ip: "4"}}
	var running int32
	var maxRunning int32

	runParallelHosts(hosts, 2, 1, func(ctx context.Context, h config.Host) machine.Result {
		current := atomic.AddInt32(&running, 1)
		for {
			max := atomic.LoadInt32(&maxRunning)
			if current <= max || atomic.CompareAndSwapInt32(&maxRunning, max, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return machine.Result{Ip: h.Ip, Cmd: "test", Result: "ok"}
	})

	if got := atomic.LoadInt32(&maxRunning); got > 2 {
		t.Fatalf("runParallelHosts max concurrency = %d, want <= 2", got)
	}
}

func TestBatchSummarySkipped(t *testing.T) {
	summary := newBatchSummary(3)
	summary.add("host-a", nil)
	summary.add("host-b", context.DeadlineExceeded)

	if got := summary.skipped(); got != 1 {
		t.Fatalf("summary.skipped() = %d, want 1", got)
	}
	if summary.ok() {
		t.Fatalf("summary.ok() should be false when a host failed")
	}
}

// outputTimeoutForTest 返回 output.TIMEOUT，避免测试里散落魔法数字。
func outputTimeoutForTest() int {
	return output.TIMEOUT
}
