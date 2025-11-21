//go:build e2e

package helpers

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type RunResult struct {
	Stdout, Stderr string
	Err            error
}

func RunCLI(ctx context.Context, extraEnv []string, args ...string) RunResult {
	// Adjust for your platform/arch naming scheme
	binPath := filepath.Join("..", "..", "bin", "veriflow")

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = append(os.Environ(), extraEnv...)

	var out, err bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &err

	runErr := cmd.Run()
	return RunResult{
		Stdout: out.String(),
		Stderr: err.String(),
		Err:    runErr,
	}
}

func TestCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
