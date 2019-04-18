// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"strings"
	"testing"
)

type test struct {
	str   string
	valid bool
}

func TestIsDateTime(t *testing.T) {
	tests := []test{
		{"1985-04-12T23:20:50.52Z", true},
		{"1996-12-19T16:39:57-08:00", true},
		{"1990-12-31T23:59:59Z", true},
		{"1990-12-31T15:59:59-08:00", true},
		{"1937-01-01T12:00:27.87+00:20", true},
		{"1963-06-19T08:30:06.283185Z", true},
		{"06/19/1963 08:30:06 PST", false},
		{"2013-350T01:01:01", false},
	}
	for i, test := range tests {
		if test.valid != isDateTime(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsDate(t *testing.T) {
	tests := []test{
		{"1963-06-19", true},
		{"06/19/1963", false},
		{"2013-350", false}, // only RFC3339 not all of ISO 8601 are valid
	}
	for i, test := range tests {
		if test.valid != isDate(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsTime(t *testing.T) {
	tests := []test{
		{"08:30:06.283185Z", true},
		{"08:30:06 PST", false},
		{"01:01:01,1111", false}, // only RFC3339 not all of ISO 8601 are valid
	}
	for i, test := range tests {
		if test.valid != isTime(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsHostname(t *testing.T) {
	tests := []test{
		{"www.example.com", true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 61), true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 61) + ".", true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 62) + ".", false}, // length more than 253 characters long
		{"www..com", false}, // empty label
		{"-a-host-name-that-starts-with--", false},
		{"not_a_valid_host_name", false},
		{"a-vvvvvvvvvvvvvvvveeeeeeeeeeeeeeeerrrrrrrrrrrrrrrryyyyyyyyyyyyyyyy-long-host-name-component", false},
		{"www.example-.com", false}, // label ends with a hyphen
	}
	for i, test := range tests {
		if test.valid != isHostname(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsEmail(t *testing.T) {
	tests := []test{
		{"joe.bloggs@example.com", true},
		{"2962", false},                                   // no "@" character
		{strings.Repeat("a", 244) + "@google.com", false}, // more than 254 characters long
		{strings.Repeat("a", 65) + "@google.com", false},  // local part more than 64 characters long
		{"santhosh@-google.com", false},                   // invalid domain name
	}
	for i, test := range tests {
		if test.valid != isEmail(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsIPV4(t *testing.T) {
	tests := []test{
		{"192.168.0.1", true},
		{"192.168.0.test", false},  // non-integer component
		{"127.0.0.0.1", false},     // too many components
		{"256.256.256.256", false}, // out-of-range values
		{"127.0", false},           // without 4 components
		{"0x7f000001", false},      // an integer
	}
	for i, test := range tests {
		if test.valid != isIPV4(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsIPV6(t *testing.T) {
	tests := []test{
		{"::1", true},
		{"192.168.0.1", false},                     // is IPV4
		{"12345::", false},                         // out-of-range values
		{"1:1:1:1:1:1:1:1:1:1:1:1:1:1:1:1", false}, // too many components
		{"::laptop", false},                        // containing illegal characters
	}
	for i, test := range tests {
		if test.valid != isIPV6(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsURI(t *testing.T) {
	tests := []test{
		{"http://foo.bar/?baz=qux#quux", true},
		{"//foo.bar/?baz=qux#quux", false}, // an invalid protocol-relative URI Reference
		{"\\\\WINDOWS\\fileshare", false},  // an invalid URI
		{"abc", false},                     // an invalid URI though valid URI reference
	}
	for i, test := range tests {
		if test.valid != isURI(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsURITemplate(t *testing.T) {
	tests := []test{
		{"http://example.com/dictionary/{term:1}/{term}", true},
		{"http://example.com/dictionary/{term:1}/{term", false},
		{"http://example.com/dictionary", true}, // without variables
		{"dictionary/{term:1}/{term}", true},    // relative url-template
	}
	for i, test := range tests {
		if test.valid != isURITemplate(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsRegex(t *testing.T) {
	tests := []test{
		{"([abc])+\\s+$", true},
		{"^(abc]", false}, // unclosed parenthesis
	}
	for i, test := range tests {
		if test.valid != isRegex(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsJSONPointer(t *testing.T) {
	tests := []test{
		{"", true}, // empty
		{"/ ", true},
		{"/foo/baz", true},
		{"/foo/bar~0/baz~1/%a", true},
		{"/g|h", true},
		{"/i\\j", true},
		{"/k\"l", true},
		{"/foo//bar", true},   // empty segment
		{"/foo/bar/", true},   // last empty segment
		{"/foo/-", true},      // last array position
		{"/foo/-/bar", true},  // - used as object member
		{"/~1~0~0~1~1", true}, // multiple escape characters
		{"/foo/baz~", false},  // ~ not escaped
		{"/~-1", false},       // wrong escape character
		{"/~~", false},        // multiple characters not escaped
		// escaped with fractional part
		{"/~1.1", true},
		{"/~0.1", true},
		// uri fragment identifier
		{"#", false},
		{"#/", false},
		{"#a", false},
		// some escaped, but not all
		{"/~0~", false},
		{"/~0/~", false},
		{"/~0/~", false},
		// isn't empty nor starts with /
		{"a", false},
		{"0", false},
		{"a/a", false},
	}
	for i, test := range tests {
		if test.valid != isJSONPointer(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestRelativeJSONPointer(t *testing.T) {
	tests := []test{
		{"1", true},             // upwards RJP
		{"0/foo/bar", true},     // downwards RJP
		{"2/0/baz/1/zip", true}, // up and then down RJP, with array index
		{"0#", true},            // taking the member or index name
		{"/foo/bar", false},     // valid json-pointer, but invalid RJP
	}
	for i, test := range tests {
		if test.valid != isRelativeJSONPointer(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}
