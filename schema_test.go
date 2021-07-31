// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v3"
	_ "github.com/santhosh-tekuri/jsonschema/v3/httploader"
)

var testSuite = "testdata/JSON-Schema-Test-Suite@8daea3f"

var skipTests = map[string]string{
	"TestDraft4/optional/zeroTerminatedFloats.json/some_languages_do_not_distinguish_between_different_types_of_numeric_value":                      "this behavior is changed in new drafts",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/Line_tabulation_matches":                                             "\\s does not match vertical tab",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/latin-1_non-breaking-space_matches":                                  "\\s does not match unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/zero-width_whitespace_matches":                                       "\\s does not match unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/paragraph_separator_matches_(line_terminator)":                       "\\s does not match unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/EM_SPACE_matches_(Space_Separator)":                                  "\\s does not match unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/Line_tabulation_does_not_match":                       "\\S matches unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/latin-1_non-breaking-space_does_not_match":            "\\S matches unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/zero-width_whitespace_does_not_match":                 "\\S matches unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/paragraph_separator_does_not_match_(line_terminator)": "\\S matches unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/EM_SPACE_does_not_match_(Space_Separator)":            "\\S matches unicode whitespace",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_upper_letter":                                      "\\cX is not supported",
	"TestDraft4/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_lower_letter":                                      "\\cX is not supported",
	//
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/Line_tabulation_matches":                                             "\\s does not match vertical tab",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/latin-1_non-breaking-space_matches":                                  "\\s does not match unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/zero-width_whitespace_matches":                                       "\\s does not match unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/paragraph_separator_matches_(line_terminator)":                       "\\s does not match unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/EM_SPACE_matches_(Space_Separator)":                                  "\\s does not match unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/Line_tabulation_does_not_match":                       "\\S matches unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/latin-1_non-breaking-space_does_not_match":            "\\S matches unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/zero-width_whitespace_does_not_match":                 "\\S matches unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/paragraph_separator_does_not_match_(line_terminator)": "\\S matches unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/EM_SPACE_does_not_match_(Space_Separator)":            "\\S matches unicode whitespace",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_upper_letter":                                      "\\cX is not supported",
	"TestDraft6/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_lower_letter":                                      "\\cX is not supported",
	//
	"TestDraft7/optional/format/idn-hostname.json":                                                                                                  "idn-hostname format is not implemented",
	"TestDraft7/optional/format/idn-email.json":                                                                                                     "idn-email format is not implemented",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/Line_tabulation_matches":                                             "\\s does not match vertical tab",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/latin-1_non-breaking-space_matches":                                  "\\s does not match unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/zero-width_whitespace_matches":                                       "\\s does not match unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/paragraph_separator_matches_(line_terminator)":                       "\\s does not match unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\s_matches_whitespace/EM_SPACE_matches_(Space_Separator)":                                  "\\s does not match unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/Line_tabulation_does_not_match":                       "\\S matches unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/latin-1_non-breaking-space_does_not_match":            "\\S matches unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/zero-width_whitespace_does_not_match":                 "\\S matches unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/paragraph_separator_does_not_match_(line_terminator)": "\\S matches unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_\\S_matches_everything_but_whitespace/EM_SPACE_does_not_match_(Space_Separator)":            "\\S matches unicode whitespace",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_upper_letter":                                      "\\cX is not supported",
	"TestDraft7/optional/ecmascript-regex.json/ECMA_262_regex_escapes_control_codes_with_\\c_and_lower_letter":                                      "\\cX is not supported",
}

func TestDraft4(t *testing.T) {
	testFolder(t, testSuite+"/tests/draft4", jsonschema.Draft4)
}

func TestDraft6(t *testing.T) {
	testFolder(t, testSuite+"/tests/draft6", jsonschema.Draft6)
}

func TestDraft7(t *testing.T) {
	testFolder(t, testSuite+"/tests/draft7", jsonschema.Draft7)
}

type testGroup struct {
	Description string
	Schema      json.RawMessage
	Tests       []struct {
		Description string
		Data        json.RawMessage
		Valid       bool
		Skip        *string
	}
}

func TestMain(m *testing.M) {
	server := &http.Server{Addr: "localhost:1234", Handler: http.FileServer(http.Dir(testSuite + "/remotes"))}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()
	os.Exit(m.Run())
}

