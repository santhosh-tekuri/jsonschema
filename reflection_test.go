package jsonschema

import (
	"strings"
	"testing"
)

func TestValidateArbitraryMap(t *testing.T) {
	schema, err := UnmarshalJSON(strings.NewReader(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name"]
	}`))
	if err != nil {
		t.Fatal(err)
	}

	c := NewCompiler()
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch := c.MustCompile("schema.json")

	// Test with map[string]string
	t.Run("map[string]string", func(t *testing.T) {
		data := map[string]string{
			"name": "Alice",
			"age":  "30",
		}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for wrong type, got nil")
		}
	})

	// Test with map[string]int
	t.Run("map[string]int", func(t *testing.T) {
		data := map[string]int{
			"name": 123,
			"age":  30,
		}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for wrong type, got nil")
		}
	})

	// Test with map[string]interface{} (should work)
	t.Run("map[string]interface{}", func(t *testing.T) {
		data := map[string]interface{}{
			"name": "Alice",
			"age":  30,
		}
		err := sch.Validate(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test with arbitrary map with missing required field
	t.Run("missing required field", func(t *testing.T) {
		data := map[string]interface{}{
			"age": 30,
		}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for missing required field, got nil")
		}
	})
}

func TestValidateArbitrarySlice(t *testing.T) {
	schema, err := UnmarshalJSON(strings.NewReader(`{
		"type": "array",
		"items": {"type": "number"},
		"minItems": 2
	}`))
	if err != nil {
		t.Fatal(err)
	}

	c := NewCompiler()
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch := c.MustCompile("schema.json")

	// Test with []int
	t.Run("[]int", func(t *testing.T) {
		data := []int{1, 2, 3}
		err := sch.Validate(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test with []float64
	t.Run("[]float64", func(t *testing.T) {
		data := []float64{1.5, 2.5, 3.5}
		err := sch.Validate(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test with []string (should fail type check)
	t.Run("[]string", func(t *testing.T) {
		data := []string{"a", "b", "c"}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for wrong type, got nil")
		}
	})

	// Test with []interface{} (should work)
	t.Run("[]interface{}", func(t *testing.T) {
		data := []interface{}{1, 2.5, 3}
		err := sch.Validate(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test with []int with minItems violation
	t.Run("minItems violation", func(t *testing.T) {
		data := []int{1}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for minItems violation, got nil")
		}
	})
}

func TestValidateArbitraryMapWithInteger(t *testing.T) {
	schema, err := UnmarshalJSON(strings.NewReader(`{
		"type": "object",
		"properties": {
			"count": {"type": "integer"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	c := NewCompiler()
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch := c.MustCompile("schema.json")

	// Test with map containing int
	t.Run("map with int", func(t *testing.T) {
		data := map[string]int{
			"count": 42,
		}
		err := sch.Validate(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test with map containing float (should fail integer check)
	t.Run("map with float", func(t *testing.T) {
		data := map[string]float64{
			"count": 42.5,
		}
		err := sch.Validate(data)
		if err == nil {
			t.Fatal("expected error for non-integer, got nil")
		}
	})
}
