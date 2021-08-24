// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"fmt"
	"strings"
)

// InvalidJSONTypeError is the error type returned by ValidateInterface.
// this tells that specified go object is not valid jsonType.
type InvalidJSONTypeError string

func (e InvalidJSONTypeError) Error() string {
	return fmt.Sprintf("jsonschema: invalid jsonType: %s", string(e))
}

// InfiniteLoopError is returned by Compile/Validate.
// this gives url#keywordLocation that lead to infinity loop.
type InfiniteLoopError string

func (e InfiniteLoopError) Error() string {
	return "jsonschema: infinite loop " + string(e)
}

func infiniteLoopError(stack []schemaRef, sref schemaRef) InfiniteLoopError {
	var path string
	for _, ref := range stack {
		if path == "" {
			path += ref.schema.Location
		} else {
			path += "/" + ref.path
		}
	}
	return InfiniteLoopError(path + "/" + sref.path)
}

// SchemaError is the error type returned by Compile.
type SchemaError struct {
	// SchemaURL is the url to json-schema that filed to compile.
	// This is helpful, if your schema refers to external schemas
	SchemaURL string

	// Err is the error that occurred during compilation.
	// It could be ValidationError, because compilation validates
	// given schema against the json meta-schema
	Err error
}

func (se *SchemaError) Error() string {
	return fmt.Sprintf("json-schema %q compilation failed", se.SchemaURL)
}

func (se *SchemaError) GoString() string {
	if _, ok := se.Err.(*ValidationError); ok {
		return fmt.Sprintf("json-schema %q compilation failed. Reason:\n%#v", se.SchemaURL, se.Err)
	}
	return fmt.Sprintf("json-schema %q compilation failed. Reason: %v", se.SchemaURL, se.Err)
}

// ValidationError is the error type returned by Validate.
type ValidationError struct {
	KeywordLocation         string             // validation path of validating keyword or schema
	AbsoluteKeywordLocation string             // absolute location of validating keyword or schema
	InstanceLocation        string             // location of the json value within the instance being validated
	Message                 string             // describes error
	Causes                  []*ValidationError // nested validation errors
}

func (ve *ValidationError) add(causes ...error) error {
	for _, cause := range causes {
		ve.Causes = append(ve.Causes, cause.(*ValidationError))
	}
	return ve
}

// MessageFmt returns the Message formatted, but does not include child Cause messages.
//
// Deprecated: use Error method
func (ve *ValidationError) MessageFmt() string {
	return ve.Error()
}

func (ve *ValidationError) Error() string {
	loc := ve.AbsoluteKeywordLocation
	loc = loc[strings.IndexByte(loc, '#')+1:]
	if loc == "" {
		loc = "/"
	}
	return fmt.Sprintf("I[%s] S[%s] %s", ve.InstanceLocation, loc, ve.Message)
}

func (ve *ValidationError) GoString() string {
	msg := ve.Error()
	for _, c := range ve.Causes {
		for _, line := range strings.Split(c.GoString(), "\n") {
			msg += "\n  " + line
		}
	}
	return msg
}

func joinPtr(ptr1, ptr2 string) string {
	if len(ptr1) == 0 {
		return ptr2
	}
	if len(ptr2) == 0 {
		return ptr1
	}
	return ptr1 + "/" + ptr2
}

func absPtr(ptr string) string {
	if ptr == "" {
		return "#"
	}
	if ptr[0] != '#' {
		return "#/" + ptr
	}
	return ptr
}
