// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"regexp"

	"github.com/santhosh-tekuri/jsonschema/loader"
)

type Compiler struct {
	resources map[string]*resource
}

func NewCompiler() *Compiler {
	return &Compiler{make(map[string]*resource)}
}

func (c *Compiler) AddResource(url string, data []byte) error {
	r, err := newResource(url, data)
	if err != nil {
		return err
	}
	c.resources[r.url] = r
	return nil
}

func (c *Compiler) Compile(url string) (*Schema, error) {
	return c.compileRef(split(url))
}

func (c *Compiler) compileRef(base, ref string) (*Schema, error) {
	if _, ok := c.resources[base+"#"]; !ok {
		data, err := loader.Load(base)
		if err != nil {
			return nil, err
		}
		r, err := newResource(base, data)
		if err != nil {
			return nil, err
		}
		c.resources[r.url] = r
	}
	br := c.resources[base+"#"]
	url, err := br.resolveURL(ref)
	if err != nil {
		return nil, err
	}
	if _, ok := c.resources[url]; !ok {
		ir, err := br.resolveInternal(ref)
		if err != nil {
			return nil, err
		}
		if ir == nil {
			return c.Compile(url)
		} else {
			c.resources[ir.url] = ir
		}
	}
	r := c.resources[url]
	if err := c.compileResource(r); err != nil {
		return nil, err
	}
	return r.schema, nil
}

func (c Compiler) compileResource(r *resource) error {
	if r.schema == nil {
		url, ptr := split(r.url)
		if draft4 != nil {
			if err := draft4.validate(r.doc); err != nil {
				finishContext(err, draft4)
				return &SchemaError{r.url, err}
			}
		}
		_, err := c.compile(url, ptr, r.doc.(map[string]interface{}))
		return err
	}
	return nil
}

