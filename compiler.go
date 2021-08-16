// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// A Draft represents json-schema draft
type Draft struct {
	meta    *Schema
	id      string // property name used to represent schema id.
	version int
}

var latest = Draft2020

// A Compiler represents a json-schema compiler.
//
// Currently draft4, draft6 and draft7 are supported
type Compiler struct {
	// Draft represents the draft used when '$schema' attribute is missing.
	//
	// This defaults to latest draft (currently draft2019-09).
	Draft     *Draft
	resources map[string]*resource

	// Extensions is used to register extensions.
	extensions map[string]extension

	// ExtractAnnotations tells whether schema annotations has to be extracted
	// in compiled Schema or not.
	ExtractAnnotations bool

	// LoadURL loads the document at given URL.
	//
	// If nil, package global LoadURL is used.
	LoadURL func(s string) (io.ReadCloser, error)

	// AssertFormat for specifications >= draft2019-09.
	AssertFormat bool

	// AssertContent for specifications >= draft2019-09.
	AssertContent bool
}

// NewCompiler returns a json-schema Compiler object.
// if '$schema' attribute is missing, it is treated as draft7. to change this
// behavior change Compiler.Draft value
func NewCompiler() *Compiler {
	return &Compiler{Draft: latest, resources: make(map[string]*resource), extensions: make(map[string]extension)}
}

// AddResource adds in-memory resource to the compiler.
//
// Note that url must not have fragment
func (c *Compiler) AddResource(url string, r io.Reader) error {
	res, err := newResource(url, r)
	if err != nil {
		return err
	}
	c.resources[res.url] = res
	return nil
}

// MustCompile is like Compile but panics if the url cannot be compiled to *Schema.
// It simplifies safe initialization of global variables holding compiled Schemas.
func (c *Compiler) MustCompile(url string) *Schema {
	s, err := c.Compile(url)
	if err != nil {
		panic(fmt.Sprintf("jsonschema: %#v", err))
	}
	return s
}

// Compile parses json-schema at given url returns, if successful,
// a Schema object that can be used to match against json.
//
// error returned will be of type *SchemaError
func (c *Compiler) Compile(url string) (*Schema, error) {
	sch, err := c.compileURL(url, nil, "#")
	if err != nil {
		err = &SchemaError{url, err}
	}
	return sch, err
}

func (c *Compiler) compileURL(url string, stack []schemaRef, ptr string) (*Schema, error) {
	switch url {
	case "http://json-schema.org/draft/2020-12/schema#", "https://json-schema.org/draft/2020-12/schema#":
		return Draft2020.meta, nil
	case "http://json-schema.org/draft/2019-09/schema#", "https://json-schema.org/draft/2019-09/schema#":
		return Draft2019.meta, nil
	case "http://json-schema.org/draft-07/schema#", "https://json-schema.org/draft-07/schema#":
		return Draft7.meta, nil
	case "http://json-schema.org/draft-06/schema#", "https://json-schema.org/draft-06/schema#":
		return Draft6.meta, nil
	case "http://json-schema.org/draft-04/schema#", "https://json-schema.org/draft-04/schema#":
		return Draft4.meta, nil
	}
	b, f := split(url)
	if _, ok := c.resources[b]; !ok {
		r, err := c.loadURL(b)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		if err := c.AddResource(b, r); err != nil {
			return nil, err
		}
	}
	r := c.resources[b]
	if r.draft == nil {
		if m, ok := r.doc.(map[string]interface{}); ok {
			if url, ok := m["$schema"]; ok {
				if _, ok = url.(string); !ok {
					return nil, fmt.Errorf("invalid $schema %v", url)
				}
				switch normalize(url.(string)) {
				case "http://json-schema.org/schema#", "https://json-schema.org/schema#":
					r.draft = latest
				case "http://json-schema.org/draft/2020-12/schema#", "https://json-schema.org/draft/2020-12/schema#":
					r.draft = Draft2020
				case "http://json-schema.org/draft/2019-09/schema#", "https://json-schema.org/draft/2019-09/schema#":
					r.draft = Draft2019
				case "http://json-schema.org/draft-07/schema#", "https://json-schema.org/draft-07/schema#":
					r.draft = Draft7
				case "http://json-schema.org/draft-06/schema#", "https://json-schema.org/draft-06/schema#":
					r.draft = Draft6
				case "http://json-schema.org/draft-04/schema#", "https://json-schema.org/draft-04/schema#":
					r.draft = Draft4
				default:
					return nil, fmt.Errorf("unknown $schema %q", url)
				}
			}
		}
		if r.draft == nil {
			r.draft = c.Draft
		}
	}
	base, err := r.resolveID(*r, r.doc)
	if err != nil {
		return nil, err
	}
	return c.compileRef(r, stack, ptr, base, f)
}

