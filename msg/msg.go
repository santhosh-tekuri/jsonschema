package msg

import "fmt"
import "math/big"
import "strings"

// Empty captures error fields for empty message.
type Empty struct{}

func (Empty) String() string {
	return ""
}

// False captures error fields for false boolean schema.
type False struct{}

func (False) String() string {
	return "not allowed"
}

// Type captures error fields for 'type'.
type Type struct {
	Got  string   // type of the value we got
	Want []string // types that are allowed
}

func (d Type) String() string {
	return fmt.Sprintf("expected %s, but got %s", strings.Join(d.Want, " or "), d.Got)
}

// Format captures error fields for 'format'.
type Format struct {
	Got  interface{} // the value we got
	Want string      // format that is allowed
}

func (d Format) String() string {
	var got = d.Got
	if v, ok := got.(string); ok {
		got = quote(v)
	}
	return fmt.Sprintf("%v is not valid %s", got, quote(d.Want))
}

// MinProperties captures error fields for 'minProperties'.
type MinProperties struct {
	Got  int // num properties we got
	Want int // min properties allowed
}

func (d MinProperties) String() string {
	return fmt.Sprintf("minimum %d properties allowed, but found %d properties", d.Want, d.Got)
}

// MaxProperties captures error fields for 'maxProperties'.
type MaxProperties struct {
	Got  int // num properties we got
	Want int // max properties allowed
}

func (d MaxProperties) String() string {
	return fmt.Sprintf("maximum %d properties allowed, but found %d properties", d.Want, d.Got)
}

// Required captures error fields for 'required'.
type Required struct {
	Want []string // properties that are missing
}

func (d Required) String() string {
	return fmt.Sprintf("missing properties: %s", strings.Join(d.Want, ", "))
}

// AdditionalProperties captures error fields for 'additionalProperties'.
type AdditionalProperties struct {
	Got []string // additional properties we got
}

func (d AdditionalProperties) String() string {
	pnames := make([]string, 0, len(d.Got))
	for _, pname := range d.Got {
		pnames = append(pnames, quote(pname))
	}
	return fmt.Sprintf("additionalProperties %s not allowed", strings.Join(pnames, ", "))
}

// DependentRequired captures error fields for 'dependentRequired', 'dependencies'.
type DependentRequired struct {
	Want string // property that is required
	Got  string // property that requires Want
}

func (d DependentRequired) String() string {
	return fmt.Sprintf("property %s is required, if %s property exists", quote(d.Want), quote(d.Got))
}

// MinItems captures error fields for 'minItems'.
type MinItems struct {
	Got  int // num items we got
	Want int // min items allowed
}

func (d MinItems) String() string {
	return fmt.Sprintf("minimum %d items required, but found %d items", d.Want, d.Got)
}

// MaxItems captures error fields for 'maxItems'.
type MaxItems struct {
	Got  int // num items we got
	Want int // max items allowed
}

func (d MaxItems) String() string {
	return fmt.Sprintf("maximum %d items required, but found %d items", d.Want, d.Got)
}

// MinContains captures error fields for 'minContains'.
type MinContains struct {
	Got  []int // item indexes matching contains schema
	Want int   // min items allowed matching contains schema
}

func (d MinContains) String() string {
	return fmt.Sprintf("minimum %d valid items required, but found %d valid items", d.Want, len(d.Got))
}

// MaxContains captures error fields for 'maxContains'.
type MaxContains struct {
	Got  []int // item indexes matching contains schema
	Want int   // max items allowed matching contains schema
}

func (d MaxContains) String() string {
	return fmt.Sprintf("maximum %d valid items required, but found %d valid items", d.Want, len(d.Got))
}

// UniqueItems captures error fields for 'uniqueItems'.
type UniqueItems struct {
	Got [2]int // item indexes that are not unique
}

func (d UniqueItems) String() string {
	return fmt.Sprintf("items at index %d and %d are equal", d.Got[0], d.Got[1])
}

// OneOf captures error fields for 'oneOf'.
type OneOf struct {
	Got []int // subschema indexes that matched
}

func (d OneOf) String() string {
	if len(d.Got) == 0 {
		return "oneOf failed"
	}
	return fmt.Sprintf("valid against subschemas %d and %d", d.Got[0], d.Got[1])
}

// AnyOf captures error fields for 'anyOf'.
type AnyOf struct{}

func (AnyOf) String() string {
	return "anyOf failed"
}

// AllOf captures error fields for 'allOf'.
type AllOf struct {
	Got []int // subschema indexes that did not match
}

func (d AllOf) String() string {
	got := fmt.Sprintf("%v", d.Got)
	got = got[1 : len(got)-1]
	return fmt.Sprintf("invalid against subschemas %v", got)
}

// Not captures error fields for 'not'.
type Not struct{}

func (Not) String() string {
	return "not failed"
}

// Schema captures error fields for top schema, '$ref', '$recursiveRef', '$dynamicRef'.
type Schema struct {
	Want string // url of schema that did not match
}

func (d Schema) String() string {
	return fmt.Sprintf("doesn't validate with %s", quote(d.Want))
}

