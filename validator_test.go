package jsonschema_test

import (
	"math"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestInvalidFloats(t *testing.T) {
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"minimum": 0}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}

	values := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, v := range values {
		err := sch.Validate(v)
		if err == nil {
			t.Errorf("Validate(%v) = nil, want error", v)
		}
	}
}

func TestIssue275_UnparseableNumberNoPanic(t *testing.T) {
	schemas := []string{
		`{"minimum": 0}`,
		`{"maximum": 10}`,
		`{"exclusiveMinimum": 0}`,
		`{"exclusiveMaximum": 10}`,
		`{"multipleOf": 3}`,
	}
	for _, schemaJSON := range schemas {
		t.Run(schemaJSON, func(t *testing.T) {
			c := jsonschema.NewCompiler()
			doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
			if err != nil {
				t.Fatal(err)
			}
			if err := c.AddResource("schema.json", doc); err != nil {
				t.Fatal(err)
			}
			sch, err := c.Compile("schema.json")
			if err != nil {
				t.Fatal(err)
			}

			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`1e9999999`))
			if err != nil {
				t.Fatal(err)
			}
			if err := sch.Validate(inst); err == nil {
				t.Fatalf("Validate(1e9999999) = nil, want error")
			}

			okInst, err := jsonschema.UnmarshalJSON(strings.NewReader(`3`))
			if err != nil {
				t.Fatal(err)
			}
			if err := sch.Validate(okInst); err != nil {
				t.Fatalf("Validate(3) = %v, want nil", err)
			}
		})
	}
}
