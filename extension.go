// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

// ExtCompiler compiles custom keyword(s) into ExtSchema.
type ExtCompiler interface {
	// Compile compiles the schema m and returns its compiled representation.
	// if the schema m does not contain the keywords defined by this extension,
	// compiled representation nil should be returned.
	Compile(ctx CompilerContext, m map[string]interface{}) (ExtSchema, error)
}

// ExtSchema is schema representation of custom keyword(s)
type ExtSchema interface {
	// Validate validates the json value v with this ExtSchema.
	// Returned error must be *ValidationError.
	Validate(ctx ValidationContext, v interface{}) error
}

type extension struct {
	meta     *Schema
	compiler ExtCompiler
}

// RegisterExtension registers custom keyword(s) into this compiler.
//
// name is extension name, used only to avoid name collisions.
// meta captures the metaschema for the new keywords.
// This is used to validate the schema before calling ext.Compile.
func (c *Compiler) RegisterExtension(name string, meta *Schema, ext ExtCompiler) {
	c.extensions[name] = extension{meta, ext}
}

// CompilerContext ---

// CompilerContext provides additional context required in compiling for extension.
type CompilerContext struct {
	c     *Compiler
	r     *resource
	stack []schemaRef
	base  resource
}

// Compile compiles given value v into *Schema. This is useful in implementing
// keyword like allOf/oneOf.
//
// vPtr is the jsonpointer to v.
//
// applicableOnSameInstance tells whether current schema and the given schema v
// are applied on same instance value. this is used to detect infinite loop in schema.
func (ctx CompilerContext) Compile(v interface{}, vPtr string, applicableOnSameInstance bool) (*Schema, error) {
	var stack []schemaRef
	if applicableOnSameInstance {
		stack = ctx.stack
	}
	_ = stack
	return nil, nil
	//return ctx.c.compile(ctx.r, stack, schemaRef{vPtr, nil}, ctx.base, v)
}

// CompileRef compiles the schema referenced by ref uri
//
// refPtr is the jsonpointer to ref.
//
// applicableOnSameInstance tells whether current schema and the given schema v
// are applied on same instance value. this is used to detect infinite loop in schema.
func (ctx CompilerContext) CompileRef(ref string, refPtr string, applicableOnSameInstance bool) (*Schema, error) {
	var stack []schemaRef
	if applicableOnSameInstance {
		stack = ctx.stack
	}
	_ = stack
	return nil, nil
	//return ctx.c.compileRef(ctx.r, stack, refPtr, ctx.base, ref)
}

// ValidationContext ---

// ValidationContext provides additional context required in validating for extension.
type ValidationContext struct {
	scope []*Schema
}

// Validate validates schema s with value v. Extension must use this method instead of
// *Schema.ValidateInterface method. This will be useful in implementing keywords like
// allOf/oneOf
func (ctx ValidationContext) Validate(s *Schema, v interface{}) error {
	_, err := s.validate(ctx.scope, v)
	return err
}

// Error used to construct validation error by extensions. schemaPtr is relative json pointer.
func (ValidationContext) Error(schemaPtr string, format string, a ...interface{}) *ValidationError {
	return validationError(schemaPtr, format, a...)
}

// Group is used by extensions to group multiple errors as causes to parent error.
// This is useful in implementing keywords like allOf where each schema specified
// in allOf can result a validationError.
func (ValidationError) Group(parent *ValidationError, causes ...error) error {
	return parent.add(causes...)
}
