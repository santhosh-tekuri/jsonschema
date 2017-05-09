// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"fmt"
	"strings"
)

type SchemaError struct {
	SchemaURL string
	Err       error
}

func (se *SchemaError) Error() string {
	return fmt.Sprintf("schemaURL: %s\n%s", se.SchemaURL, se.Err)
}

type ValidationError struct {
	Message     string
	InstancePtr string
	SchemaURL   string
	SchemaPtr   string
	Causes      []*ValidationError
}

func (ve *ValidationError) add(causes ...error) error {
	for _, cause := range causes {
		addContext(ve.InstancePtr, ve.SchemaPtr, cause)
		ve.Causes = append(ve.Causes, cause.(*ValidationError))
	}
	return ve
}

func (ve *ValidationError) Error() string {
	msg := fmt.Sprintf("I[%s] S[%s] %s", ve.InstancePtr, ve.SchemaPtr, ve.Message)
	for _, c := range ve.Causes {
		for _, line := range strings.Split(c.Error(), "\n") {
			msg += "\n    " + line
		}
	}
	return msg
}

func validationError(schemaPtr string, format string, a ...interface{}) *ValidationError {
	return &ValidationError{fmt.Sprintf(format, a...), "", "", schemaPtr, nil}
}

func addContext(instancePtr, schemaPtr string, err error) error {
	ve := err.(*ValidationError)
	if len(ve.SchemaURL) == 0 {
		ve.InstancePtr = joinPtr(instancePtr, ve.InstancePtr)
		ve.SchemaPtr = joinPtr(schemaPtr, ve.SchemaPtr)
		for _, cause := range ve.Causes {
			addContext(instancePtr, schemaPtr, cause)
		}
	}
	return ve
}

func finishContext(err error, s *Schema) {
	ve := err.(*ValidationError)
	if len(ve.SchemaURL) == 0 {
		if len(ve.InstancePtr) == 0 {
			ve.InstancePtr = "#"
		} else {
			ve.InstancePtr = "#/" + ve.InstancePtr
		}
		ve.SchemaURL = *s.url
		ve.SchemaPtr = *s.ptr + "/" + ve.SchemaPtr
		for _, cause := range ve.Causes {
			finishContext(cause, s)
		}
	}
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
