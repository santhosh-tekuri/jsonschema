// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema"
	_ "github.com/santhosh-tekuri/jsonschema/httploader"
)

func TestValidate(t *testing.T) {
	server := &http.Server{Addr: ":1234", Handler: http.FileServer(http.Dir("testdata/remotes"))}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			t.Fatal(err)
		}
	}()
	defer server.Close()

	type testGroup struct {
		Description string
		Schema      json.RawMessage
		Tests       []struct {
			Description string
			Data        json.RawMessage
			Valid       bool
		}
	}
	err := filepath.Walk("testdata/draft4", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Error(err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(info.Name()) != ".json" {
			return nil
		}

		t.Run(strings.TrimSuffix(info.Name(), ".json"), func(t *testing.T) {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				t.Error(err)
				return
			}
			var tg []testGroup
			if err = json.Unmarshal(data, &tg); err != nil {
				t.Error(err)
				return
			}
			for _, group := range tg {
				t.Run(group.Description, func(t *testing.T) {
					c := jsonschema.NewCompiler()
					if err := c.AddResource("test.json", group.Schema); err != nil {
						t.Fatalf("add resource failed, reason: %v", err)
					}
					schema, err := c.Compile("test.json")
					if err != nil {
						t.Fatalf("schema compilation failed, reason: %v", err)
					}
					for _, test := range group.Tests {
						t.Run(test.Description, func(t *testing.T) {
							err = schema.Validate(test.Data)
							valid := err == nil
							if !valid {
								t.Logf("validation failed. reason:\n%s", err)
							}
							if test.Valid != valid {
								t.Errorf("expected valid=%t got valid=%t", test.Valid, valid)
							}
						})
					}
				})
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

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
			if err := c.AddResource("test.json", []byte("{}")); err != nil {
				t.Fatal(err)
			}
			s, err := c.Compile("test.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate([]byte(test.doc)); err != nil {
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
		if err := jsonschema.NewCompiler().AddResource("test.json", []byte("{")); err == nil {
			t.Error("error expected")
		} else {
			t.Log(err)
		}
	})

	t.Run("multiple json", func(t *testing.T) {
		if err := jsonschema.NewCompiler().AddResource("test.json", []byte("{}{}")); err == nil {
			t.Error("error expected")
		} else {
			t.Log(err)
		}
	})

	t.Run("schemaRef", func(t *testing.T) {
		c := jsonschema.NewCompiler()
		if err := c.AddResource("test.json", []byte("[1]")); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Compile("test.json#/0"); err == nil {
			t.Error("error expected")
		} else {
			t.Log(err)
		}
	})

	type test struct {
		Description string
		Schema      json.RawMessage
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
			if err := c.AddResource("test.json", test.Schema); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Compile("test.json"); err == nil {
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

	abs, err := filepath.Abs("testdata")
	if err != nil {
		t.Error(err)
		return
	}
	validTests := []struct {
		schema, doc string
	}{
		{"testdata/customer_schema.json#/0", "testdata/customer.json"},
		{"file://" + abs + "/customer_schema.json#/0", "testdata/customer.json"},
		{httpServer.URL + "/customer_schema.json#/0", "testdata/customer.json"},
		{httpsServer.URL + "/customer_schema.json#/0", "testdata/customer.json"},
	}
	for i, test := range validTests {
		t.Logf("valid #%d: %+v", i, test)
		s, err := jsonschema.Compile(test.schema)
		if err != nil {
			t.Errorf("valid #%d: %v", i, err)
			return
		}
		data, err := ioutil.ReadFile(test.doc)
		if err != nil {
			t.Errorf("valid #%d: %v", i, err)
			return
		}
		if err = s.Validate(data); err != nil {
			t.Errorf("valid #%d: %v", i, err)
		}
	}

	invalidTests := []string{
		"testdata/missing.json",
		"file://" + abs + "/missing.json",
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
