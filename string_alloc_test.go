package jsonschema_test

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileContent(t testing.TB, schema string) *jsonschema.Schema {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(schema), &doc); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertContent()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

// A string is copied only for the content keywords that read its bytes.
func TestStringValidationDoesNotCopyTheString(t *testing.T) {
	sch := compileContent(t, `{"type":"string"}`)
	value := strings.Repeat("a", 1<<20)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := sch.Validate(value); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= 1<<20 {
		t.Fatalf("validating a 1 MiB string allocated %d bytes", allocated)
	}
}

func TestContentKeywordsStillReadTheString(t *testing.T) {
	tests := []struct {
		name, schema, value string
		valid               bool
	}{
		{"no keyword", `{"type":"string"}`, "{", true},
		{"no keyword, empty", `{"type":"string"}`, "", true},
		{"encoding, good", `{"contentEncoding":"base64"}`, "aGk=", true},
		{"encoding, bad", `{"contentEncoding":"base64"}`, "!!!", false},
		{"encoding, empty", `{"contentEncoding":"base64"}`, "", true},
		{"media type, good", `{"contentMediaType":"application/json"}`, "{}", true},
		{"media type, bad", `{"contentMediaType":"application/json"}`, "{", false},
		{"media type, empty", `{"contentMediaType":"application/json"}`, "", false},
		{"both, good", `{"contentEncoding":"base64","contentMediaType":"application/json"}`, "e30=", true},
		{"both, bad media type", `{"contentEncoding":"base64","contentMediaType":"application/json"}`, "aGk=", false},
		{"both, bad encoding", `{"contentEncoding":"base64","contentMediaType":"application/json"}`, "!!!", false},
		{"content schema, good", `{"contentMediaType":"application/json","contentSchema":{"type":"object"}}`, "{}", true},
		{"content schema, bad", `{"contentMediaType":"application/json","contentSchema":{"type":"object"}}`, "[]", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := compileContent(t, tc.schema).Validate(tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("Validate(%q) = %v, want valid %v", tc.value, err, tc.valid)
			}
		})
	}
}

func BenchmarkValidateLongString(b *testing.B) {
	sch := compileContent(b, `{"type":"string"}`)
	value := strings.Repeat("a", 256<<10)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := sch.Validate(value); err != nil {
			b.Fatal(err)
		}
	}
}
