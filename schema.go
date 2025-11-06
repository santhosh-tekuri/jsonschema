/*
Package jsonschema provides json-schema compilation and validation.

The schema is compiled against the version specified in "$schema" property.
If "$schema" property is missing, it uses latest draft which currently implemented
by this library.

You can force to use specific draft,  when "$schema" is missing, as follows:

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft4)

This package supports loading json-schema from filePath and fileURL.

see examples for usage.
*/
package jsonschema

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
)

// Schema is the representation of a compiled
// jsonschema.
type Schema struct {
	up                urlPtr
	resource          *Schema
	dynamicAnchors    map[string]*Schema
	allPropsEvaluated bool
	allItemsEvaluated bool
	numItemsEvaluated int

	DraftVersion int
	Location     string

	// type agnostic --
	Bool            *bool // boolean schema
	ID              string
	Ref             *Schema
	Anchor          string
	RecursiveRef    *Schema
	RecursiveAnchor bool
	DynamicRef      *DynamicRef
	DynamicAnchor   string // "" if not specified
	Types           *Types
	Enum            *Enum
	Const           *any
	Not             *Schema
	AllOf           []*Schema
	AnyOf           []*Schema
	OneOf           []*Schema
	If              *Schema
	Then            *Schema
	Else            *Schema
	Format          *Format

	// object --
	MaxProperties         *int
	MinProperties         *int
	Required              []string
	PropertyNames         *Schema
	Properties            map[string]*Schema
	PatternProperties     map[Regexp]*Schema
	AdditionalProperties  any            // nil or bool or *Schema
	Dependencies          map[string]any // value is []string or *Schema
	DependentRequired     map[string][]string
	DependentSchemas      map[string]*Schema
	UnevaluatedProperties *Schema

	// array --
	MinItems         *int
	MaxItems         *int
	UniqueItems      bool
	Contains         *Schema
	MinContains      *int
	MaxContains      *int
	Items            any // nil or []*Schema or *Schema
	AdditionalItems  any // nil or bool or *Schema
	PrefixItems      []*Schema
	Items2020        *Schema
	UnevaluatedItems *Schema

	// string --
	MinLength        *int
	MaxLength        *int
	Pattern          Regexp
	ContentEncoding  *Decoder
	ContentMediaType *MediaType
	ContentSchema    *Schema

	// number --
	Maximum          *big.Rat
	Minimum          *big.Rat
	ExclusiveMaximum *big.Rat
	ExclusiveMinimum *big.Rat
	MultipleOf       *big.Rat

	Extensions []SchemaExt

	// annotations --
	Title       string
	Description string
	Default     *any
	Comment     string
	ReadOnly    bool
	WriteOnly   bool
	Examples    []any
	Deprecated  bool
}

// --

type jsonType int

const (
	invalidType jsonType = 0
	nullType    jsonType = 1 << iota
	booleanType
	numberType
	integerType
	stringType
	arrayType
	objectType
)

func typeOf(v any) jsonType {
	if v == nil {
		return nullType
	}
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Bool:
		return booleanType
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return numberType
	case reflect.String:
		if _, ok := v.(json.Number); ok {
			return numberType
		}
		return stringType
	case reflect.Slice, reflect.Array:
		return arrayType
	case reflect.Map:
		return objectType
	default:
		return invalidType
	}
}

func typeFromString(s string) jsonType {
	switch s {
	case "null":
		return nullType
	case "boolean":
		return booleanType
	case "number":
		return numberType
	case "integer":
		return integerType
	case "string":
		return stringType
	case "array":
		return arrayType
	case "object":
		return objectType
	}
	return invalidType
}

func (jt jsonType) String() string {
	switch jt {
	case nullType:
		return "null"
	case booleanType:
		return "boolean"
	case numberType:
		return "number"
	case integerType:
		return "integer"
	case stringType:
		return "string"
	case arrayType:
		return "array"
	case objectType:
		return "object"
	}
	return ""
}

// --

// Types encapsulates list of json value types.
type Types int

func newTypes(v any) *Types {
	var types Types
	switch v := v.(type) {
	case string:
		types.Add(v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				types.Add(s)
			}
		}
	}
	if types.IsEmpty() {
		return nil
	}
	return &types
}

func (tt Types) IsEmpty() bool {
	return tt == 0
}

// Add specified json type. If typ is
// not valid json type it is ignored.
func (tt *Types) Add(typ string) {
	tt.add(typeFromString(typ))
}

func (tt *Types) add(t jsonType) {
	*tt = Types(int(*tt) | int(t))
}

func (tt Types) contains(t jsonType) bool {
	return int(tt)&int(t) != 0
}

func (tt Types) ToStrings() []string {
	types := []jsonType{
		nullType, booleanType, numberType, integerType,
		stringType, arrayType, objectType,
	}
	var arr []string
	for _, t := range types {
		if tt.contains(t) {
			arr = append(arr, t.String())
		}
	}
	return arr
}

func (tt Types) String() string {
	return fmt.Sprintf("%v", tt.ToStrings())
}

// --

type Enum struct {
	Values []any
	types  Types
}

func newEnum(arr []any) *Enum {
	var types Types
	for _, item := range arr {
		types.add(typeOf(item))
	}
	return &Enum{arr, types}
}

// --

type DynamicRef struct {
	Ref    *Schema
	Anchor string // "" if not specified
}

func newSchema(up urlPtr) *Schema {
	return &Schema{up: up, Location: up.String()}
}
