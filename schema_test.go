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
	"testing"

	"strings"

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

		t.Log(info.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			t.Errorf("  FAIL: %v\n", err)
			return nil
		}
		var tg []testGroup
		if err = json.Unmarshal(data, &tg); err != nil {
			t.Errorf("  FAIL: %v\n", err)
			return nil
		}
		for _, group := range tg {
			t.Logf("  %s\n", group.Description)
			c := jsonschema.NewCompiler()
			if err := c.AddResource("test.json", group.Schema); err != nil {
				t.Errorf("    FAIL: add resource failed, reason: %v\n", err)
				continue
			}
			schema, err := c.Compile("test.json")
			if err != nil {
				t.Errorf("    FAIL: schema compilation failed, reason: %v\n", err)
				continue
			}
			for _, test := range group.Tests {
				t.Logf("      %s\n", test.Description)
				err = schema.Validate(test.Data)
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