// AdditionalItems captures error fields for 'additionalItems'.
type AdditionalItems struct {
	Got  int // num items we got
	Want int // num items allowed
}

func (d AdditionalItems) String() string {
	return fmt.Sprintf("only %d items are allowed, but found %d items", d.Want, d.Got)
}

// MinLength captures error fields for 'minLength'.
type MinLength struct {
	Got  int // length of string we got
	Want int // min length of string allowed
}

func (d MinLength) String() string {
	return fmt.Sprintf("length must be >= %d, but got %d", d.Want, d.Got)
}

// MaxLength captures error fields for 'maxLength'.
type MaxLength struct {
	Got  int // length of string we got
	Want int // max length of string allowed
}

func (d MaxLength) String() string {
	return fmt.Sprintf("length must be <= %d, but got %d", d.Want, d.Got)
}

// Pattern captures error fields for 'pattern'.
type Pattern struct {
	Got  string // string value we got
	Want string // regex that should match
}

func (d Pattern) String() string {
	return fmt.Sprintf("%s does not match pattern %s", quote(d.Got), quote(d.Want))
}

// Minimum captures error fields for 'minimum'.
type Minimum struct {
	Got  interface{} // number we got
	Want *big.Rat    // min number allowed
}

func (d Minimum) String() string {
	want, _ := d.Want.Float64()
	return fmt.Sprintf("must be >= %v but found %v", want, d.Got)
}

// Maximum captures error fields for 'maximum'.
type Maximum struct {
	Got  interface{} // number we got
	Want *big.Rat    // max number allowed
}

func (d Maximum) String() string {
	want, _ := d.Want.Float64()
	return fmt.Sprintf("must be <= %v but found %v", want, d.Got)
}

// ExclusiveMinimum captures error fields for 'exclusiveMinimum'.
type ExclusiveMinimum struct {
	Got  interface{} // number we got
	Want *big.Rat    // exclusive min number allowed
}

func (d ExclusiveMinimum) String() string {
	want, _ := d.Want.Float64()
	return fmt.Sprintf("must be > %v but found %v", want, d.Got)
}

// ExclusiveMaximum captures error fields for 'exclusiveMaximum'.
type ExclusiveMaximum struct {
	Got  interface{} // number we got
	Want *big.Rat    // exclusive max number allowed
}

func (d ExclusiveMaximum) String() string {
	want, _ := d.Want.Float64()
	return fmt.Sprintf("must be < %v but found %v", want, d.Got)
}

// MultipleOf captures error fields for 'multipleOf'.
type MultipleOf struct {
	Got  interface{} // number we got
	Want *big.Rat    // only multiple of this allowed
}

func (d MultipleOf) String() string {
	want, _ := d.Want.Float64()
	return fmt.Sprintf("%v not multipleOf %v", d.Got, want)
}

// Then captures error fields for 'then'.
type Then struct{}

func (Then) String() string {
	return "if-then failed"
}

// Else captures error fields for 'else'.
type Else struct{}

func (Else) String() string {
	return "if-else failed"
}

// Const captures error fields for 'const'.
type Const struct {
	Got  interface{} // value we got
	Want interface{} // value allowed
}

func (d Const) String() string {
	switch d.Want.(type) {
	case map[string]interface{}, []interface{}:
		return "const failed"
	default:
		return fmt.Sprintf("value must be %#v", d.Want)
	}
}

// Enum captures error fields for 'enum'.
type Enum struct {
	Got  interface{}   // value we got
	Want []interface{} // list of values allowed
}

func (d Enum) String() string {
	allPrimitives := true
	for _, item := range d.Want {
		switch item.(type) {
		case map[string]interface{}, []interface{}:
			allPrimitives = false
			break
		}
	}
	if allPrimitives {
		if len(d.Want) == 1 {
			return fmt.Sprintf("value must be %#v", d.Want[0])
		} else {
			strEnum := make([]string, len(d.Want))
			for i, item := range d.Want {
				strEnum[i] = fmt.Sprintf("%#v", item)
			}
			return fmt.Sprintf("value must be one of %s", strings.Join(strEnum, ", "))
		}
	}
	return "enum failed"
}

// ContentEncoding captures error fields for 'contentEncoding'.
type ContentEncoding struct {
	Got  string // value we got
	Want string // content encoding of the value allowed
}

func (d ContentEncoding) String() string {
	return fmt.Sprintf("value is not %s encoded", d.Want)
}

// ContentMediaType captures error fields for 'contentMediaType'.
type ContentMediaType struct {
	Got  []byte // decoded value we got
	Want string // media type of value allowed
}

func (d ContentMediaType) String() string {
	return fmt.Sprintf("value is not of mediatype %s", quote(d.Want))
}

// ContentSchema captures error fields for 'contentSchema'.
type ContentSchema struct {
	Got []byte // decoded value we got
}

func (ContentSchema) String() string {
	return "value is not valid json"
}

// quote returns single-quoted string
func quote(s string) string {
	s = fmt.Sprintf("%q", s)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s[1:len(s)-1] + "'"
}
