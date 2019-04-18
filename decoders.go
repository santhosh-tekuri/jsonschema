// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/base64"
)

// The Decoder type is a function, that returns
// the bytes represented by encoded string.
type Decoder func(string) ([]byte, error)

var decoders = map[string]Decoder{
	"base64": base64.StdEncoding.DecodeString,
}

// Register registers Decoder object for given encoding.
func RegisterDecoder(name string, d Decoder) {
	decoders[name] = d
}

// Get returns Decoder object for given encoding, if found.
func GetDecoder(name string) (Decoder, bool) {
	d, ok := decoders[name]
	return d, ok
}
