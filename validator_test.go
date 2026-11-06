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

func TestPropertyNamesLocation(t *testing.T) {
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"type": "object",
		"properties": {
			"foo": {
				"type": "object",
				"propertyNames": {"pattern": "^[a-z]+$"}
			}
		}
	}`))
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

	err = sch.Validate(map[string]any{
		"foo": map[string]any{"BAR": "baz"},
	})
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}

	out := err.(*jsonschema.ValidationError).BasicOutput()

	want := map[string]string{
		"/properties/foo/propertyNames":         "/foo",
		"/properties/foo/propertyNames/pattern": "/foo",
	}
	got := map[string]string{}
	for _, u := range out.Errors {
		got[u.KeywordLocation] = u.InstanceLocation
	}
	for kwLoc, instLoc := range want {
		actual, ok := got[kwLoc]
		if !ok {
			t.Errorf("missing unit with keywordLocation %q; got units %v", kwLoc, got)
			continue
		}
		if actual != instLoc {
			t.Errorf("keywordLocation %q: instanceLocation = %q, want %q", kwLoc, actual, instLoc)
		}
	}
	if _, doubled := got["/properties/foo/propertyNames/propertyNames"]; doubled {
		t.Errorf("keywordLocation has duplicated /propertyNames; got units %v", got)
	}
}
