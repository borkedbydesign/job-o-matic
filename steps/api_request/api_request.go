package api_request

import (
	"context"
	"fmt"
	"io"
	"job-o-matic/registry"
	"net/http"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type ApiStep struct {
	Method  string         `hcl:"method"`
	Url     hcl.Expression `hcl:"url"`
	Body    hcl.Expression `hcl:"body,optional"`
	Output  string         `hcl:"output"`
	Headers hcl.Expression `hcl:"headers,optional"`
}

func valueToString(val cty.Value, field string) (string, error) {
	if val.IsNull() {
		return "", fmt.Errorf("%s is null", field)
	}
	if !val.IsKnown() {
		return "", fmt.Errorf("%s not resolved yet", field)
	}
	if val.Type() != cty.String {
		return "", fmt.Errorf("%s must be a string, got %s", field, val.Type().FriendlyName())
	}
	return val.AsString(), nil
}

func (s *ApiStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Url.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	url, err := valueToString(val, "url")
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if s.Body != nil {
		val, diags = s.Body.Value(evalCtx)
		if diags.HasErrors() {
			return nil, diags
		}
		if !val.IsNull() {
			bodyStr, err := valueToString(val, "body")
			if err != nil {
				return nil, err
			}
			bodyReader = strings.NewReader(bodyStr)
		}
	}

	req, err := http.NewRequestWithContext(ctx, s.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if s.Headers != nil {
		headersVal, diags := s.Headers.Value(evalCtx)
		if diags.HasErrors() {
			return nil, diags
		}
		if !headersVal.IsNull() {
			if !headersVal.CanIterateElements() {
				return nil, fmt.Errorf("headers must be an object/map, got %s", headersVal.Type().FriendlyName())
			}
			for k, v := range headersVal.AsValueMap() {
				headerVal, err := valueToString(v, fmt.Sprintf("header %q", k))
				if err != nil {
					return nil, err
				}
				req.Header.Set(k, headerVal)
			}
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := cty.ObjectVal(map[string]cty.Value{
		"status_code": cty.NumberIntVal(int64(resp.StatusCode)),
		"body":        cty.StringVal(string(body)),
	})
	return map[string]cty.Value{
		s.Output: result,
	}, nil
}

func init() {
	registry.RegisterStep("api_request", func() registry.Runner {
		return &ApiStep{}
	})
}
