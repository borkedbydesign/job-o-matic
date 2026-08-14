package main

import (
	"context"
	"errors"
	"fmt"
	"job-o-matic/engine"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/urfave/cli/v3"
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
func run(filename string, workdir string, debug bool, varsRaw []string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to open %q: %v\n", filename, err))
	}
	if workdir != "" {
		if err := os.Chdir(workdir); err != nil {
			return errors.New(fmt.Sprintf("failed to chdir %q: %v\n", workdir, err))
		}
	}

	vars := parseVars(varsRaw)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	reporter := engine.NewChanReporter(64)
	done := make(chan struct{})
	go func() {
		runWorkflow(reporter, debug)
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
		return errors.New(fmt.Sprintf("run failed: %v\n", runErr))
	}
	return nil
}

func main() {
	cmd := &cli.Command{
		Name:                  "job-o-matic",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:    "run",
				Aliases: []string{"r"},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "variables for the workflow",
					},
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "enable debug mode",
					},
					&cli.StringFlag{
						Name:  "workdir",
						Usage: "set working directory",
					},
				},
				Usage: "run a workflow",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					run(cmd.Args().Get(0), cmd.String("workdir"), cmd.Bool("debug"), cmd.StringSlice("var"))
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
