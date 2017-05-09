// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package formats

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Format func(string) bool

var formats = map[string]Format{
	"date-time":  IsDateTime,
	"hostname":   IsHostname,
	"email":      IsEmail,
	"ip-address": IsIPV4,
	"ipv4":       IsIPV4,
	"ipv6":       IsIPV6,
	"uri":        IsURI,
	"uriref":     IsURIRef,
	"regex":      IsRegex,
}

func init() {
	formats["format"] = func(s string) bool {
		_, ok := formats[s]
		return ok
	}
}

func Register(name string, f Format) {
	formats[name] = f
}

func Get(name string) (Format, bool) {
	f, ok := formats[name]
	return f, ok
}

func IsDateTime(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return true
	}
	return false
}

// https://en.wikipedia.org/wiki/Hostname#Restrictions_on_valid_host_names
func IsHostname(s string) bool {
	// entire hostname (including the delimiting dots but not a trailing dot) has a maximum of 253 ASCII characters
	strLen := len(s)
	if strings.HasSuffix(s, ".") {
		strLen -= 1
	}
	if strLen > 253 {
		return false
	}

	// Hostnames are composed of series of labels concatenated with dots, as are all domain names
	for _, label := range strings.Split(s, ".") {
		// Each label must be from 1 to 63 characters long
		if labelLen := len(label); labelLen < 1 || labelLen > 63 {
			return false
		}

		// labels could not start with a digit or with a hyphen
		if first := s[0]; (first >= '0' && first <= '9') || (first == '-') {
			return false
		}

		// must not end with a hyphen
		if label[len(label)-1] == '-' {
			return false
		}

		// labels may contain only the ASCII letters 'a' through 'z' (in a case-insensitive manner),
		// the digits '0' through '9', and the hyphen ('-')
		for _, c := range label {
			if valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || (c == '-'); !valid {
				return false
			}
		}
	}

	return true
}

// https://en.wikipedia.org/wiki/Email_address
func IsEmail(s string) bool {
	// entire email address to be no more than 254 characters long
	if len(s) > 254 {
		return false
	}

	// email address is generally recognized as having two parts joined with an at-sign
	at := strings.LastIndexByte(s, '@')
	if at == -1 {
		return false
	}
	local := s[0:at]
	domain := s[at+1:]

	// local part may be up to 64 characters long
	if len(local) > 64 {
		return false
	}

	// domain may have a maximum of 255 characters[
	if len(domain) > 255 {
		return false
	}

	// domain must match the requirements for a hostname
	if !IsHostname(domain) {
		return false
	}

	//todo: some validations yet to be implemented

	return true
}

func IsIPV4(s string) bool {
	groups := strings.Split(s, ".")
	if len(groups) != 4 {
		return false
	}
	for _, group := range groups {
		n, err := strconv.Atoi(group)
		if err != nil {
			return false
		}
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func IsIPV6(s string) bool {
	if !strings.Contains(s, ":") {
		return false
	}
	return net.ParseIP(s) != nil
}

func IsURI(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs()
}

func IsURIRef(s string) bool {
	_, err := url.Parse(s)
	return err == nil
}

func IsRegex(s string) bool {
	_, err := regexp.Compile(s)
	return err == nil
}
