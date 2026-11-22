package identity

import (
	"os"
	"testing"
)

func TestChainStartsAtPidAndClimbs(t *testing.T) {
	chain := Chain(os.Getpid(), 5)
	if len(chain) < 2 {
		t.Fatalf("chain = %+v, want this process and at least its parent", chain)
	}
	if chain[0].PID != os.Getpid() || chain[0].PPID != os.Getppid() {
		t.Fatalf("chain[0] = %+v, want pid %d ppid %d", chain[0], os.Getpid(), os.Getppid())
	}
	if chain[1].PID != chain[0].PPID {
		t.Fatalf("chain[1] = %+v does not follow chain[0] = %+v", chain[1], chain[0])
	}
	if chain[0].Name == "" {
		t.Fatalf("chain[0] has no name: %+v", chain[0])
	}
}

func TestIsClaude(t *testing.T) {
	for name, want := range map[string]bool{
		"claude": true, "/Users/u/.local/bin/claude": true, `C:\Users\u\.local\bin\claude.exe`: true,
		"Claude.EXE": true, "claude-code": false, "node.exe": false, "bash.exe": false,
	} {
		if got := IsClaude(name); got != want {
			t.Errorf("IsClaude(%q) = %v, want %v", name, got, want)
		}
	}
}
