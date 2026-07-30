package api_request

import (
	"context"
	"io"
	"job-o-matic/registry"
	"log"
	"net/http"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type ApiStep struct {
	Method  string            `hcl:"method"`
	Url     hcl.Expression    `hcl:"url"`
	Output  string            `hcl:"output"`
	Headers map[string]string `hcl:"headers,optional"`
}

func (s *ApiStep) Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error) {
	val, diags := s.Url.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	url := val.AsString()

	req, err := http.NewRequestWithContext(ctx, s.Method, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
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
