package json_extract

import (
	"context"
	"encoding/json"
	"fmt"
	"job-o-matic/registry"

	"github.com/PaesslerAG/jsonpath"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type JsonExtractStep struct {
	Input  hcl.Expression `hcl:"input"` // e.g. steps.fetch_user.response.body
	Path   string         `hcl:"path"`  // e.g. $.user.name
	Output string         `hcl:"output"`
}

func (s *JsonExtractStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Input.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if val.Type() != cty.String {
		return nil, fmt.Errorf("json_extract: input must resolve to a string, got %s", val.Type().FriendlyName())
	}

	var data interface{}
	if err := json.Unmarshal([]byte(val.AsString()), &data); err != nil {
		return nil, fmt.Errorf("json_extract: input is not valid JSON: %w", err)
	}

	result, err := jsonpath.Get(s.Path, data)
	if err != nil {
		return nil, fmt.Errorf("json_extract: path %q failed: %w", s.Path, err)
	}

	return map[string]cty.Value{
		s.Output: toCtyValue(result),
	}, nil
}

// toCtyValue converts arbitrary decoded JSON (from encoding/json) into cty.
func toCtyValue(v interface{}) cty.Value {
	switch t := v.(type) {
	case string:
		return cty.StringVal(t)
	case float64:
		return cty.NumberFloatVal(t)
	case bool:
		return cty.BoolVal(t)
	case nil:
		return cty.NullVal(cty.DynamicPseudoType)
	case map[string]interface{}:
		vals := make(map[string]cty.Value, len(t))
		for k, v := range t {
			vals[k] = toCtyValue(v)
		}
		return cty.ObjectVal(vals)
	case []interface{}:
		if len(t) == 0 {
			return cty.EmptyTupleVal
		}
		vals := make([]cty.Value, len(t))
		for i, v := range t {
			vals[i] = toCtyValue(v)
		}
		return cty.TupleVal(vals)
	default:
		b, _ := json.Marshal(t)
		return cty.StringVal(string(b))
	}
}

func init() {
	registry.RegisterStep("json_extract", func() registry.Runner {
		return &JsonExtractStep{}
	})
}
