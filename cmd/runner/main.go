package main

import (
	"context"
	"fmt"
	"job-o-matic/engine"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func parseVars(raw []string) map[string]string {
	vars := make(map[string]string, len(raw))
	for _, kv := range raw {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			vars[k] = v
		}
	}
	return vars
}

func runWorkflow(r *engine.ChanReporter, debug bool) {
	p := mpb.New(mpb.WithWidth(1), mpb.WithOutput(os.Stdout))
	bars := make(map[string]*mpb.Bar)
	texts := make(map[string]*string)
	var debugLines []string
	for e := range r.Events() {
		switch e.Status {
		case engine.StatusLevelStarted, engine.StatusLevelDone:
		case engine.StatusStepStarted:
			text := "running"
			texts[e.StepId] = &text
			bar := p.AddSpinner(1,
				mpb.PrependDecorators(
					decor.Name(fmt.Sprintf("[L%d] %-24s (%s)", e.Level, e.StepId, e.StepType)),
				),
				mpb.AppendDecorators(decor.Any(func(decor.Statistics) string {
					return *texts[e.StepId]
				})),
			)
			bars[e.StepId] = bar
		case engine.StatusStepSucceeded:
			if t, ok := texts[e.StepId]; ok {
				*t = "ok"
			}
			if bar, ok := bars[e.StepId]; ok {
				bar.SetTotal(1, true)
			}
			if debug {
				for k, v := range e.Outputs {
					b, err := ctyjson.Marshal(v, v.Type())
					if err != nil {
						continue
					}
					debugLines = append(debugLines, fmt.Sprintf("       %s.%s = %s", e.StepId, k, string(b)))
				}
			}
		case engine.StatusStepFailed:
			if t, ok := texts[e.StepId]; ok {
				*t = fmt.Sprintf("FAILED: %v", e.Err)
			}
			if bar, ok := bars[e.StepId]; ok {
				bar.SetTotal(1, true)
			}
		case engine.StatusStepCancelled:
			if t, ok := texts[e.StepId]; ok {
				*t = "cancelled"
			}
			if bar, ok := bars[e.StepId]; ok {
				bar.SetTotal(1, true)
			}
		case engine.StatusStepSkipped:
			if t, ok := texts[e.StepId]; ok {
				*t = fmt.Sprintf("skipped: %v", e.Err)
			}
			if bar, ok := bars[e.StepId]; ok {
				bar.SetTotal(1, true)
			}
		}
	}
	p.Wait()

	for _, line := range debugLines {
		fmt.Fprintln(os.Stdout, line)
	}
}
func run(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: job-o-matic run <file> [flags]")
		return 2
	}
	filename := args[0]

	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	debug := fs.BoolP("debug", "d", false, "print step outputs")
	workdir := fs.StringP("workdir", "w", "", "working directory to run from (default: current)")
	varsRaw := fs.StringArrayP("var", "", nil, "set a variable KEY=VALUE (repeatable)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: job-o-matic run <file> [flags]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, fs.FlagUsages())
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open %q: %v\n", filename, err)
		return 1
	}
	if *workdir != "" {
		if err := os.Chdir(*workdir); err != nil {
			fmt.Fprintf(os.Stderr, "failed to chdir %q: %v\n", *workdir, err)
			return 1
		}
	}

	vars := parseVars(*varsRaw)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	reporter := engine.NewChanReporter(64)
	done := make(chan struct{})
	go func() {
		runWorkflow(reporter, *debug)
		close(done)
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.RunFile(filename, content, reporter, vars)
	}()

	var runErr error
	select {
	case runErr = <-errCh:
	case <-sigCtx.Done():
		fmt.Fprintln(os.Stderr, "interrupted, waiting for running steps to finish...")
		runErr = <-errCh
	}

	reporter.Close()
	<-done
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", runErr)
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: job-o-matic run <file> [flags]")
		os.Exit(2)
	}
	var code int
	switch os.Args[1] {
	case "run":
		code = run(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Fprintln(os.Stderr, "Usage: job-o-matic run <file> [flags]")
		code = 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		code = 2
	}
	os.Exit(code)
}
