package jsonschema_test

import (
	"strings"
	"testing"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Issue #261: an oversized numeric token must not force unbounded big.Rat work
// during validation of a schema with a numeric keyword. After the fix, such a
// token is rejected cheaply (validation returns an error) instead of being
// parsed into an arbitrary-precision big.Rat.
func TestIssue261_OversizedNumberRejected(t *testing.T) {
	schemaDoc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"minimum":0}`))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("memory://schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("memory://schema.json")
	if err != nil {
		t.Fatal(err)
	}

	// A normal-sized number still validates fine (does not trip the guard).
	ok, err := jsonschema.UnmarshalJSON(strings.NewReader("5"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(ok); err != nil {
		t.Fatalf("normal number should pass: %v", err)
	}

	// A pathological token is rejected, and rejected quickly.
	input := "0." + strings.Repeat("1", 200000)
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	verr := sch.Validate(inst)
	elapsed := time.Since(start)
	if verr == nil {
		t.Fatal("expected oversized number token to be rejected, got nil error")
	}
	if elapsed > 5*time.Millisecond {
		t.Fatalf("guard should reject cheaply; took %v", elapsed)
	}
	t.Logf("rejected oversized token in %v: %v", elapsed, verr)
}
