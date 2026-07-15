package output

import (
	"testing"
	"time"
)

// TestFormatBytesPerSecond 验证大文件传输吞吐会格式化为可读单位。
func TestFormatBytesPerSecond(t *testing.T) {
	got := formatBytesPerSecond(2*1024*1024, time.Second)
	if got != "2.00 MB" {
		t.Fatalf("formatBytesPerSecond() = %q, want %q", got, "2.00 MB")
	}
}

// TestFormatBytesPerSecondZeroDuration 验证异常耗时不会产生除零或无效吞吐。
func TestFormatBytesPerSecondZeroDuration(t *testing.T) {
	got := formatBytesPerSecond(1024, 0)
	if got != "0 B" {
		t.Fatalf("formatBytesPerSecond() = %q, want %q", got, "0 B")
	}
}

// TestColorize 验证终端颜色包装会包含 ANSI 开始和重置序列。
func TestColorize(t *testing.T) {
	got := colorize("成功", colorGreen)
	want := colorGreen + "成功" + colorReset
	if got != want {
		t.Fatalf("colorize() = %q, want %q", got, want)
	}
}
