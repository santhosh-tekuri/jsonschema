package jsonschema_test

import (
	"math"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileSchema(t *testing.T, raw string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
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
	return sch
}

func TestExactLengthError(t *testing.T) {
	// When minLength == maxLength, a mismatch should say "exactly N"
	// rather than "at least N" / "at most N". Fixes #247.
	sch := compileSchema(t, `{"minLength": 10, "maxLength": 10}`)

	err := sch.Validate("short") // 5 runes
	if err == nil {
		t.Fatal("Validate(short) = nil, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "want exactly 10") {
		t.Errorf("too-short exact length error = %q, want substring %q", msg, "want exactly 10")
	}

	long := strings.Repeat("x", 11)
	err = sch.Validate(long)
	if err == nil {
		t.Fatal("Validate(11-char) = nil, want error")
	}
	msg = err.Error()
	if !strings.Contains(msg, "want exactly 10") {
		t.Errorf("too-long exact length error = %q, want substring %q", msg, "want exactly 10")
	}

	if err := sch.Validate(strings.Repeat("x", 10)); err != nil {
		t.Errorf("Validate(10-char) = %v, want nil", err)
	}

	// minLength only still uses the open-ended message
	minOnly := compileSchema(t, `{"minLength": 10}`)
	err = minOnly.Validate("short")
	if err == nil {
		t.Fatal("minLength-only Validate = nil, want error")
	}
	msg = err.Error()
	if strings.Contains(msg, "exactly") {
		t.Errorf("minLength-only error = %q, did not want %q", msg, "exactly")
	}
	if !strings.Contains(msg, "want 10") {
		t.Errorf("minLength-only error = %q, want substring %q", msg, "want 10")
	}

	maxOnly := compileSchema(t, `{"maxLength": 10}`)
	err = maxOnly.Validate(strings.Repeat("x", 11))
	if err == nil {
		t.Fatal("maxLength-only Validate = nil, want error")
	}
	msg = err.Error()
	if strings.Contains(msg, "exactly") {
		t.Errorf("maxLength-only error = %q, did not want %q", msg, "exactly")
	}
}

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
