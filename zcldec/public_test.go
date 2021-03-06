package zcldec

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-zcl/zcl"
	"github.com/zclconf/go-zcl/zcl/zclsyntax"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		config    string
		spec      Spec
		ctx       *zcl.EvalContext
		want      cty.Value
		diagCount int
	}{
		{
			``,
			&ObjectSpec{},
			nil,
			cty.EmptyObjectVal,
			0,
		},
		{
			"a = 1\n",
			&ObjectSpec{},
			nil,
			cty.EmptyObjectVal,
			1, // attribute named "a" is not expected here
		},
		{
			"a = 1\n",
			&ObjectSpec{
				"a": &AttrSpec{
					Name: "a",
					Type: cty.Number,
				},
			},
			nil,
			cty.ObjectVal(map[string]cty.Value{
				"a": cty.NumberIntVal(1),
			}),
			0,
		},
		{
			"a = 1\n",
			&AttrSpec{
				Name: "a",
				Type: cty.Number,
			},
			nil,
			cty.NumberIntVal(1),
			0,
		},
		{
			"a = \"1\"\n",
			&AttrSpec{
				Name: "a",
				Type: cty.Number,
			},
			nil,
			cty.NumberIntVal(1),
			0,
		},
		{
			"a = true\n",
			&AttrSpec{
				Name: "a",
				Type: cty.Number,
			},
			nil,
			cty.UnknownVal(cty.Number),
			1, // incorrect type - number required.
		},
		{
			``,
			&AttrSpec{
				Name:     "a",
				Type:     cty.Number,
				Required: true,
			},
			nil,
			cty.NullVal(cty.Number),
			1, // attribute "a" is required
		},

		{
			`
b {
}
`,
			&BlockSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
			},
			nil,
			cty.EmptyObjectVal,
			0,
		},
		{
			``,
			&BlockSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
			},
			nil,
			cty.NullVal(cty.DynamicPseudoType),
			0,
		},
		{
			"a {}\n",
			&BlockSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
			},
			nil,
			cty.NullVal(cty.DynamicPseudoType),
			1, // blocks of type "a" are not supported
		},
		{
			``,
			&BlockSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
				Required: true,
			},
			nil,
			cty.NullVal(cty.DynamicPseudoType),
			1, // a block of type "b" is required
		},
		{
			`
b {}
b {}
`,
			&BlockSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
				Required: true,
			},
			nil,
			cty.EmptyObjectVal,
			1, // only one "b" block is allowed
		},
		{
			`
b {}
b {}
`,
			&BlockListSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
			},
			nil,
			cty.ListVal([]cty.Value{cty.EmptyObjectVal, cty.EmptyObjectVal}),
			0,
		},
		{
			``,
			&BlockListSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
			},
			nil,
			cty.ListValEmpty(cty.DynamicPseudoType),
			0,
		},
		{
			`
b {}
b {}
b {}
`,
			&BlockListSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
				MaxItems: 2,
			},
			nil,
			cty.ListVal([]cty.Value{cty.EmptyObjectVal, cty.EmptyObjectVal, cty.EmptyObjectVal}),
			1, // too many b blocks
		},
		{
			`
b {}
b {}
`,
			&BlockListSpec{
				TypeName: "b",
				Nested:   ObjectSpec{},
				MinItems: 10,
			},
			nil,
			cty.ListVal([]cty.Value{cty.EmptyObjectVal, cty.EmptyObjectVal}),
			1, // insufficient b blocks
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d-%s", i, test.config), func(t *testing.T) {
			file, parseDiags := zclsyntax.ParseConfig([]byte(test.config), "", zcl.Pos{Line: 1, Column: 1, Byte: 0})
			body := file.Body
			got, valDiags := Decode(body, test.spec, test.ctx)

			var diags zcl.Diagnostics
			diags = append(diags, parseDiags...)
			diags = append(diags, valDiags...)

			if len(diags) != test.diagCount {
				t.Errorf("wrong number of diagnostics %d; want %d", len(diags), test.diagCount)
				for _, diag := range diags {
					t.Logf(" - %s", diag.Error())
				}
			}

			if !got.RawEquals(test.want) {
				t.Errorf("wrong result\ngot:  %#v\nwant: %#v", got, test.want)
			}
		})
	}
}

func TestSourceRange(t *testing.T) {
	tests := []struct {
		config string
		spec   Spec
		want   zcl.Range
	}{
		{
			"a = 1\n",
			&AttrSpec{
				Name: "a",
			},
			zcl.Range{
				Start: zcl.Pos{Line: 1, Column: 5, Byte: 4},
				End:   zcl.Pos{Line: 1, Column: 6, Byte: 5},
			},
		},
		{
			`
b {
  a = 1
}
`,
			&BlockSpec{
				TypeName: "b",
				Nested: &AttrSpec{
					Name: "a",
				},
			},
			zcl.Range{
				Start: zcl.Pos{Line: 3, Column: 7, Byte: 11},
				End:   zcl.Pos{Line: 3, Column: 8, Byte: 12},
			},
		},
		{
			`
b {
  c {
    a = 1
  }
}
`,
			&BlockSpec{
				TypeName: "b",
				Nested: &BlockSpec{
					TypeName: "c",
					Nested: &AttrSpec{
						Name: "a",
					},
				},
			},
			zcl.Range{
				Start: zcl.Pos{Line: 4, Column: 9, Byte: 19},
				End:   zcl.Pos{Line: 4, Column: 10, Byte: 20},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d-%s", i, test.config), func(t *testing.T) {
			file, diags := zclsyntax.ParseConfig([]byte(test.config), "", zcl.Pos{Line: 1, Column: 1, Byte: 0})
			if len(diags) != 0 {
				t.Errorf("wrong number of diagnostics %d; want %d", len(diags), 0)
				for _, diag := range diags {
					t.Logf(" - %s", diag.Error())
				}
			}
			body := file.Body

			got := SourceRange(body, test.spec)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("wrong result\ngot:  %#v\nwant: %#v", got, test.want)
			}
		})
	}

}
