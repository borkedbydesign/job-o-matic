package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"job-o-matic/registry"
	"os"
	"os/exec"

	"github.com/hashicorp/hcl/v2"
	"github.com/kballard/go-shellquote"
	"github.com/zclconf/go-cty/cty"
)

type ShellStep struct {
	Command hcl.Expression `hcl:"command"`
	Output  string         `hcl:"output"`
}

func (s *ShellStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Command.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	command := val.AsString()

	parts, err := shellquote.Split(command)
	if err != nil {
		return nil, fmt.Errorf("invalid command %q: %w", command, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	// stream to console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// also capture stdout/stderr for returning
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		} else {
			// non-exit-error (spawn/exec failure, context cancel, etc.)
			return nil, fmt.Errorf("failed to run command %q: %w", command, runErr)
		}

		// return error details (not a hard failure)
		result := cty.ObjectVal(map[string]cty.Value{
			"exit_code": cty.NumberIntVal(int64(exitCode)),
			"stdout":    cty.StringVal(stdoutBuf.String()),
			"stderr":    cty.StringVal(stderrBuf.String()),
		})
		return map[string]cty.Value{s.Output: result}, nil
	}

	result := cty.ObjectVal(map[string]cty.Value{
		"exit_code": cty.NumberIntVal(0),
		"stdout":    cty.StringVal(stdoutBuf.String()),
		"stderr":    cty.StringVal(stderrBuf.String()),
	})
	return map[string]cty.Value{s.Output: result}, nil
}

func init() {
	registry.RegisterStep("shell", func() registry.Runner {
		return &ShellStep{}
	})
}