func testFolder(t *testing.T, folder string, draft *jsonschema.Draft) {
	fis, err := ioutil.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range fis {
		if fi.IsDir() {
			t.Run(fi.Name(), func(t *testing.T) {
				testFolder(t, path.Join(folder, fi.Name()), draft)
			})
			continue
		}
		if path.Ext(fi.Name()) != ".json" {
			continue
		}
		t.Run(fi.Name(), func(t *testing.T) {
			if reason, ok := skipTests[t.Name()]; ok {
				t.Skip(reason)
			}
			data, err := ioutil.ReadFile(path.Join(folder, fi.Name()))
			if err != nil {
				t.Fatal(err)
			}
			var tg []struct {
				Description string
				Schema      json.RawMessage
				Tests       []struct {
					Description string
					Data        json.RawMessage
					Valid       bool
				}
			}
			if err = json.Unmarshal(data, &tg); err != nil {
				t.Fatal(err)
			}
			for _, group := range tg {
				t.Run(group.Description, func(t *testing.T) {
					if reason, ok := skipTests[t.Name()]; ok {
						t.Skip(reason)
					}
					c := jsonschema.NewCompiler()
					c.Draft = draft
					if err := c.AddResource("schema.json", bytes.NewReader(group.Schema)); err != nil {
						t.Fatal(err)
					}
					schema, err := c.Compile("schema.json")
					if err != nil {
						t.Fatal(err)
					}
					for _, test := range group.Tests {
						t.Run(test.Description, func(t *testing.T) {
							if reason, ok := skipTests[t.Name()]; ok {
								t.Skip(reason)
							}
							err = schema.Validate(bytes.NewReader(test.Data))
							valid := err == nil
							if !valid {
								for _, line := range strings.Split(err.Error(), "\n") {
									t.Logf("        %s\n", line)
								}
							}
							if test.Valid != valid {
								t.Fatalf("valid: got %v, want %v", valid, test.Valid)
							}
						})
					}
				})
			}
		})
	}
}

func TestInvalidDocs(t *testing.T) {
	invalidDocTests := []struct {
		description string
		doc         string
	}{
		{"non json instance", "{"},
		{"multiple json instance", "{}{}"},
	}
	for _, test := range invalidDocTests {
		t.Run(test.description, func(t *testing.T) {
			c := jsonschema.NewCompiler()
			if err := c.AddResource("test.json", strings.NewReader("{}")); err != nil {
				t.Fatal(err)
			}
			s, err := c.Compile("test.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(strings.NewReader(test.doc)); err != nil {
				t.Log(err)
			} else {
				t.Error("error expected")
			}
		})
	}
}

func TestInvalidSchema(t *testing.T) {
	t.Run("MustCompile with panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("panic expected")
			}
		}()
		jsonschema.MustCompile("testdata/invalid_schema.json")
	})

	t.Run("MustCompile without panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Error("panic not expected")
			}
		}()
		jsonschema.MustCompile("testdata/customer_schema.json#/0")
	})

	t.Run("invalid json", func(t *testing.T) {
		if err := jsonschema.NewCompiler().AddResource("test.json", strings.NewReader("{")); err == nil {
			t.Error("error expected")
		} else {
			t.Log(err)
		}
	})

	t.Run("multiple json", func(t *testing.T) {
		if err := jsonschema.NewCompiler().AddResource("test.json", strings.NewReader("{}{}")); err == nil {
			t.Error("error expected")
		} else {
			t.Log(err)
		}
	})

	type test struct {
		Description string
		Schema      json.RawMessage
		Fragment    string
	}
	data, err := ioutil.ReadFile("testdata/invalid_schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var tests []test
	if err = json.Unmarshal(data, &tests); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.Description, func(t *testing.T) {
			c := jsonschema.NewCompiler()
			url := "test.json"
			if err := c.AddResource(url, bytes.NewReader(test.Schema)); err != nil {
				t.Fatal(err)
			}
			if len(test.Fragment) > 0 {
				url += test.Fragment
			}
			if _, err = c.Compile(url); err == nil {
				t.Error("error expected")
			} else {
				t.Log(err)
			}
		})
	}
}

