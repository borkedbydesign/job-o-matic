package engine

import (
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var jsonEncodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "value",
			Type:             cty.DynamicPseudoType,
			AllowNull:        true,
			AllowDynamicType: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		b, err := ctyjson.Marshal(args[0], args[0].Type())
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(string(b)), nil
	},
})

func workflowFunctions() map[string]function.Function {
	return map[string]function.Function{
		"jsonencode": jsonEncodeFunc,
	}
}
