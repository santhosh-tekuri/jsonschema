package jsonschema

import "testing"

func TestCompiler_AddResourceJSON(t *testing.T) {
	c := NewCompiler()
	err := c.AddResourceJSON("main.json", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":   "string",
				"format": "uuid",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("main.json")
	if err := sch.Validate(map[string]any{"id": "00000000-0000-0000-0000-000000000000"}); err != nil {
		t.Fatal(err)
	}
}
