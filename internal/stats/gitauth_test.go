package stats

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/justwaters/sitrep/internal/transport"
)

func TestGitAuthChecker_LoggedIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script shim not supported on windows")
	}
	fakeGH(t, 0, "✓ Logged in to github.com account someuser (keyring)\n")

	c := NewGitAuthChecker()
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var result transport.GitAuthResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.LoggedIn {
		t.Error("expected LoggedIn=true")
	}
	if result.Account != "someuser" {
		t.Errorf("Account = %q, want someuser", result.Account)
	}
}

func TestGitAuthChecker_NotLoggedIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script shim not supported on windows")
	}
	fakeGH(t, 1, "You are not logged into any GitHub hosts\n")

	c := NewGitAuthChecker()
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var result transport.GitAuthResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.LoggedIn {
		t.Error("expected LoggedIn=false")
	}
}

func TestGitAuthChecker_MissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewGitAuthChecker()
	if _, err := c.Collect(context.Background()); err == nil {
		t.Error("expected an error when gh is not in PATH")
	}
}

// fakeGH installs a shell shim named `gh` on PATH for the duration of the
// test, exiting with the given code and writing output to stderr (like
// the real `gh auth status`).
func fakeGH(t *testing.T, exitCode int, output string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncat <<'SITREP_EOF' 1>&2\n" + output + "SITREP_EOF\nexit " + strconv.Itoa(exitCode) + "\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
