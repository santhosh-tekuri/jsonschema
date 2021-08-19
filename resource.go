// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type resource struct {
	url          string
	loc          string
	doc          interface{}
	draft        *Draft
	subresources map[string]*resource
	schema       *Schema
}

// DecodeJSON decodes json document from r.
//
// Note that number is decoded into json.Number instead of as a float64
func DecodeJSON(r io.Reader) (interface{}, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	var doc interface{}
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	if t, _ := decoder.Token(); t != nil {
		return nil, fmt.Errorf("invalid character %v after top-level value", t)
	}
	return doc, nil
}

func newResource(url string, r io.Reader) (*resource, error) {
	if strings.IndexByte(url, '#') != -1 {
		panic(fmt.Sprintf("BUG: newResource(%q)", url))
	}
	doc, err := DecodeJSON(r)
	if err != nil {
		return nil, fmt.Errorf("jsonschema: invalid json %q reason: %v", url, err)
	}
	return &resource{
		url: url,
		loc: "#",
		doc: doc,
	}, nil
}

func resolveURL(base, ref string) (string, error) {
	if ref == "" {
		return base, nil
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if refURL.IsAbs() {
		return normalize(ref), nil
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if baseURL.IsAbs() {
		return normalize(baseURL.ResolveReference(refURL).String()), nil
	}

	// filepath resolving
	base, _ = split(base)
	ref, fragment := split(ref)
	if ref == "" {
		return base + fragment, nil
	}
	dir, _ := filepath.Split(base)
	return filepath.Join(dir, ref) + fragment, nil
}

func isPtrFragment(f string) bool {
	if !strings.HasPrefix(f, "#") {
		panic(fmt.Sprintf("BUG: isPtrFragment(%q)", f))
	}
	return len(f) == 1 || f[1] == '/'
}

func (r *resource) resolveID(base resource, d interface{}) (newBase resource, err error) {
	if d, ok := d.(map[string]interface{}); ok {
		if id, ok := d[r.draft.id]; ok {
			if id, ok := id.(string); ok {
				url, err := resolveURL(base.url, id)
				if err != nil {
					return resource{}, err
				}
				return resource{url: url, doc: d}, nil
			}
		}
	}
	return base, nil
}

func (r *resource) resolvePtr(ptr string, base resource) (resource, interface{}, error) {
	if !strings.HasPrefix(ptr, "#/") {
		panic(fmt.Sprintf("BUG: resolvePtr(%q)", ptr))
	}
	u := base.url + ptr
	doc := base.doc
	p := strings.TrimPrefix(ptr, "#/")
	for _, item := range strings.Split(p, "/") {
		item = strings.Replace(item, "~1", "/", -1)
		item = strings.Replace(item, "~0", "~", -1)
		item, err := url.PathUnescape(item)
		if err != nil {
			return resource{}, nil, fmt.Errorf("jsonschema: invalid jsonpointer %q", ptr)
		}
		switch d := doc.(type) {
		case map[string]interface{}:
			doc = d[item]
		case []interface{}:
			index, err := strconv.Atoi(item)
			if err != nil {
				return resource{}, nil, fmt.Errorf("jsonschema: %q not found", u)
			}
			if index < 0 || index >= len(d) {
				return resource{}, nil, fmt.Errorf("jsonschema: %q not found", u)
			}
			doc = d[index]
		default:
			return resource{}, nil, fmt.Errorf("jsonschema: %q not found", u)
		}
		base, err = r.resolveID(base, doc)
		if err != nil {
			return resource{}, nil, err
		}
	}
	return base, doc, nil
}

func split(uri string) (string, string) {
	hash := strings.IndexByte(uri, '#')
	if hash == -1 {
		return uri, "#"
	}
	return uri[0:hash], uri[hash:]
}

func normalize(url string) string {
	base, fragment := split(url)
	if rootFragment(fragment) {
		fragment = "#"
	}
	return base + fragment
}

func rootFragment(fragment string) bool {
	return fragment == "" || fragment == "#" || fragment == "#/"
}

func resolveIDs(draft *Draft, base string, v interface{}, ids map[string]map[string]interface{}) error {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	addID := func(u string) error {
		if u != "" {
			b, err := resolveURL(base, u)
			if err != nil {
				return err
			}
			base = b
			if _, ok := ids[base]; ok {
				return fmt.Errorf("jsonschema: ambigious canonical uri %q", base)
			}
			ids[base] = m
		}
		return nil
	}
	u := ""
	if id, ok := m[draft.id]; ok {
		u = id.(string)
	}
	if err := addID(u); err != nil {
		return err
	}
	if anchor, ok := m["$anchor"]; draft.version >= 2019 && ok {
		if err := addID(u + "#" + anchor.(string)); err != nil {
			return err
		}
	}
	if dynamicAnchor, ok := m["$dynamicAnchor"]; draft.version >= 2020 && ok {
		if err := addID(u + "#" + dynamicAnchor.(string)); err != nil {
			return err
		}
	}

	resolveIDs := func(v interface{}) error {
		return resolveIDs(draft, base, v, ids)
	}

	schemaKeys := []string{"not", "additionalProperties", "items", "additionalItems"}
	if draft.version >= 6 {
		schemaKeys = append(schemaKeys, "propertyNames", "contains")
	}
	if draft.version >= 7 {
		schemaKeys = append(schemaKeys, "if", "then", "else")
	}
	if draft.version >= 2019 {
		schemaKeys = append(schemaKeys, "unevaluatedProperties", "unevaluatedItems")
	}
	for _, pname := range schemaKeys {
		if m, ok := m[pname]; ok {
			if err := resolveIDs(m); err != nil {
				return err
			}
		}
	}

	schemasKeys := []string{"items", "allOf", "anyOf", "oneOf"}
	if draft.version >= 2020 {
		schemasKeys = append(schemasKeys, "prefixItems")
	}
	for _, pname := range schemasKeys {
		if pvalue, ok := m[pname]; ok {
			if arr, ok := pvalue.([]interface{}); ok {
				for _, m := range arr {
					if err := resolveIDs(m); err != nil {
						return err
					}
				}
			}
		}
	}

	mapKeys := []string{"definitions", "properties", "patternProperties", "dependencies"}
	if draft.version >= 2019 {
		mapKeys = append(mapKeys, "$defs", "dependentSchemas")
	}
	for _, pname := range mapKeys {
		if props, ok := m[pname]; ok {
			for _, m := range props.(map[string]interface{}) {
				if err := resolveIDs(m); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
