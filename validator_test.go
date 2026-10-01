package jsonschema_test

import (
	"errors"
	"math"
	"slices"
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

func TestPropertyNames_InstanceLocation(t *testing.T) {
	c := jsonschema.NewCompiler()
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"items": {"propertyNames": {"enum": ["ok"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}

	// The first item has a bad property name; the second is valid.
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`[{"bad": 1}, {"ok": 1}]`))
	if err != nil {
		t.Fatal(err)
	}
	err = sch.Validate(inst)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *jsonschema.ValidationError, got %T", err)
	}
	if len(ve.Causes) == 0 {
		t.Fatal("expected causes")
	}
	for _, cause := range ve.Causes {
		if !slices.Equal(cause.InstanceLocation, []string{"0"}) {
			t.Errorf("got InstanceLocation %v, want [0]", cause.InstanceLocation)
		}
	}
}

func TestContentSchema_InstanceLocation(t *testing.T) {
	c := jsonschema.NewCompiler()
	c.AssertContent()
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"items": {"contentMediaType": "application/json", "contentSchema": {"type": "integer"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}

	// The first item has an invalid contentSchema; the second is valid.
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`["\"bad\"", "1"]`))
	if err != nil {
		t.Fatal(err)
	}
	err = sch.Validate(inst)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *jsonschema.ValidationError, got %T", err)
	}
	if len(ve.Causes) == 0 {
		t.Fatal("expected causes")
	}
	for _, cause := range ve.Causes {
		if !slices.Equal(cause.InstanceLocation, []string{"0"}) {
			t.Errorf("got InstanceLocation %v, want [0]", cause.InstanceLocation)
		}
	}
}

