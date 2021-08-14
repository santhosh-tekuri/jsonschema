// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v3"
)

func Example() {
	sch, err := jsonschema.Compile("testdata/person_schema.json")
	if err != nil {
		log.Fatalf("%#v", err)
	}

	f, err := os.Open("testdata/person.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err = sch.Validate(f); err != nil {
		log.Fatalf("%#v", err)
	}
	// Output:
}

// Example_fromStrings shows how to load schema from string.
func Example_fromStrings() {
	schema := `{"type": "object"}`
	instance := `{"foo": "bar"}`

	sch, err := jsonschema.CompileString("schema.json", schema)
	if err != nil {
		log.Fatalf("%#v", err)
	}

	if err = sch.Validate(strings.NewReader(instance)); err != nil {
		log.Fatalf("%#v", err)
	}
	// Output:
}

// Example_userDefinedFormat shows how to define 'odd-number' format.
func Example_userDefinedFormat() {
	jsonschema.Formats["odd-number"] = func(v interface{}) bool {
		switch v := v.(type) {
		case json.Number, float64, int, int32, int64:
			n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
			return n%2 != 0
		default:
			return true
		}
	}

	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"type": "integer",
		"format": "odd-number"
	}`
	instance := `5`

	sch, err := jsonschema.CompileString("schema.json", schema)
	if err != nil {
		log.Fatalf("%#v", err)
	}

	if err = sch.Validate(strings.NewReader(instance)); err != nil {
		log.Fatalf("%#v", err)
	}
	// Output:
}

// Example_userDefinedContent shows how to define:
// "hex" contentEncoding and "application/xml" contentMediaType
func Example_userDefinedContent() {
	jsonschema.Decoders["hex"] = hex.DecodeString
	jsonschema.MediaTypes["application/xml"] = func(b []byte) error {
		return xml.Unmarshal(b, new(interface{}))
	}

	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"type": "object",
		"properties": {
			"xml" : {
				"type": "string",
				"contentEncoding": "hex",
				"contentMediaType": "application/xml"
			}
		}
	}`
	instance := `{"xml": "3c726f6f742f3e"}`

	sch, err := jsonschema.CompileString("schema.json", schema)
	if err != nil {
		log.Fatalf("%#v", err)
	}

	if err = sch.Validate(strings.NewReader(instance)); err != nil {
		log.Fatalf("%#v", err)
	}
	// Output:
}
