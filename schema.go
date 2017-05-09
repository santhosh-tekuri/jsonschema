// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/formats"
)

type Schema struct {
	url *string
	ptr *string

	// type agnostic validations
	ref   *Schema
	types []string
	enum  []interface{}
	not   *Schema
	allOf []*Schema
	anyOf []*Schema
	oneOf []*Schema

	// object validations
	minProperties        int // -1 if not specified
	maxProperties        int // -1 if not specified
	required             []string
	properties           map[string]*Schema
	regexProperties      bool // property names must be valid regex
	patternProperties    map[*regexp.Regexp]*Schema
	additionalProperties interface{}            // nil or false or *Schema
	dependencies         map[string]interface{} // value is *Schema or []string

	// array validations
	minItems        int // -1 if not specified
	maxItems        int // -1 if not specified
	uniqueItems     bool
	items           interface{} // nil or *Schema or []*Schema
	additionalItems interface{} // nil or bool or *Schema

	// string validations
	minLength int // -1 if not specified
	maxLength int // -1 if not specified
	pattern   *regexp.Regexp
	format    string

	// number validators
	minimum          *float64
	exclusiveMinimum bool
	maximum          *float64
	exclusiveMaximum bool
	multipleOf       *float64
}

func Compile(url string) (*Schema, error) {
	return NewCompiler().Compile(url)
}

func (s *Schema) Validate(data []byte) error {
	var doc interface{}
	err := json.Unmarshal(data, &doc)
	if err != nil {
		return err
	}
	err = s.validate(doc)
	if err != nil {
		finishContext(err, s)
	}
	return err
}

