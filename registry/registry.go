package registry

import (
	"context"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Runner interface {
	Run(ctx context.Context, evalCtx *hcl.EvalContext) (map[string]cty.Value, error)
}

type Factory func() Runner

var factories = map[string]Factory{}

func RegisterStep(typeName string, factory Factory) {
	factories[typeName] = factory
}

func GetFactory(typeName string) (Factory, bool) {
	f, ok := factories[typeName]
	return f, ok
}

func ListSteps() []string {
	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	return out
}