func TestCompileURL(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport)
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.InsecureSkipVerify = true

	handler := http.FileServer(http.Dir("testdata"))
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	httpsServer := httptest.NewTLSServer(handler)
	defer httpsServer.Close()

	validTests := []struct {
		schema, doc string
	}{
		{"testdata/customer_schema.json#/0", "testdata/customer.json"},
		{toFileURL("testdata/customer_schema.json") + "#/0", "testdata/customer.json"},
		{httpServer.URL + "/customer_schema.json#/0", "testdata/customer.json"},
		{httpsServer.URL + "/customer_schema.json#/0", "testdata/customer.json"},
		{toFileURL("testdata/empty schema.json"), "testdata/empty schema.json"},
	}
	for i, test := range validTests {
		t.Logf("valid #%d: %+v", i, test)
		s, err := jsonschema.Compile(test.schema)
		if err != nil {
			t.Errorf("valid #%d: %v", i, err)
			return
		}
		f, err := os.Open(test.doc)
		if err != nil {
			t.Errorf("valid #%d: %v", i, err)
			return
		}
		err = s.Validate(f)
		_ = f.Close()
		if err != nil {
			t.Errorf("valid #%d: %v", i, err)
		}
	}

	invalidTests := []string{
		"testdata/syntax_error.json",
		"testdata/missing.json",
		toFileURL("testdata/missing.json"),
		httpServer.URL + "/missing.json",
		httpsServer.URL + "/missing.json",
	}
	for i, test := range invalidTests {
		t.Logf("invalid #%d: %v", i, test)
		if _, err := jsonschema.Compile(test); err == nil {
			t.Errorf("invalid #%d: expected error", i)
		} else {
			t.Logf("invalid #%d: %v", i, err)
		}
	}
}

func TestValidateInterface(t *testing.T) {
	files := []string{
		testSuite + "/tests/draft4/type.json",
		testSuite + "/tests/draft4/minimum.json",
		testSuite + "/tests/draft4/maximum.json",
	}
	for _, file := range files {
		t.Log(filepath.Base(file))
		data, err := ioutil.ReadFile(file)
		if err != nil {
			t.Errorf("  FAIL: %v\n", err)
			return
		}
		var tg []testGroup
		if err = json.Unmarshal(data, &tg); err != nil {
			t.Errorf("  FAIL: %v\n", err)
			return
		}
		for _, group := range tg {
			t.Logf("  %s\n", group.Description)
			c := jsonschema.NewCompiler()
			if err := c.AddResource("test.json", bytes.NewReader(group.Schema)); err != nil {
				t.Errorf("    FAIL: add resource failed, reason: %v\n", err)
				continue
			}
			c.Draft = jsonschema.Draft4
			schema, err := c.Compile("test.json")
			if err != nil {
				t.Errorf("    FAIL: schema compilation failed, reason: %v\n", err)
				continue
			}
			for _, test := range group.Tests {
				t.Logf("      %s\n", test.Description)

				decoder := json.NewDecoder(bytes.NewReader(test.Data))
				var doc interface{}
				if err := decoder.Decode(&doc); err != nil {
					t.Errorf("        FAIL: decode json failed, reason: %v\n", err)
					continue
				}

				err = schema.ValidateInterface(doc)
				valid := err == nil
				if !valid {
					for _, line := range strings.Split(err.Error(), "\n") {
						t.Logf("        %s\n", line)
					}
				}
				if test.Valid != valid {
					t.Errorf("        FAIL: expected valid=%t got valid=%t\n", test.Valid, valid)
				}
			}
		}
	}
}

func TestInvalidJsonTypeError(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	err := compiler.AddResource("test.json", strings.NewReader(`{ "type": "string"}`))
	if err != nil {
		t.Fatalf("addResource failed. reason: %v\n", err)
	}
	schema, err := compiler.Compile("test.json")
	if err != nil {
		t.Fatalf("schema compilation failed. reason: %v\n", err)
	}
	v := struct{ name string }{"hello world"}
	err = schema.ValidateInterface(v)
	switch err.(type) {
	case jsonschema.InvalidJSONTypeError:
		// passed
	default:
		t.Fatalf("got %v. want InvalidJSONTypeErr", err)
	}
}

