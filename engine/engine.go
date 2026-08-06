package engine

import (
	"context"
	"fmt"
	"job-o-matic/registry"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"

	"job-o-matic/model"
)

type StepOutcome string

const (
	OutcomeSucceeded StepOutcome = "succeeded"
	OutcomeFailed    StepOutcome = "failed"
	OutcomeSkipped   StepOutcome = "skipped"
)

type StepResult struct {
	Id      string
	Outputs map[string]cty.Value
	Err     error
	Outcome StepOutcome
}

func EvalContextForWorkflow(w *model.Workflow, variables map[string]string) (*hcl.EvalContext, map[string]cty.Value) {
	vars := make(map[string]cty.Value)
	for _, v := range w.Variables {
		if val, ok := variables[v.Name]; ok {
			vars[v.Name] = cty.StringVal(val)
		} else {
			vars[v.Name] = cty.StringVal(v.Default)
		}
	}
	steps := make(map[string]cty.Value)
	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var":   cty.ObjectVal(vars),
			"steps": cty.ObjectVal(steps),
		},
		Functions: workflowFunctions(),
	}, steps
}

func RunFile(filename string, src []byte, reporter Reporter, variables map[string]string) error {
	cfg := &model.Config{}
	if err := hclsimple.Decode(filename, src, nil, cfg); err != nil {
		return err
	}

	for _, workflow := range cfg.Workflows {
		ctx, steps := EvalContextForWorkflow(workflow, variables)
		stepStatus := map[string]StepOutcome{}

		levels, err := BuildLevels(workflow.Steps)
		if err != nil {
			return fmt.Errorf("error Building Levels %q", err)
		}

		for levelIdx, level := range levels {
			reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, Status: StatusLevelStarted})

			g, gctx := errgroup.WithContext(context.Background())
			results := make(chan StepResult, len(level))

			for _, step := range level {
				step := step
				g.Go(func() error {
					reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, StepType: step.Type, Status: StatusStepStarted})

					select {
					case <-gctx.Done():
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepCancelled, Err: gctx.Err()})
						results <- StepResult{Id: step.Id, Err: gctx.Err(), Outcome: OutcomeFailed}
						return gctx.Err()
					default:
					}

					// cascade: if anything this step depends on didn't succeed
					// (skipped OR failed), skip this step too — never reaches
					// its own `when` or decodes its body.
					for _, dep := range step.DependsOn {
						if stepStatus[dep] != OutcomeSucceeded {
							reason := fmt.Errorf("dependency %q was %s", dep, stepStatus[dep])
							reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepSkipped, Err: reason})
							results <- StepResult{Id: step.Id, Outcome: OutcomeSkipped}
							return nil
						}
					}

					whenVal, diags := step.When.Value(ctx)
					if diags.HasErrors() {
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepFailed, Err: diags})
						results <- StepResult{Id: step.Id, Err: diags, Outcome: OutcomeFailed}
						return diags
					}
					if !whenVal.IsNull() {
						if whenVal.Type() != cty.Bool {
							err := fmt.Errorf("when condition for step %q did not evaluate to a bool (got %s)", step.Id, whenVal.Type().FriendlyName())
							reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepFailed, Err: err})
							results <- StepResult{Id: step.Id, Err: err, Outcome: OutcomeFailed}
							return err
						}
						if !whenVal.True() {
							reason := fmt.Errorf("condition evaluated false")
							reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepSkipped, Err: reason})
							results <- StepResult{Id: step.Id, Outcome: OutcomeSkipped}
							return nil
						}
					}

					factory, ok := registry.GetFactory(step.Type)
					if !ok {
						err := fmt.Errorf("unknown step type %q", step.Type)
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepFailed, Err: err})
						results <- StepResult{Id: step.Id, Err: err, Outcome: OutcomeFailed}
						return err
					}

					runner := factory()

					if diags := gohcl.DecodeBody(step.Remain, nil, runner); diags.HasErrors() {
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepFailed, Err: diags})
						results <- StepResult{Id: step.Id, Err: diags, Outcome: OutcomeFailed}
						return diags
					}

					outputs, err := runner.Run(gctx, ctx)
					outcome := OutcomeSucceeded
					if err != nil {
						outcome = OutcomeFailed
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepFailed, Err: err})
					} else {
						reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, StepId: step.Id, Status: StatusStepSucceeded, Outputs: outputs})
					}
					results <- StepResult{Id: step.Id, Outputs: outputs, Err: err, Outcome: outcome}
					return err
				})
			}

			_ = g.Wait()
			close(results)

			for res := range results {
				stepStatus[res.Id] = res.Outcome
				if res.Outcome == OutcomeSucceeded && res.Outputs != nil {
					steps[res.Id] = cty.ObjectVal(res.Outputs)
				}
			}
			ctx.Variables["steps"] = cty.ObjectVal(steps)

			reporter.Emit(Event{Workflow: workflow.Id, Level: levelIdx, Status: StatusLevelDone})
		}
	}

	return nil
}
