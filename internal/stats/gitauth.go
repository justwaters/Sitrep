package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/justwaters/sitrep/internal/transport"
)

// GitAuthChecker reports whether the `gh` CLI is currently authenticated,
// by running `gh auth status` and inspecting its exit code and output.
type GitAuthChecker struct{}

func NewGitAuthChecker() *GitAuthChecker { return &GitAuthChecker{} }

func (c *GitAuthChecker) Name() string { return transport.CheckGitAuth }

var ghAccountRE = regexp.MustCompile(`account\s+(\S+)`)

func (c *GitAuthChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // gh auth status writes its human-readable status to stderr
	runErr := cmd.Run()

	result := transport.GitAuthResult{LoggedIn: runErr == nil}
	if runErr == nil {
		if m := ghAccountRE.FindStringSubmatch(out.String()); len(m) == 2 {
			result.Account = m[1]
		}
	}
	return json.Marshal(result)
}
