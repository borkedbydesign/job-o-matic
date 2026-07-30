package write_file

import (
	"context"
	"fmt"
	"job-o-matic/registry"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type WriteFileStep struct {
	Location hcl.Expression `hcl:"location"`
	Content  hcl.Expression `hcl:"content"`
	Output   string         `hcl:"output"`
}

func (s *WriteFileStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Location.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	path := val.AsString()

	contentVal, diags := s.Content.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if contentVal.Type() != cty.String {
		return nil, fmt.Errorf("content must be a string, got %s", contentVal.Type().FriendlyName())
	}
	content := contentVal.AsString()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file %q: %w", path, err)
	}

	result := cty.ObjectVal(map[string]cty.Value{
		"body": cty.StringVal(content),
	})

	return map[string]cty.Value{
		s.Output: result,
	}, nil
}

func init() {
	registry.RegisterStep("write_file", func() registry.Runner {
		return &WriteFileStep{}
	})
}