func TestExtractAnnotations(t *testing.T) {
	t.Run("false", func(t *testing.T) {
		compiler := jsonschema.NewCompiler()

		err := compiler.AddResource("test.json", strings.NewReader(`{
			"title": "this is title"
		}`))
		if err != nil {
			t.Fatalf("addResource failed. reason: %v\n", err)
		}

		schema, err := compiler.Compile("test.json")
		if err != nil {
			t.Fatalf("schema compilation failed. reason: %v\n", err)
		}

		if schema.Title != "" {
			t.Error("title should not be extracted")
		}
	})

	t.Run("true", func(t *testing.T) {
		compiler := jsonschema.NewCompiler()
		compiler.ExtractAnnotations = true

		err := compiler.AddResource("test.json", strings.NewReader(`{
			"title": "this is title"
		}`))
		if err != nil {
			t.Fatalf("addResource failed. reason: %v\n", err)
		}

		schema, err := compiler.Compile("test.json")
		if err != nil {
			t.Fatalf("schema compilation failed. reason: %v\n", err)
		}

		if schema.Title != "this is title" {
			t.Errorf("title: got %q, want %q", schema.Title, "this is title")
		}
	})

	t.Run("examples", func(t *testing.T) {
		compiler := jsonschema.NewCompiler()
		compiler.ExtractAnnotations = true

		err := compiler.AddResource("test.json", strings.NewReader(`{
			"title": "this is title",
			"format": "date-time",
			"examples": ["2019-04-09T21:54:56.052Z"]
		}`))
		if err != nil {
			t.Fatalf("addResource failed. reason: %v\n", err)
		}

		schema, err := compiler.Compile("test.json")
		if err != nil {
			t.Fatalf("schema compilation failed. reason: %v\n", err)
		}

		if schema.Title != "this is title" {
			t.Errorf("title: got %q, want %q", schema.Title, "this is title")
		}
		if schema.Examples[0] != "2019-04-09T21:54:56.052Z" {
			t.Errorf("example: got %q, want %q", schema.Examples[0], "2019-04-09T21:54:56.052Z")
		}
	})
}

func toFileURL(path string) string {
	path, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		path = "/" + path
	}
	u, err := url.Parse("file://" + path)
	if err != nil {
		panic(err)
	}
	return u.String()
}

// TestPanic tests https://github.com/santhosh-tekuri/jsonschema/issues/18
func TestPanic(t *testing.T) {
	schema_d := `
	{
		"type": "object",
		"properties": {
		"myid": { "type": "integer" },
		"otype": { "$ref": "defs.json#someid" }
		}
	}
	`
	defs_d := `
	{
		"definitions": {
		"stt": {
			"$schema": "http://json-schema.org/draft-07/schema#",
			"$id": "#someid",
				"type": "object",
			"enum": [ { "name": "stainless" }, { "name": "zinc" } ]
		}
		}
	}
	`
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft7
	if err := c.AddResource("schema.json", strings.NewReader(schema_d)); err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("defs.json", strings.NewReader(defs_d)); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Compile("schema.json"); err != nil {
		t.Fatal(err)
	}
}

func TestNonStringFormat(t *testing.T) {
	jsonschema.Formats["even-number"] = func(v interface{}) bool {
		switch v := v.(type) {
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return false
			}
			return i%2 == 0
		default:
			return false
		}
	}
	schema := `{"type": "integer", "format": "even-number"}`
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", strings.NewReader(schema)); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Validate(strings.NewReader("5")); err == nil {
		t.Fatal("error expected")
	}
	if err = s.Validate(strings.NewReader("6")); err != nil {
		t.Fatal(err)
	}
}

func TestCompiler_LoadURL(t *testing.T) {
	const (
		base   = `{ "type": "string" }`
		schema = `{ "allOf": [{ "$ref": "base.json" }, { "maxLength": 3 }] }`
	)

	c := jsonschema.NewCompiler()
	c.LoadURL = func(s string) (io.ReadCloser, error) {
		switch s {
		case "base.json":
			return ioutil.NopCloser(strings.NewReader(base)), nil
		case "schema.json":
			return ioutil.NopCloser(strings.NewReader(schema)), nil
		default:
			return nil, errors.New("unsupported schema")
		}
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Validate(strings.NewReader(`"foo"`)); err != nil {
		t.Fatal(err)
	}
	if err = s.Validate(strings.NewReader(`"long"`)); err == nil {
		t.Fatal("error expected")
	}
}

func TestCompiler_dollar_schema(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{"http_trailing_hash", `{"$schema":"http://json-schema.org/draft-07/schema#"}`},
		{"http_no_trailing_hash", `{"$schema":"http://json-schema.org/draft-07/schema"}`},
		{"https_trailing_hash", `{"$schema":"https://json-schema.org/draft-07/schema#"}`},
		{"https_no_trailing_hash", `{"$schema":"https://json-schema.org/draft-07/schema"}`},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := jsonschema.NewCompiler()
			if err := c.AddResource("test.json", strings.NewReader(tt.schema)); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Compile("test.json"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
