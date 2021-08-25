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
	"runtime"
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

func newResource(url string, r io.Reader) (*resource, error) {
	if strings.IndexByte(url, '#') != -1 {
		panic(fmt.Sprintf("BUG: newResource(%q)", url))
	}
	doc, err := unmarshal(r)
	if err != nil {
		return nil, fmt.Errorf("jsonschema: invalid json %q reason: %v", url, err)
	}
	url, err = toAbs(url)
	if err != nil {
		return nil, err
	}
	return &resource{
		url: url,
		loc: "#",
		doc: doc,
	}, nil
}

func (r *resource) fillSubschemas(c *Compiler, res *resource) error {
	if err := c.validateSchema(r, res.doc, res.loc[1:]); err != nil {
		return err
	}

	if r.subresources == nil {
		r.subresources = make(map[string]*resource)
	}
	if err := r.draft.listSubschemas(res, r.baseURL(res.loc), r.subresources); err != nil {
		return err
	}

	// ensure subresource.url uniquness
	url2loc := make(map[string]string)
	for _, sr := range r.subresources {
		if sr.url != "" {
			if loc, ok := url2loc[sr.url]; ok {
				return fmt.Errorf("jsonschema: %s and %s in %s have same canonical-uri", loc, sr.loc, r.url)
			}
			url2loc[sr.url] = sr.loc
		}
	}

	return nil
}

func (r *resource) findResources(res *resource) []*resource {
	var result []*resource
	loc := res.loc + "/"
	for _, sr := range r.subresources {
		if strings.HasPrefix(sr.loc, loc) {
			result = append(result, sr)
		}
	}
	return result
}

func (r *resource) findResource(url string) *resource {
	if r.url == url {
		return r
	}
	for _, res := range r.subresources {
		if res.url == url {
			return res
		}
	}
	return nil
}

func (r *resource) resolveFragment(c *Compiler, sr *resource, f string) (*resource, error) {
	if f == "#" || f == "#/" {
		return sr, nil
	}

	// resolve by anchor
	if !strings.HasPrefix(f, "#/") {
		// check in given resource
		for _, anchor := range r.draft.anchors(sr.doc) {
			if anchor == f[1:] {
				return sr, nil
			}
		}

		// check in subresources
		for _, res := range r.subresources {
			if res.loc == sr.loc || strings.HasPrefix(res.loc, sr.loc+"/") {
				for _, anchor := range r.draft.anchors(res.doc) {
					if anchor == f[1:] {
						return res, nil
					}
				}
			}
		}
		return nil, nil
	}

	// resolve by ptr
	loc := sr.loc + f[1:]
	if res, ok := r.subresources[loc]; ok {
		return res, nil
	}

	// non-standrad location
	doc := r.doc
	for _, item := range strings.Split(loc[2:], "/") {
		item = strings.Replace(item, "~1", "/", -1)
		item = strings.Replace(item, "~0", "~", -1)
		item, err := url.PathUnescape(item)
		if err != nil {
			return nil, err
		}
		switch d := doc.(type) {
		case map[string]interface{}:
			if _, ok := d[item]; !ok {
				return nil, nil
			}
			doc = d[item]
		case []interface{}:
			index, err := strconv.Atoi(item)
			if err != nil {
				return nil, err
			}
			if index < 0 || index >= len(d) {
				return nil, nil
			}
			doc = d[index]
		default:
			return nil, nil
		}
	}

	id, err := r.draft.resolveID(r.baseURL(loc), doc)
	if err != nil {
		return nil, err
	}
	res := &resource{url: id, loc: loc, doc: doc}
	r.subresources[loc] = res
	if err := r.fillSubschemas(c, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *resource) baseURL(loc string) string {
	for {
		if sr, ok := r.subresources[loc]; ok {
			if sr.url != "" {
				return sr.url
			}
		}
		slash := strings.LastIndexByte(loc, '/')
		if slash == -1 {
			break
		}
		loc = loc[:slash]
	}
	return r.url
}

// url helpers ---

func toAbs(s string) (string, error) {
	// if windows absolute file path, convert to file url
	// because: net/url parses driver name as scheme
	if runtime.GOOS == "windows" && len(s) >= 3 && s[1:3] == `:\` {
		s = "file:///" + filepath.ToSlash(s)
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return s, nil
	}
	if s, err = filepath.Abs(s); err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return "file:///" + filepath.ToSlash(s), nil
	}
	return "file://" + s, nil
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
		return ref, nil
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if baseURL.IsAbs() {
		return baseURL.ResolveReference(refURL).String(), nil
	}

	// filepath resolving
	ref, fragment := split(ref)
	if filepath.IsAbs(ref) {
		return ref + fragment, nil
	}
	base, _ = split(base)
	if ref == "" {
		return base + fragment, nil
	}
	dir, _ := filepath.Split(base)
	return filepath.Join(dir, ref) + fragment, nil
}

func split(uri string) (string, string) {
	hash := strings.IndexByte(uri, '#')
	if hash == -1 {
		return uri, "#"
	}
	f := uri[hash:]
	if f == "#/" {
		f = "#"
	}
	return uri[0:hash], f
}

func (s *Schema) url() string {
	u, _ := split(s.Location)
	return u
}

func (s *Schema) loc() string {
	_, f := split(s.Location)
	if f == "#" {
		return "/"
	}
	return f[1:]
}

func unmarshal(r io.Reader) (interface{}, error) {
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
