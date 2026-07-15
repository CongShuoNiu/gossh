package scp

import "testing"

// TestShellQuote 验证远端 shell 参数会被单引号包裹并正确转义内部单引号。
func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a path/it's.txt")
	want := `'/tmp/a path/it'\''s.txt'`
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}