func (c Compiler) loadURL(s string) (io.ReadCloser, error) {
	if c.LoadURL != nil {
		return c.LoadURL(s)
	}
	return LoadURL(s)
}

func (c *Compiler) compileRef(r *resource, stack []schemaRef, refPtr string, base resource, ref string) (*Schema, error) {
	ref, err := resolveURL(base.url, ref)
	if err != nil {
		return nil, err
	}
	if s, ok := r.schemas[ref]; ok {
		if err := checkLoop(stack, schemaRef{refPtr, s}); err != nil {
			return nil, err
		}
		return s, nil
	}

	u, f := split(ref)
	bu, _ := split(base.url)
	if u != bu || !isPtrFragment(f) {
		ids := make(map[string]map[string]interface{})
		if err := resolveIDs(r.draft, r.url, r.doc, ids); err != nil {
			return nil, err
		}
		id := normalize(u)
		if !isPtrFragment(f) {
			id = ref
		}
		if v, ok := ids[id]; ok {
			base = resource{url: u, doc: v}
		} else {
			// external resource
			b, _ := split(ref)
			if b == r.url {
				// infinite loop detected
				return nil, fmt.Errorf("jsonschema: invalid %s %q", refPtr, ref)
			}
			return c.compileURL(ref, stack, refPtr)
		}
	}

	doc := base.doc
	if !rootFragment(f) && isPtrFragment(f) {
		base, doc, err = r.resolvePtr(f, base)
		if err != nil {
			return nil, err
		}
	}
	ptr := f
	if rootFragment(ptr) {
		ptr = f
	} else {
		ptr = strings.TrimPrefix(ptr, "#/")
	}
	if err := c.validateSchema(r, ptr, doc); err != nil {
		return nil, err
	}

	s := newSchema(u, f, doc)
	r.schemas[ref] = s
	return c.compile(r, stack, schemaRef{refPtr, s}, base, doc)
}

func (c *Compiler) compile(r *resource, stack []schemaRef, sref schemaRef, base resource, m interface{}) (*Schema, error) {
	if sref.schema == nil {
		u, _ := split(base.url)
		sref.schema = newSchema(u, "", m)
	}
	switch m := m.(type) {
	case bool:
		sref.schema.Always = &m
		return sref.schema, nil
	default:
		return sref.schema, c.compileMap(r, stack, sref, base, m.(map[string]interface{}))
	}
}

