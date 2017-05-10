// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type resource struct {
	url    string
	doc    interface{}
	schema *Schema
}

func newResource(base string, data []byte) (*resource, error) {
	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &resource{normalize(base), doc, nil}, nil
}

func (r *resource) resolveURL(ref string) (string, error) {
	if ref == "" {
		return r.url, nil
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if refURL.IsAbs() {
		return normalize(ref), nil
	}

	baseURL, err := url.Parse(r.url)
	if err != nil {
		return "", err
	}
	if baseURL.IsAbs() {
		return normalize(baseURL.ResolveReference(refURL).String()), nil
	} else {
		base, _ := split(r.url)
		ref, fragment := split(ref)
		if ref == "" {
			return base + fragment, nil
		}
		dir, _ := filepath.Split(base)
		return filepath.Join(dir, ref) + fragment, nil
	}
}

func (r *resource) resolveFromID(doc interface{}, url string) (interface{}, error) {
	switch doc := doc.(type) {
	case map[string]interface{}:
		if id, ok := doc["$id"]; ok {
			id, err := r.resolveURL(id.(string))
			if err != nil {
				return nil, err
			}
			if id == url {
				return doc, nil
			}
		}
		for _, v := range doc {
			if v, ok := v.(map[string]interface{}); ok {
				if v, err := r.resolveFromID(v, url); err == nil {
					if v != nil {
						return v, nil
					}
				} else {
					return nil, err
				}
			}
		}
	case []interface{}:
		for _, v := range doc {
			if v, err := r.resolveFromID(v, url); err == nil {
				if v != nil {
					return v, nil
				}
			} else {
				return nil, err
			}
		}
	}
	return nil, nil
}

func (r *resource) resolvePtr(doc interface{}, ptr string) (interface{}, error) {
	p := strings.TrimPrefix(ptr, "#/")
	for _, item := range strings.Split(p, "/") {
		item = strings.Replace(item, "~1", "/", -1)
		item = strings.Replace(item, "~0", "~", -1)
		item, err := url.PathUnescape(item)
		if err != nil {
			return nil, errors.New("unable to url unscape: " + item)
		}
		switch doc.(type) {
		case map[string]interface{}:
			doc = doc.(map[string]interface{})[item]
		case []interface{}:
			index, err := strconv.Atoi(item)
			if err != nil {
				return nil, fmt.Errorf("invalid $ref %q, reason: %s", ptr, err)
			}
			arrLen := len(doc.([]interface{}))
			if index < 0 || index > arrLen {
				return nil, fmt.Errorf("invalid $ref %q, reason: array index outofrange", ptr)
			}
			doc = doc.([]interface{})[index]
		default:
			return nil, errors.New("invalid $ref " + ptr)
		}
	}
	return doc, nil
}

func (r *resource) resolveInternal(ref string) (*resource, error) {
	if rootFragment(ref) {
		return r, nil
	}
	u, err := r.resolveURL(ref)
	if err != nil {
		return nil, err
	}
	doc, err := r.resolveFromID(r.doc, u)
	if err != nil {
		return nil, err
	}
	if doc != nil {
		return &resource{normalize(u), doc, nil}, nil
	}

	if strings.HasPrefix(ref, "#/") {
		doc, err := r.resolvePtr(r.doc, ref)
		if err != nil {
			return nil, err
		}
		u, _ := split(r.url)
		return &resource{u + ref, doc, nil}, nil
	}

	return nil, nil
}

func split(uri string) (string, string) {
	if hash := strings.IndexByte(uri, '#'); hash == -1 {
		return uri, "#"
	} else {
		return uri[0:hash], uri[hash:]
	}
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
