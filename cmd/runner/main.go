package main

import (
	"flag"
	"fmt"
	"io"
	"job-o-matic/engine"
	"os"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func printEvents(w io.Writer, r *engine.ChanReporter, debug bool) {
	for e := range r.Events() {
		switch e.Status {
		case engine.StatusLevelStarted:
			fmt.Fprintf(w, "==> level %d starting\n", e.Level)
		case engine.StatusStepStarted:
			fmt.Fprintf(w, "  -> %s (%s) started\n", e.StepId, e.StepType)
		case engine.StatusStepSucceeded:
			fmt.Fprintf(w, "  ok   %s\n", e.StepId)
			if debug {
				printOutputs(w, e.Outputs)
			}
		case engine.StatusStepFailed:
			fmt.Fprintf(w, "  FAIL %s: %v\n", e.StepId, e.Err)
		case engine.StatusStepCancelled:
			fmt.Fprintf(w, "  skip %s (cancelled)\n", e.StepId)
		case engine.StatusStepSkipped:
			fmt.Fprintf(w, "  skip %s (%v)\n", e.StepId, e.Err)
		case engine.StatusLevelDone:
			fmt.Fprintf(w, "==> level %d done\n", e.Level)
		}
	}
}

func printOutputs(w io.Writer, outputs map[string]cty.Value) {
	for k, v := range outputs {
		b, err := ctyjson.Marshal(v, v.Type())
		if err != nil {
			fmt.Fprintf(w, "       %s: <unprintable: %v>\n", k, err)
			continue
		}
		fmt.Fprintf(w, "       %s = %s\n", k, string(b))
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: job-o-matic run <file> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -debug")
	fmt.Fprintln(os.Stderr, "        Print step outputs")
}

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}

	filename := args[0]

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	debug := fs.Bool("debug", false, "print step outputs")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open %q: %v\n", filename, err)
		return 1
	}

	reporter := engine.NewChanReporter(64)

	done := make(chan struct{})
	go func() {
		printEvents(os.Stdout, reporter, *debug)
		close(done)
	}()

	err = engine.RunFile(filename, content, reporter)

	reporter.Close()
	<-done

	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		return 1
	}

	return 0
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var code int

	switch os.Args[1] {
	case "run":
		code = run(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		code = 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		code = 2
	}

	os.Exit(code)
}
