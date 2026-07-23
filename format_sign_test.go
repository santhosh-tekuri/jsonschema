package jsonschema_test

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestFormatRejectsSignedNumericTokens checks that the numeric tokens of the
// ipv4, time, date-time and email (bracketed IPv4 literal) formats are parsed
// as digit-only. strconv.Atoi accepts a leading '+'/'-' sign, so signed tokens
// such as "+4" or "-0" used to validate even though the RFC 3339 and dotted-quad
// grammars are digit-only.
func TestFormatRejectsSignedNumericTokens(t *testing.T) {
	compile := func(t *testing.T, format string) *jsonschema.Schema {
		t.Helper()
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		url := "mem://format-sign.json"
		if err := c.AddResource(url, map[string]any{"type": "string", "format": format}); err != nil {
			t.Fatalf("add resource: %v", err)
		}
		sch, err := c.Compile(url)
		if err != nil {
			t.Fatalf("compile %s: %v", format, err)
		}
		return sch
	}

	cases := []struct {
		format string
		value  string
		valid  bool
	}{
		{"ipv4", "1.2.3.4", true},
		{"ipv4", "1.2.3.+4", false},
		{"ipv4", "+1.2.3.4", false},
		{"ipv4", "1.2.3.-0", false},
		{"time", "12:30:00Z", true},
		{"time", "+9:30:00Z", false},
		{"time", "12:+9:00Z", false},
		{"time", "12:00:00+09:+0", false},
		{"date-time", "2020-01-01T12:30:00Z", true},
		{"date-time", "2020-01-01T+9:30:00Z", false},
		{"email", "a@b.com", true},
		{"email", "a@[1.2.3.+4]", false},
	}
	for _, tc := range cases {
		sch := compile(t, tc.format)
		got := sch.Validate(tc.value) == nil
		if got != tc.valid {
			t.Errorf("format %q value %q: valid=%v, want %v", tc.format, tc.value, got, tc.valid)
		}
	}
}
