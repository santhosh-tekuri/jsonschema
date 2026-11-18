package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Lookahead from JSON Schema pattern dialects that Go's regexp rejects.
const lookaheadPattern = `^((Agent|Bash|Edit)(\((?=.*[^)*?])[^)]+\))?|mcp__.*)$`

func TestEcmaCompileLookahead(t *testing.T) {
	if _, err := regexp.Compile(lookaheadPattern); err == nil {
		t.Fatal("stdlib regexp unexpectedly accepted lookahead")
	}
	re, err := ecmaCompile(lookaheadPattern)
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("Agent(ls)") {
		t.Fatal("Agent(ls) should match")
	}
	if re.MatchString("nope") {
		t.Fatal("nope should not match")
	}
}

func TestJVCompilerAcceptsLookaheadPattern(t *testing.T) {
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"type": "string",
		"pattern": "^((Agent|Bash|Edit)(\\((?=.*[^)*?])[^)]+\\))?|mcp__.*)$"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.UseRegexpEngine(ecmaCompile)
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := sch.Validate("Agent(ls)"); err != nil {
		t.Fatalf("valid instance: %v", err)
	}
	if err := sch.Validate("nope"); err == nil {
		t.Fatal("invalid instance accepted")
	}
}

func TestStdlibCompilerRejectsLookaheadPattern(t *testing.T) {
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"type": "string",
		"pattern": "^((Agent|Bash|Edit)(\\((?=.*[^)*?])[^)]+\\))?|mcp__.*)$"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Compile("schema.json"); err == nil {
		t.Fatal("stdlib engine unexpectedly compiled lookahead pattern")
	}
}
