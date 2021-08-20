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

func toAbs(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return s, nil
	}
	return filepath.Abs(s)
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
