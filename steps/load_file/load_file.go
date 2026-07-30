package load_file

import (
	"context"
	"fmt"
	"job-o-matic/registry"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type LoadFileStep struct {
	Location hcl.Expression `hcl:"location"`
	Output   string         `hcl:"output"`
}

func (s *LoadFileStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Location.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	path := val.AsString()

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open file:", err)
		os.Exit(1)
	}

	result := cty.ObjectVal(map[string]cty.Value{
		"body": cty.StringVal(string(content)),
	})

	return map[string]cty.Value{
		s.Output: result,
	}, nil
}

func init() {
	registry.RegisterStep("load_file", func() registry.Runner {
		return &LoadFileStep{}
	})
}
