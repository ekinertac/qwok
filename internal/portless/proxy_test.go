package portless

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProxyRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTLESS_STATE_DIR", dir)

	// No pid file -> not running.
	if ProxyRunning() {
		t.Fatal("ProxyRunning should be false with no proxy.pid")
	}

	// A live pid (our own process) -> running.
	if err := os.WriteFile(filepath.Join(dir, "proxy.pid"), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ProxyRunning() {
		t.Fatal("ProxyRunning should be true for a live pid")
	}

	// A pid that cannot exist -> not running (stale pid file self-heals).
	if err := os.WriteFile(filepath.Join(dir, "proxy.pid"), []byte("2147483640"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ProxyRunning() {
		t.Fatal("ProxyRunning should be false for a dead pid")
	}
}
