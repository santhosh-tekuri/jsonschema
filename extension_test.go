// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v3"
)

var powerOfMeta = jsonschema.MustCompileString("powerOf.json", `{
	"properties" : {
		"powerOf": {
			"type": "integer",
			"exclusiveMinimum": 0
		}
	}
}`)

type powerOfCompiler struct{}

func (powerOfCompiler) Compile(ctx jsonschema.CompilerContext, m map[string]interface{}) (jsonschema.ExtSchema, error) {
	if pow, ok := m["powerOf"]; ok {
		n, err := pow.(json.Number).Int64()
		return powerOfSchema(n), err
	}

	// nothing to compile, return nil
	return nil, nil
}

type powerOfSchema int64

func (s powerOfSchema) Validate(ctx jsonschema.ValidationContext, v interface{}) error {
	switch v.(type) {
	case json.Number, float64, int, int32, int64:
		pow := int64(s)
		n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		for n%pow == 0 {
			n = n / pow
		}
		if n != 1 {
			return ctx.Error("powerOf", "%v not powerOf %v", v, pow)
		}
		return nil
	default:
		return nil
	}
}

func TestPowerOfExt(t *testing.T) {
	t.Run("invalidSchema", func(t *testing.T) {
		c := jsonschema.NewCompiler()
		c.RegisterExtension("powerOf", powerOfMeta, powerOfCompiler{})
		if err := c.AddResource("test.json", strings.NewReader(`{"powerOf": "hello"}`)); err != nil {
			t.Fatal(err)
		}
		_, err := c.Compile("test.json")
		if err == nil {
			t.Fatal("error expected")
		}
		t.Log(err)
	})
	t.Run("validSchema", func(t *testing.T) {
		c := jsonschema.NewCompiler()
		c.RegisterExtension("powerOf", powerOfMeta, powerOfCompiler{})
		if err := c.AddResource("test.json", strings.NewReader(`{"powerOf": 10}`)); err != nil {
			t.Fatal(err)
		}
		sch, err := c.Compile("test.json")
		if err != nil {
			t.Fatal(err)
		}
		t.Run("validInstance", func(t *testing.T) {
			if err := sch.Validate(strings.NewReader(`100`)); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("invalidInstance", func(t *testing.T) {
			if err := sch.Validate(strings.NewReader(`111`)); err == nil {
				t.Fatal("validation must fail")
			} else {
				if !strings.Contains(err.Error(), "111 not powerOf 10") {
					t.Fatal("validation error expected to contain powerOf message")
				}
				t.Log(err)
			}
		})
	})
}
