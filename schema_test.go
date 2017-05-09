// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema"
	_ "github.com/santhosh-tekuri/jsonschema/httploader"
)

func TestValidate(t *testing.T) {
	type testGroup struct {
		Description string
		Schema      json.RawMessage
		Tests       []struct {
			Description string
			Data        json.RawMessage
			Valid       bool
		}
	}
	err := filepath.Walk("testdata/tests", func(path string, info os.FileInfo, err error) error {
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
					c.AddResource("test.json", group.Schema)
					schema, err := c.Compile("test.json")
					if err != nil {
						t.Fatalf("schema compilation failed, reason: %v", err)
					}
					for _, test := range group.Tests {
						t.Run(test.Description, func(t *testing.T) {
							err = schema.Validate(test.Data)
							valid := err == nil
							if !valid {
								t.Log(err)
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
}

func TestInvalidSchema(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		if err := jsonschema.NewCompiler().AddResource("test.json", []byte("{")); err == nil {
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
	abs, err := filepath.Abs("testdata/customer_schema.json")
	if err != nil {
		t.Error(err)
		return
	}
	tests := []struct {
		schema, doc string
	}{
		{"testdata/customer_schema.json#/0", "testdata/customer.json"},
		{"file://" + abs + "#/0", "testdata/customer.json"},
	}
	for i, test := range tests {
		t.Logf("#%d: %+v", i, test)
		s, err := jsonschema.Compile(test.schema)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
			return
		}
		data, err := ioutil.ReadFile(test.doc)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
			return
		}
		if err = s.Validate(data); err != nil {
			t.Errorf("#%d: %v", i, err)
		}
	}
}
