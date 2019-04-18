// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"bytes"
	"encoding/json"
)

// The MediaType type is a function, that validates
// whether the bytes represent data of given mediaType.
type MediaType func([]byte) error

var mediaTypes = map[string]MediaType{
	"application/json": validateJSON,
}

// Register registers MediaType object for given mediaType.
func RegisterMediaType(name string, mt MediaType) {
	mediaTypes[name] = mt
}

// Get returns MediaType object for given mediaType, if found.
func GetMediaType(name string) (MediaType, bool) {
	mt, ok := mediaTypes[name]
	return mt, ok
}

func validateJSON(b []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(b))
	var v interface{}
	return decoder.Decode(&v)
}