func (c *Compiler) compileMap(r *resource, stack []schemaRef, sref schemaRef, base resource, m map[string]interface{}) error {
	if err := checkLoop(stack, sref); err != nil {
		return err
	}
	stack = append(stack, sref)

	var s = sref.schema
	var err error

	if ref, ok := m["$ref"]; ok {
		s.Ref, err = c.compileRef(r, stack, "$ref", base, ref.(string))
		if err != nil {
			return err
		}
		if r.draft.version < 2019 {
			// All other properties in a "$ref" object MUST be ignored
			return nil
		}
	}

	if r.draft.version >= 2019 {
		if ref, ok := m["$recursiveRef"]; ok {
			s.RecursiveRef, err = c.compileRef(r, stack, "$recursiveRef", base, ref.(string))
			if err != nil {
				return err
			}
		}
	}
	if r.draft.version >= 2020 {
		if ref, ok := m["$dynamicRef"]; ok {
			s.DynamicRef, err = c.compileRef(r, stack, "$dynamicRef", base, ref.(string))
			if err != nil {
				return err
			}
		}
	}

	if t, ok := m["type"]; ok {
		switch t := t.(type) {
		case string:
			s.Types = []string{t}
		case []interface{}:
			s.Types = toStrings(t)
		}
	}

	if e, ok := m["enum"]; ok {
		s.Enum = e.([]interface{})
		allPrimitives := true
		for _, item := range s.Enum {
			switch jsonType(item) {
			case "object", "array":
				allPrimitives = false
				break
			}
		}
		s.enumError = "enum failed"
		if allPrimitives {
			if len(s.Enum) == 1 {
				s.enumError = fmt.Sprintf("value must be %#v", s.Enum[0])
			} else {
				strEnum := make([]string, len(s.Enum))
				for i, item := range s.Enum {
					strEnum[i] = fmt.Sprintf("%#v", item)
				}
				s.enumError = fmt.Sprintf("value must be one of %s", strings.Join(strEnum, ", "))
			}
		}
	}

	compile := func(stack []schemaRef, ptr string, m interface{}) (*Schema, error) {
		var err error
		base, err = r.resolveID(base, m)
		if err != nil {
			return nil, err
		}
		return c.compile(r, stack, schemaRef{ptr, nil}, base, m)
	}

	loadSchema := func(pname string, stack []schemaRef) (*Schema, error) {
		if pvalue, ok := m[pname]; ok {
			return compile(stack, escape(pname), pvalue)
		}
		return nil, nil
	}

	if s.Not, err = loadSchema("not", stack); err != nil {
		return err
	}

	loadSchemas := func(pname string, stack []schemaRef) ([]*Schema, error) {
		if pvalue, ok := m[pname]; ok {
			pvalue := pvalue.([]interface{})
			schemas := make([]*Schema, len(pvalue))
			for i, v := range pvalue {
				sch, err := compile(stack, escape(pname)+"/"+strconv.Itoa(i), v)
				if err != nil {
					return nil, err
				}
				schemas[i] = sch
			}
			return schemas, nil
		}
		return nil, nil
	}
	if s.AllOf, err = loadSchemas("allOf", stack); err != nil {
		return err
	}
	if s.AnyOf, err = loadSchemas("anyOf", stack); err != nil {
		return err
	}
	if s.OneOf, err = loadSchemas("oneOf", stack); err != nil {
		return err
	}

	loadInt := func(pname string) int {
		if num, ok := m[pname]; ok {
			i, _ := num.(json.Number).Int64()
			return int(i)
		}
		return -1
	}
	s.MinProperties, s.MaxProperties = loadInt("minProperties"), loadInt("maxProperties")

	if req, ok := m["required"]; ok {
		s.Required = toStrings(req.([]interface{}))
	}

	if props, ok := m["properties"]; ok {
		props := props.(map[string]interface{})
		s.Properties = make(map[string]*Schema, len(props))
		for pname, pmap := range props {
			s.Properties[pname], err = compile(nil, "properties/"+escape(pname), pmap)
			if err != nil {
				return err
			}
		}
	}

	if regexProps, ok := m["regexProperties"]; ok {
		s.RegexProperties = regexProps.(bool)
	}

	if patternProps, ok := m["patternProperties"]; ok {
		patternProps := patternProps.(map[string]interface{})
		s.PatternProperties = make(map[*regexp.Regexp]*Schema, len(patternProps))
		for pattern, pmap := range patternProps {
			s.PatternProperties[regexp.MustCompile(pattern)], err = compile(nil, "patternProperties/"+escape(pattern), pmap)
			if err != nil {
				return err
			}
		}
	}

	if additionalProps, ok := m["additionalProperties"]; ok {
		switch additionalProps := additionalProps.(type) {
		case bool:
			s.AdditionalProperties = additionalProps
		case map[string]interface{}:
			s.AdditionalProperties, err = compile(nil, "additionalProperties", additionalProps)
			if err != nil {
				return err
			}
		}
	}

	if deps, ok := m["dependencies"]; ok {
		deps := deps.(map[string]interface{})
		s.Dependencies = make(map[string]interface{}, len(deps))
		for pname, pvalue := range deps {
			switch pvalue := pvalue.(type) {
			case []interface{}:
				s.Dependencies[pname] = toStrings(pvalue)
			default:
				s.Dependencies[pname], err = compile(stack, "dependencies/"+escape(pname), pvalue)
				if err != nil {
					return err
				}
			}
		}
	}

	if r.draft.version >= 2019 {
		if deps, ok := m["dependentRequired"]; ok {
			deps := deps.(map[string]interface{})
			s.DependentRequired = make(map[string][]string, len(deps))
			for pname, pvalue := range deps {
				s.DependentRequired[pname] = toStrings(pvalue.([]interface{}))
			}
		}
		if deps, ok := m["dependentSchemas"]; ok {
			deps := deps.(map[string]interface{})
			s.DependentSchemas = make(map[string]*Schema, len(deps))
			for pname, pvalue := range deps {
				s.DependentSchemas[pname], err = compile(stack, "dependentSchemas/"+escape(pname), pvalue)
				if err != nil {
					return err
				}
			}
		}
		if s.UnevaluatedProperties, err = loadSchema("unevaluatedProperties", nil); err != nil {
			return err
		}
		if s.UnevaluatedItems, err = loadSchema("unevaluatedItems", nil); err != nil {
			return err
		}
	}

	s.MinItems, s.MaxItems = loadInt("minItems"), loadInt("maxItems")

	if unique, ok := m["uniqueItems"]; ok {
		s.UniqueItems = unique.(bool)
	}

	if r.draft.version >= 2020 {
		if s.PrefixItems, err = loadSchemas("prefixItems", nil); err != nil {
			return err
		}
		if s.Items2020, err = loadSchema("items", nil); err != nil {
			return err
		}
	} else {
		if items, ok := m["items"]; ok {
			switch items := items.(type) {
			case []interface{}:
				s.Items, err = loadSchemas("items", nil)
				if err != nil {
					return err
				}
				if additionalItems, ok := m["additionalItems"]; ok {
					switch additionalItems := additionalItems.(type) {
					case bool:
						s.AdditionalItems = additionalItems
					case map[string]interface{}:
						s.AdditionalItems, err = compile(nil, "additionalItems", additionalItems)
						if err != nil {
							return err
						}
					}
				}
			default:
				s.Items, err = compile(nil, "items", items)
				if err != nil {
					return err
				}
			}
		}
	}

	s.MinLength, s.MaxLength = loadInt("minLength"), loadInt("maxLength")

	if pattern, ok := m["pattern"]; ok {
		s.Pattern = regexp.MustCompile(pattern.(string))
	}

	if format, ok := m["format"]; ok {
		s.Format = format.(string)
		s.format, _ = Formats[s.Format]
	}

	loadRat := func(pname string) *big.Rat {
		if num, ok := m[pname]; ok {
			r, _ := new(big.Rat).SetString(string(num.(json.Number)))
			return r
		}
		return nil
	}

	s.Minimum = loadRat("minimum")
	if exclusive, ok := m["exclusiveMinimum"]; ok {
		if exclusive, ok := exclusive.(bool); ok {
			if exclusive {
				s.Minimum, s.ExclusiveMinimum = nil, s.Minimum
			}
		} else {
			s.ExclusiveMinimum = loadRat("exclusiveMinimum")
		}
	}

	s.Maximum = loadRat("maximum")
	if exclusive, ok := m["exclusiveMaximum"]; ok {
		if exclusive, ok := exclusive.(bool); ok {
			if exclusive {
				s.Maximum, s.ExclusiveMaximum = nil, s.Maximum
			}
		} else {
			s.ExclusiveMaximum = loadRat("exclusiveMaximum")
		}
	}

	s.MultipleOf = loadRat("multipleOf")

	if c.ExtractAnnotations {
		if title, ok := m["title"]; ok {
			s.Title = title.(string)
		}
		if description, ok := m["description"]; ok {
			s.Description = description.(string)
		}
		s.Default = m["default"]
	}

	if r.draft.version >= 6 {
		if c, ok := m["const"]; ok {
			s.Constant = []interface{}{c}
		}
		if s.PropertyNames, err = loadSchema("propertyNames", nil); err != nil {
			return err
		}
		if s.Contains, err = loadSchema("contains", nil); err != nil {
			return err
		}
		if r.draft.version >= 2020 {
			// any item in an array that passes validation of the contains schema is considered "evaluated"
			s.ContainsEval = true
		}
		s.MinContains, s.MaxContains = 1, -1
	}

	if r.draft.version >= 7 {
		if m["if"] != nil {
			if s.If, err = loadSchema("if", stack); err != nil {
				return err
			}
			if s.Then, err = loadSchema("then", stack); err != nil {
				return err
			}
			if s.Else, err = loadSchema("else", stack); err != nil {
				return err
			}
		}
		if encoding, ok := m["contentEncoding"]; ok {
			s.ContentEncoding = encoding.(string)
			s.decoder, _ = Decoders[s.ContentEncoding]
		}
		if mediaType, ok := m["contentMediaType"]; ok {
			s.ContentMediaType = mediaType.(string)
			s.mediaType, _ = MediaTypes[s.ContentMediaType]
		}
		if c.ExtractAnnotations {
			if readOnly, ok := m["readOnly"]; ok {
				s.ReadOnly = readOnly.(bool)
			}
			if writeOnly, ok := m["writeOnly"]; ok {
				s.WriteOnly = writeOnly.(bool)
			}
			if examples, ok := m["examples"]; ok {
				s.Examples = examples.([]interface{})
			}
		}
	}

	if r.draft.version >= 2019 {
		s.decoder = nil
		s.mediaType = nil
		if !c.AssertFormat {
			s.format = nil
		}

		s.MinContains, s.MaxContains = loadInt("minContains"), loadInt("maxContains")
		if s.MinContains == -1 {
			s.MinContains = 1
		}
	}

	for name, ext := range c.extensions {
		es, err := ext.compiler.Compile(CompilerContext{c, r, stack, base}, m)
		if err != nil {
			return err
		}
		if es != nil {
			if s.Extensions == nil {
				s.Extensions = make(map[string]ExtSchema)
			}
			s.Extensions[name] = es
		}
	}

	return nil
}

