// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

var Extensions = make(map[string]Extension)

type Extension struct {
	Meta     *Schema
	Compile  func(ctx CompilerContext, m map[string]interface{}) (interface{}, error)
	Validate func(ctx ValidationContext, s interface{}, v interface{}) error
}

type CompilerContext struct {
	c    *Compiler
	r    *resource
	base string
}

func (ctx CompilerContext) Compile(v interface{}) (*Schema, error) {
	return ctx.c.compile(ctx.r, nil, ctx.base, v)
}

func (ctx CompilerContext) CompileRef(ref string) (*Schema, error) {
	b, _ := split(ctx.base)
	return ctx.c.compileRef(ctx.r, b, ref)
}

type ValidationContext struct{}

func (ValidationContext) Validate(s *Schema, v interface{}) error {
	return s.validate(v)
}

func (ValidationContext) Error(schemaPtr string, format string, a ...interface{}) *ValidationError {
	return validationError(schemaPtr, format, a...)
}

func (ValidationError) Group(parent *ValidationError, causes ...error) error {
	return parent.add(causes...)
}
