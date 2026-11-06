package jsonschema

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestOneOfDiscriminatorThroughLocalRefs(t *testing.T) {
	sch := compileOneOfTestSchema(t, `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs": {
			"catKind": {"const": "cat"},
			"dogKind": {"const": "dog"},
			"cat": {
				"type": "object",
				"required": ["kind"],
				"properties": {"kind": {"$ref": "#/$defs/catKind"}}
			},
			"dog": {
				"type": "object",
				"required": ["kind"],
				"properties": {"kind": {"$ref": "#/$defs/dogKind"}}
			}
		},
		"oneOf": [
			{"$ref": "#/$defs/cat"},
			{"$ref": "#/$defs/dog"},
			{"required": ["name"]}
		]
	}`)

	d := sch.oneOfDiscriminator
	if d == nil {
		t.Fatal("oneOf discriminator was not detected")
	}
	if got, want := d.property, "kind"; got != want {
		t.Fatalf("discriminator property = %q, want %q", got, want)
	}
	if got, want := d.values, []string{"cat", "dog", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("discriminator values = %v, want %v", got, want)
	}
	if got, want := d.classified, []bool{true, true, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("classified branches = %v, want %v", got, want)
	}
}

func TestOneOfDiscriminatorPreservesValidation(t *testing.T) {
	const schema = `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"oneOf": [
			{
				"required": ["kind", "a"],
				"properties": {
					"kind": {"const": "cat"},
					"a": {"type": "integer"}
				}
			},
			{
				"required": ["kind", "b"],
				"properties": {
					"kind": {"const": "cat"},
					"b": {"type": "string"}
				}
			},
			{
				"required": ["kind", "bark"],
				"properties": {
					"kind": {"const": "dog"},
					"bark": {"type": "boolean"}
				}
			},
			{
				"required": ["name"],
				"properties": {"name": {"type": "string"}}
			}
		],
		"unevaluatedProperties": false
	}`

	tests := []struct {
		name     string
		instance any
	}{
		{
			name:     "selected branch matches",
			instance: map[string]any{"kind": "dog", "bark": true},
		},
		{
			name:     "selected branch fails with full diagnostics",
			instance: map[string]any{"kind": "dog", "bark": "yes"},
		},
		{
			name:     "branches sharing discriminator remain candidates",
			instance: map[string]any{"kind": "cat", "a": 1, "b": "two"},
		},
		{
			name:     "unclassified branch remains a candidate",
			instance: map[string]any{"kind": "dog", "bark": true, "name": "Fido"},
		},
		{
			name:     "unknown discriminator can match unclassified branch",
			instance: map[string]any{"kind": "bird", "name": "Polly"},
		},
		{
			name:     "missing discriminator uses full validation",
			instance: map[string]any{"name": "Mystery"},
		},
		{
			name:     "non-string discriminator uses full validation",
			instance: map[string]any{"kind": 7, "name": "Seven"},
		},
		{
			name:     "matching branch contributes evaluated properties",
			instance: map[string]any{"kind": "cat", "a": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOneOfOptimizedMatchesBaseline(t, schema, tt.instance)
		})
	}
}

func TestOneOfDiscriminatorRequiresRequiredConst(t *testing.T) {
	sch := compileOneOfTestSchema(t, `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"oneOf": [
			{"properties": {"kind": {"const": "cat"}}},
			{"properties": {"kind": {"const": "dog"}}}
		]
	}`)
	if sch.oneOfDiscriminator != nil {
		t.Fatal("optional const property must not be used as a discriminator")
	}
}

func BenchmarkOneOfConstDiscriminator(b *testing.B) {
	optimized := compileOneOfBenchmarkSchema(b)
	baseline := compileOneOfBenchmarkSchema(b)
	baseline.oneOfDiscriminator = nil

	payload := make(map[string]any, 24)
	for i := 0; i < 24; i++ {
		payload[fmt.Sprintf("field%d", i)] = "value"
	}
	instance := map[string]any{
		"kind":    "kind6",
		"payload": payload,
	}
	if err := optimized.Validate(instance); err != nil {
		b.Fatal(err)
	}

	for _, benchmark := range []struct {
		name string
		sch  *Schema
	}{
		{name: "baseline", sch: baseline},
		{name: "optimized", sch: optimized},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := benchmark.sch.Validate(instance); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func assertOneOfOptimizedMatchesBaseline(t *testing.T, source string, instance any) {
	t.Helper()
	sch := compileOneOfTestSchema(t, source)
	if sch.oneOfDiscriminator == nil {
		t.Fatal("test schema has no compiled oneOf discriminator")
	}
	got := sch.Validate(instance)
	sch.oneOfDiscriminator = nil
	want := sch.Validate(instance)
	gotDiagnostic := canonicalValidationError(got)
	wantDiagnostic := canonicalValidationError(want)
	if gotDiagnostic != wantDiagnostic {
		t.Fatalf("optimized validation differs from baseline\noptimized: %#v\nbaseline:  %#v", got, want)
	}
}

func canonicalValidationError(err error) string {
	if err == nil {
		return ""
	}
	verr := err.(*ValidationError)
	causes := make([]string, len(verr.Causes))
	for i, cause := range verr.Causes {
		causes[i] = canonicalValidationError(cause)
	}
	// Object properties are validated in map iteration order, so cause order is
	// not part of the diagnostic contract even without this optimization.
	sort.Strings(causes)
	return fmt.Sprintf("%s|%v|%T:%#v|%v", verr.SchemaURL, verr.InstanceLocation, verr.ErrorKind, verr.ErrorKind, causes)
}

func compileOneOfTestSchema(t testing.TB, source string) *Schema {
	t.Helper()
	doc, err := UnmarshalJSON(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func compileOneOfBenchmarkSchema(b *testing.B) *Schema {
	b.Helper()
	var source strings.Builder
	source.WriteString(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"$defs":{
			"payload":{
				"type":"object",
				"required":[`)
	for i := 0; i < 24; i++ {
		if i > 0 {
			source.WriteByte(',')
		}
		fmt.Fprintf(&source, `"field%d"`, i)
	}
	source.WriteString(`],"properties":{`)
	for i := 0; i < 24; i++ {
		if i > 0 {
			source.WriteByte(',')
		}
		fmt.Fprintf(&source, `"field%d":{"type":"string"}`, i)
	}
	source.WriteString(`}}},"oneOf":[`)
	for i := 0; i < 12; i++ {
		if i > 0 {
			source.WriteByte(',')
		}
		fmt.Fprintf(&source, `{
			"type":"object",
			"required":["kind","payload"],
			"properties":{
				"kind":{"const":"kind%d"},
				"payload":{"$ref":"#/$defs/payload"}
			}
		}`, i)
	}
	source.WriteString(`]}`)
	return compileOneOfTestSchema(b, source.String())
}