func (s *Schema) validate(v interface{}) error {
	if s.ref != nil {
		if err := s.ref.validate(v); err != nil {
			finishContext(err, s.ref)
			return validationError("$ref", "$ref failed").add(err)
		}

		// All other properties in a "$ref" object MUST be ignored
		return nil
	}

	if len(s.types) > 0 {
		vType := jsonType(v)
		matched := false
		for _, t := range s.types {
			if t == "integer" {
				if vType == "number" && v == math.Trunc(v.(float64)) {
					matched = true
					break
				}
			} else if vType == t {
				matched = true
				break
			}
		}
		if !matched {
			return validationError("type", "expected %s, but got %s", strings.Join(s.types, " or "), vType)
		}
	}

	if len(s.enum) > 0 {
		matched := false
		allPrimitives := true
		for _, item := range s.enum {
			if equals(v, item) {
				matched = true
				break
			}
			switch jsonType(item) {
			case "object", "arr":
				allPrimitives = false
			}
		}
		if !matched {
			if allPrimitives {
				if len(s.enum) == 1 {
					return validationError("enum", "value must be %v", s.enum[0])
				}
				strEnum := make([]string, 0, len(s.enum))
				for _, item := range s.enum {
					strEnum = append(strEnum, fmt.Sprintf("%v", item))
				}
				return validationError("enum", "value must be one of %s", strings.Join(strEnum, ", "))
			}
			return validationError("enum", "enum failed")
		}
	}

	if s.not != nil {
		if s.not.validate(v) == nil {
			return validationError("not", "not failed")
		}
	}

	if len(s.allOf) > 0 {
		for i, sch := range s.allOf {
			if err := sch.validate(v); err != nil {
				return validationError("allOf/"+strconv.Itoa(i), "allOf failed").add(err)
			}
		}
	}

	if len(s.anyOf) > 0 {
		matched := false
		var causes []error
		for i, sch := range s.anyOf {
			if err := sch.validate(v); err == nil {
				matched = true
				break
			} else {
				causes = append(causes, addContext("", strconv.Itoa(i), err))
			}
		}
		if !matched {
			return validationError("anyOf", "anyOf failed").add(causes...)
		}
	}

	if len(s.oneOf) > 0 {
		matched := -1
		var causes []error
		for i, sch := range s.oneOf {
			if err := sch.validate(v); err == nil {
				if matched == -1 {
					matched = i
				} else {
					return validationError("oneOf", "valid against schemas at indexes %d and %d", matched, i)
				}
			} else {
				causes = append(causes, addContext("", strconv.Itoa(i), err))
			}
		}
		if matched == -1 {
			return validationError("oneOf", "oneOf failed").add(causes...)
		}
	}

	switch v := v.(type) {
	case map[string]interface{}:
		if s.minProperties != -1 {
			if len(v) < s.minProperties {
				return validationError("minProperties", "minimum %d properties allowed, but found %d properties", s.minProperties, len(v))
			}
		}
		if s.maxProperties != -1 {
			if len(v) > s.maxProperties {
				return validationError("maxProperties", "maximum %d properties allowed, but found %d properties", s.maxProperties, len(v))
			}
		}
		if len(s.required) > 0 {
			var missing []string
			for _, pname := range s.required {
				if _, ok := v[pname]; !ok {
					missing = append(missing, pname)
				}
			}
			if len(missing) > 0 {
				return validationError("required", "missing properties: %s", strings.Join(missing, ", "))
			}
		}

		var additionalProps map[string]struct{}
		if s.additionalProperties != nil {
			additionalProps = make(map[string]struct{}, len(v))
			for pname := range v {
				additionalProps[pname] = struct{}{}
			}
		}

		if len(s.properties) > 0 {
			for pname, pschema := range s.properties {
				if pvalue, ok := v[pname]; ok {
					delete(additionalProps, pname)
					if err := pschema.validate(pvalue); err != nil {
						return addContext(escape(pname), "properties/"+escape(pname), err) // todo pname escaping in sptr
					}
				}
			}
		}
		if s.regexProperties {
			for pname := range v {
				if !formats.IsRegex(pname) {
					return validationError("", "patternProperty %q is not valid regex", pname)
				}
			}
		}
		if len(s.patternProperties) > 0 {
			for pattern, pschema := range s.patternProperties {
				for pname, pvalue := range v {
					if pattern.MatchString(pname) {
						delete(additionalProps, pname)
						if err := pschema.validate(pvalue); err != nil {
							return addContext(escape(pname), "patternProperties/"+escape(pattern.String()), err) // todo pattern escaping in sptr
						}
					}
				}
			}
		}
		if s.additionalProperties != nil {
			if _, ok := s.additionalProperties.(bool); ok {
				if len(additionalProps) != 0 {
					pnames := make([]string, 0, len(additionalProps))
					for pname := range additionalProps {
						pnames = append(pnames, pname)
					}
					return validationError("additionalProperties", "additionalProperties %s not allowed", strings.Join(pnames, ", "))
				}
			} else {
				schema := s.additionalProperties.(*Schema)
				for pname := range additionalProps {
					if pvalue, ok := v[pname]; ok {
						if err := schema.validate(pvalue); err != nil {
							return addContext(escape(pname), "additionalProperties", err)
						}
					}
				}
			}
		}
		if len(s.dependencies) > 0 {
			for dname, dvalue := range s.dependencies {
				if _, ok := v[dname]; ok {
					switch dvalue := dvalue.(type) {
					case *Schema:
						if err := dvalue.validate(v); err != nil {
							return addContext("", "dependencies/"+escape(dname), err)
						}
					case []string:
						for i, pname := range dvalue {
							if _, ok := v[pname]; !ok {
								return validationError("dependencies/"+escape(dname)+"/"+strconv.Itoa(i), "property %s is required, if %s property exists", pname, dname)
							}
						}
					}
				}
			}
		}

	case []interface{}:
		if s.minItems != -1 {
			if len(v) < s.minItems {
				return validationError("minItems", "minimum %d items allowed, but found %d items", s.minItems, len(v))
			}
		}
		if s.maxItems != -1 {
			if len(v) > s.maxItems {
				return validationError("maxItems", "maximum %d items allowed, but found %d items", s.maxItems, len(v))
			}
		}
		if s.uniqueItems {
			for i := 1; i < len(v); i++ {
				for j := 0; j < i; j++ {
					if equals(v[i], v[j]) {
						return validationError("uniqueItems", "items at %d and %d are equal", j, i)
					}
				}
			}
		}
		switch items := s.items.(type) {
		case *Schema:
			for i, item := range v {
				if err := items.validate(item); err != nil {
					return addContext(strconv.Itoa(i), "items", err)
				}
			}
		case []*Schema:
			if additionalItems, ok := s.additionalItems.(bool); ok {
				if !additionalItems && len(v) > len(items) {
					return validationError("additionalItems", "only %d items are allowed, but found %d items", len(items), len(v))
				}
			}
			for i, item := range v {
				if i < len(items) {
					if err := items[i].validate(item); err != nil {
						return addContext(strconv.Itoa(i), "items/"+strconv.Itoa(i), err)
					}
				} else if sch, ok := s.additionalItems.(*Schema); ok {
					if err := sch.validate(item); err != nil {
						return addContext(strconv.Itoa(i), "additionalItems", err)
					}
				} else {
					break
				}
			}
		}

	case string:
		if s.minLength != -1 || s.maxLength != -1 {
			length := utf8.RuneCount([]byte(v))
			if s.minLength != -1 && length < s.minLength {
				return validationError("minLength", "length must be >= %d, but got %d", s.minLength, length)
			}
			if s.maxLength != -1 && length > s.maxLength {
				return validationError("maxLength", "length must be <= %d, but got %d", s.maxLength, length)
			}
		}
		if s.pattern != nil {
			if !s.pattern.MatchString(v) {
				return validationError("pattern", "does not match pattern %s", s.pattern)
			}
		}
		if len(s.format) > 0 {
			f, _ := formats.Get(s.format)
			if !f(v) {
				return validationError("format", "%q is not valid %s", v, s.format)
			}
		}

	case float64:
		if s.minimum != nil {
			if s.exclusiveMinimum {
				if v <= *s.minimum {
					return validationError("minimum", "must be > %f but found %f", *s.minimum, v)
				}
			} else {
				if v < *s.minimum {
					return validationError("minimum", "must be >= %f but found %f", *s.minimum, v)
				}
			}
		}
		if s.maximum != nil {
			if s.exclusiveMaximum {
				if v >= *s.maximum {
					return validationError("maximum", "must be < %f but found %f", *s.maximum, v)
				}
			} else {
				if v > *s.maximum {
					return validationError("maximum", "must be <= %f but found %f", *s.maximum, v)
				}
			}
		}
		if s.multipleOf != nil {
			if *s.multipleOf != 0 {
				t := v / *s.multipleOf
				if t != math.Trunc(t) {
					return validationError("multipleOf", "%f not multipleOf %f", v, *s.multipleOf)
				}
			}
		}
	}

	return nil
}

func jsonType(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func equals(v1, v2 interface{}) bool {
	v1Type, v2Type := jsonType(v1), jsonType(v2)
	if v1Type != v2Type {
		return false
	}
	switch v1Type {
	case "array":
		arr1, arr2 := v1.([]interface{}), v2.([]interface{})
		if len(arr1) != len(arr2) {
			return false
		}
		for i := range arr1 {
			if !equals(arr1[i], arr2[i]) {
				return false
			}
		}
		return true
	case "object":
		obj1, obj2 := v1.(map[string]interface{}), v2.(map[string]interface{})
		if len(obj1) != len(obj2) {
			return false
		}
		for k := range obj1 {
			if !equals(obj1[k], obj2[k]) {
				return false
			}
		}
		return true
	default:
		return v1 == v2
	}
}

func escape(token string) string {
	token = strings.Replace(token, "~", "~0", -1)
	token = strings.Replace(token, "/", "~1", -1)
	return url.PathEscape(token)
}