func (c *Compiler) validateSchema(r *resource, ptr string, v interface{}) error {
	validate := func(meta *Schema) error {
		if meta == nil {
			return nil
		}
		if _, err := meta.validate(nil, v); err != nil {
			_ = addContext(ptr, "", err)
			finishSchemaContext(err, meta)
			finishInstanceContext(err)
			return &ValidationError{
				Message:     fmt.Sprintf("doesn't validate with %q", meta.URL+meta.Ptr),
				InstancePtr: absPtr(ptr),
				SchemaURL:   meta.URL,
				SchemaPtr:   "#",
				Causes:      []*ValidationError{err.(*ValidationError)},
			}
		}
		return nil
	}

	if err := validate(r.draft.meta); err != nil {
		return err
	}
	for _, ext := range c.extensions {
		if err := validate(ext.meta); err != nil {
			return err
		}
	}
	return nil
}

func toStrings(arr []interface{}) []string {
	s := make([]string, len(arr))
	for i, v := range arr {
		s[i] = v.(string)
	}
	return s
}

// SchemaRef captures schema and the path refering to it.
type schemaRef struct {
	ptr    string  // json-pointer leading to schema s
	schema *Schema // target schema
}

func checkLoop(stack []schemaRef, sref schemaRef) error {
	var loop bool
	for _, ref := range stack {
		if ref.schema == sref.schema {
			loop = true
			break
		}
	}
	if !loop {
		return nil
	}

	var path string
	for _, ref := range stack {
		if path == "" {
			path += ref.schema.URL + ref.schema.Ptr
		} else {
			path += "/" + ref.ptr
		}
	}
	return InfiniteLoopError(path + "/" + sref.ptr)
}
