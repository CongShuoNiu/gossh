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
