package jsonschema_test

import (
	"math"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
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

// Regression: propertyNames/contentSchema retained vd.vloc directly, so a
// sibling's append could overwrite a retained error's InstanceLocation. The
// root/a/b/arr depth gives the array vloc spare capacity, so arr[1] clobbers
// arr[0]'s location unless it is cloned.
func TestPropertyNamesInstanceLocationNotMutated(t *testing.T) {
	schema := `{"properties":{"a":{"properties":{"b":{"properties":{
		"arr":{"items":{"propertyNames":{"pattern":"^[a-z]+$"}}}}}}}}}`
	instance := `{"a":{"b":{"arr":[{"BADKEY":1},{}]}}}`

	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schema))
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
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(instance))
	if err != nil {
		t.Fatal(err)
	}

	verr, ok := sch.Validate(inst).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("want a *jsonschema.ValidationError")
	}
	pn := findErrorByKind[*kind.PropertyNames](verr)
	if pn == nil {
		t.Fatal("no propertyNames error in tree")
	}
	if got := strings.Join(pn.InstanceLocation, "/"); got != "a/b/arr/0" {
		t.Errorf("InstanceLocation = %q, want %q", got, "a/b/arr/0")
	}
}

func findErrorByKind[T jsonschema.ErrorKind](v *jsonschema.ValidationError) *jsonschema.ValidationError {
	if _, ok := v.ErrorKind.(T); ok {
		return v
	}
	for _, c := range v.Causes {
		if r := findErrorByKind[T](c); r != nil {
			return r
		}
	}
	return nil
}