func (c Compiler) compile(url, ptr string, m map[string]interface{}) (*Schema, error) {
	var err error

	s := new(Schema)
	if ptr != "" {
		s.url = &url
		s.ptr = &ptr
		c.resources[url+ptr].schema = s
	}

	if ref, ok := m["$ref"]; ok {
		ref := ref.(string)
		s.ref, err = c.compileRef(url, ref)
		if err != nil {
			return nil, err
		}
		// All other properties in a "$ref" object MUST be ignored
		return s, nil
	}

	if t, ok := m["type"]; ok {
		switch t := t.(type) {
		case string:
			s.types = []string{t}
		case []interface{}:
			s.types = make([]string, len(t))
			for i, v := range t {
				s.types[i] = v.(string)
			}
		}
	}

	if e, ok := m["enum"]; ok {
		s.enum = e.([]interface{})
	}

	if not, ok := m["not"]; ok {
		s.not, err = c.compile(url, "", not.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
	}

	loadSchemas := func(pname string) ([]*Schema, error) {
		if pvalue, ok := m[pname]; ok {
			pvalue := pvalue.([]interface{})
			schemas := make([]*Schema, len(pvalue))
			for i, v := range pvalue {
				sch, err := c.compile(url, "", v.(map[string]interface{}))
				if err != nil {
					return nil, err
				}
				schemas[i] = sch
			}
			return schemas, nil
		}
		return nil, nil
	}
	if s.allOf, err = loadSchemas("allOf"); err != nil {
		return nil, err
	}
	if s.anyOf, err = loadSchemas("anyOf"); err != nil {
		return nil, err
	}
	if s.oneOf, err = loadSchemas("oneOf"); err != nil {
		return nil, err
	}

	loadInt := func(pname string) int {
		if num, ok := m[pname]; ok {
			return int(num.(float64))
		}
		return -1
	}
	s.minProperties, s.maxProperties = loadInt("minProperties"), loadInt("maxProperties")

	if req, ok := m["required"]; ok {
		req := req.([]interface{})
		s.required = make([]string, len(req))
		for i, pname := range req {
			s.required[i] = pname.(string)
		}
	}

	if props, ok := m["properties"]; ok {
		props := props.(map[string]interface{})
		s.properties = make(map[string]*Schema, len(props))
		for pname, pmap := range props {
			s.properties[pname], err = c.compile(url, "", pmap.(map[string]interface{}))
			if err != nil {
				return nil, err
			}
		}
	}

	if regexProps, ok := m["regexProperties"]; ok {
		s.regexProperties = regexProps.(bool)
	}

	if patternProps, ok := m["patternProperties"]; ok {
		patternProps := patternProps.(map[string]interface{})
		s.patternProperties = make(map[*regexp.Regexp]*Schema, len(patternProps))
		for pattern, pmap := range patternProps {
			s.patternProperties[regexp.MustCompile(pattern)], err = c.compile(url, "", pmap.(map[string]interface{}))
			if err != nil {
				return nil, err
			}
		}
	}

	if additionalProps, ok := m["additionalProperties"]; ok {
		switch additionalProps := additionalProps.(type) {
		case bool:
			if !additionalProps {
				s.additionalProperties = false
			}
		case map[string]interface{}:
			s.additionalProperties, err = c.compile(url, "", additionalProps)
			if err != nil {
				return nil, err
			}
		}
	}

	if deps, ok := m["dependencies"]; ok {
		deps := deps.(map[string]interface{})
		s.dependencies = make(map[string]interface{}, len(deps))
		for pname, pvalue := range deps {
			switch pvalue := pvalue.(type) {
			case map[string]interface{}:
				s.dependencies[pname], err = c.compile(url, "", pvalue)
				if err != nil {
					return nil, err
				}
			case []interface{}:
				props := make([]string, len(pvalue))
				for i, prop := range pvalue {
					props[i] = prop.(string)
				}
				s.dependencies[pname] = props
			}
		}
	}

	s.minItems, s.maxItems = loadInt("minItems"), loadInt("maxItems")

	if unique, ok := m["uniqueItems"]; ok {
		s.uniqueItems = unique.(bool)
	}

	if items, ok := m["items"]; ok {
		switch items := items.(type) {
		case map[string]interface{}:
			s.items, err = c.compile(url, "", items)
			if err != nil {
				return nil, err
			}
		case []interface{}:
			schemas := make([]*Schema, 0, len(items))
			for _, item := range items {
				ischema, err := c.compile(url, "", item.(map[string]interface{}))
				if err != nil {
					return nil, err
				}
				schemas = append(schemas, ischema)
			}
			s.items = schemas
		}

		if additionalItems, ok := m["additionalItems"]; ok {
			switch additionalItems := additionalItems.(type) {
			case bool:
				s.additionalItems = additionalItems
			case map[string]interface{}:
				s.additionalItems, err = c.compile(url, "", additionalItems)
				if err != nil {
					return nil, err
				}
			}
		} else {
			s.additionalItems = true
		}
	}

	s.minLength, s.maxLength = loadInt("minLength"), loadInt("maxLength")

	if pattern, ok := m["pattern"]; ok {
		s.pattern = regexp.MustCompile(pattern.(string))
	}

	if format, ok := m["format"]; ok {
		s.format = format.(string)
	}

	loadFloat := func(pname string) *float64 {
		if num, ok := m[pname]; ok {
			num := num.(float64)
			return &num
		}
		return nil
	}

	s.minimum = loadFloat("minimum")
	if s.minimum != nil {
		if exclusive, ok := m["exclusiveMinimum"]; ok {
			s.exclusiveMinimum = exclusive.(bool)
		}
	}

	s.maximum = loadFloat("maximum")
	if s.maximum != nil {
		if exclusive, ok := m["exclusiveMaximum"]; ok {
			s.exclusiveMaximum = exclusive.(bool)
		}
	}

	s.multipleOf = loadFloat("multipleOf")

	return s, nil
}
