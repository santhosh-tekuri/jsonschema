// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type resource struct {
	url     string
	doc     interface{}
	schemas map[string]*Schema
}

func decodeJson(data []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
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

func newResource(base string, data []byte) (*resource, error) {
	if strings.IndexByte(base, '#') != -1 {
		panic(fmt.Sprintf("BUG: newResource(%q)", base))
	}
	doc, err := decodeJson(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %q failed. Reason: %v", base, err)
	}
	return &resource{base, doc, make(map[string]*Schema)}, nil
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

func (r *resource) resolvePtr(ptr string) (string, interface{}, error) {
	if !strings.HasPrefix(ptr, "#/") {
		panic(fmt.Sprintf("BUG: resolvePtr(%q)", ptr))
	}
	base := r.url
	p := strings.TrimPrefix(ptr, "#/")
	var doc interface{} = r.doc
	for _, item := range strings.Split(p, "/") {
		item = strings.Replace(item, "~1", "/", -1)
		item = strings.Replace(item, "~0", "~", -1)
		item, err := url.PathUnescape(item)
		if err != nil {
			return "", nil, errors.New("unable to url unscape: " + item)
		}
		switch doc.(type) {
		case map[string]interface{}:
			if id, ok := doc.(map[string]interface{})["id"]; ok {
				if id, ok := id.(string); ok {
					if base, err = resolveURL(base, id); err != nil {
						return "", nil, err
					}
				}
			}
			doc = doc.(map[string]interface{})[item]
		case []interface{}:
			index, err := strconv.Atoi(item)
			if err != nil {
				return "", nil, fmt.Errorf("invalid $ref %q, reason: %s", ptr, err)
			}
			arrLen := len(doc.([]interface{}))
			if index < 0 || index > arrLen {
				return "", nil, fmt.Errorf("invalid $ref %q, reason: array index outofrange", ptr)
			}
			doc = doc.([]interface{})[index]
		default:
			return "", nil, errors.New("invalid $ref " + ptr)
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

func resolveIDs(base string, m map[string]interface{}, ids map[string]interface{}) error {
	if id, ok := m["id"]; ok {
		b, err := resolveURL(base, id.(string))
		if err != nil {
			return err
		}
		base = b
		ids[base] = m
	}
	if m, ok := m["not"]; ok {
		if err := resolveIDs(base, m.(map[string]interface{}), ids); err != nil {
			return err
		}
	}

	resolveArray := func(pname string) error {
		if arr, ok := m[pname]; ok {
			for _, m := range arr.([]interface{}) {
				if err := resolveIDs(base, m.(map[string]interface{}), ids); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := resolveArray("allOf"); err != nil {
		return err
	}
	if err := resolveArray("anyOf"); err != nil {
		return err
	}
	if err := resolveArray("oneOf"); err != nil {
		return err
	}

	resolveMap := func(pname string) error {
		if props, ok := m[pname]; ok {
			for _, m := range props.(map[string]interface{}) {
				if err := resolveIDs(base, m.(map[string]interface{}), ids); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := resolveMap("definitions"); err != nil {
		return err
	}
	if err := resolveMap("properties"); err != nil {
		return err
	}
	if err := resolveMap("patternProperties"); err != nil {
		return err
	}
	if additionalProps, ok := m["additionalProperties"]; ok {
		if additionalProps, ok := additionalProps.(map[string]interface{}); ok {
			if err := resolveIDs(base, additionalProps, ids); err != nil {
				return err
			}
		}
	}

	if deps, ok := m["dependencies"]; ok {
		for _, pvalue := range deps.(map[string]interface{}) {
			if m, ok := pvalue.(map[string]interface{}); ok {
				if err := resolveIDs(base, m, ids); err != nil {
					return err
				}
			}
		}
	}

	if items, ok := m["items"]; ok {
		switch items := items.(type) {
		case map[string]interface{}:
			if err := resolveIDs(base, items, ids); err != nil {
				return err
			}
		case []interface{}:
			for _, item := range items {
				if err := resolveIDs(base, item.(map[string]interface{}), ids); err != nil {
					return err
				}
			}
		}
		if additionalItems, ok := m["additionalItems"]; ok {
			if additionalItems, ok := additionalItems.(map[string]interface{}); ok {
				if err := resolveIDs(base, additionalItems, ids); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
