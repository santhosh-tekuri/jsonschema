package jsonschema_test

import (
	"log"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCustomTypes(t *testing.T) {
	schema := `{
  "type": "object",
  "properties": {
    "interfaces": {
      "type": "array",
      "items": { "type": "string" },
      "minItems": 2,
      "maxItems": 10
    },
    "test": {
      "type": "boolean"
    }
  }
}`

	compiler := jsonschema.NewCompiler()
	json, err := jsonschema.UnmarshalJSON(strings.NewReader(schema))
	if err != nil {
		log.Fatal(err)
	}

	err = compiler.AddResource("spec.json", json)
	if err != nil {
		log.Fatal(err)
	}

	type Custom bool

	values := map[string]any{
		"interfaces": []string{"eth0", "eth1"},
		"test":       Custom(true),
	}

	err = compiler.MustCompile("spec.json").Validate(values)
	if err != nil {
		log.Fatal(err)
	}
}
