package jsonschema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type rejectingLoader struct{}

func (rejectingLoader) Load(rawURL string) (any, error) {
	return nil, fmt.Errorf("unexpected load of %s", rawURL)
}

func compileSchema(t *testing.T, source string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func jsonValue(t *testing.T, source string) any {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDependenciesKeywordFollowsDeclaredDraft(t *testing.T) {
	t.Run("2020-12 does not compile annotation contents", func(t *testing.T) {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
			"$schema":"https://json-schema.org/draft/2020-12/schema",
			"dependencies":{"note":{"$schema":"https://invalid.example/schema"}}
		}`))
		if err != nil {
			t.Fatal(err)
		}
		compiler := jsonschema.NewCompiler()
		compiler.UseLoader(rejectingLoader{})
		if err := compiler.AddResource("schema.json", doc); err != nil {
			t.Fatal(err)
		}
		schema, err := compiler.Compile("schema.json")
		if err != nil {
			t.Fatalf("inactive dependencies content was compiled: %v", err)
		}
		if err := schema.Validate(jsonValue(t, `{"note":1}`)); err != nil {
			t.Fatalf("inactive dependencies keyword was enforced: %v", err)
		}
	})

	t.Run("2020-12 reference can target annotation content", func(t *testing.T) {
		schema := compileSchema(t, `{
			"$schema":"https://json-schema.org/draft/2020-12/schema",
			"dependencies":{"value":{"type":"string"}},
			"$ref":"#/dependencies/value"
		}`)
		if err := schema.Validate(jsonValue(t, `1`)); err == nil {
			t.Fatal("referenced annotation schema was not enforced")
		}
	})

	t.Run("draft-07 retains assertion behavior", func(t *testing.T) {
		schema := compileSchema(t, `{
			"$schema":"http://json-schema.org/draft-07/schema#",
			"dependencies":{"x":["y"]}
		}`)
		if err := schema.Validate(jsonValue(t, `{"x":1}`)); err == nil {
			t.Fatal("draft-07 dependency was not enforced")
		}
	})
}
