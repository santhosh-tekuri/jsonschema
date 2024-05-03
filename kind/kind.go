package kind

import (
	"fmt"
	"math/big"
	"strings"
)

// --

type InvalidJsonValue struct {
	Value any
}

func (*InvalidJsonValue) KeywordPath() []string {
	return nil
}

func (k *InvalidJsonValue) String() string {
	return fmt.Sprintf("invalid jsonType %T", k.Value)
}

// --

type Schema struct {
	Location string
}

func (*Schema) KeywordPath() []string {
	return nil
}

func (k *Schema) String() string {
	return fmt.Sprintf("jsonschema validation failed with %s", quote(k.Location))
}

// --

type Group struct{}

func (*Group) KeywordPath() []string {
	return nil
}

func (*Group) String() string {
	return "validation failed"
}

// --

type Not struct{}

func (*Not) KeywordPath() []string {
	return nil
}

func (*Not) String() string {
	return "not failed"
}

// --

type AllOf struct{}

func (*AllOf) KeywordPath() []string {
	return []string{"allOf"}
}

func (*AllOf) String() string {
	return "allOf failed"
}

// --

type AnyOf struct{}

func (*AnyOf) KeywordPath() []string {
	return []string{"anyOf"}
}

func (*AnyOf) String() string {
	return "anyOf failed"
}

// --

type OneOf struct {
	// Subschemas gives indexes of Subschemas that have matched.
	// Value nil, means none of the subschemas matched.
	Subschemas []int
}

func (*OneOf) KeywordPath() []string {
	return []string{"oneOf"}
}

func (k *OneOf) String() string {
	if len(k.Subschemas) == 0 {
		return "oneOf failed, none matched"
	}
	return fmt.Sprintf("oneOf failed, subschemas %d, %d matched", k.Subschemas[0], k.Subschemas[1])
}

//--

type FalseSchema struct{}

func (*FalseSchema) KeywordPath() []string {
	return nil
}

func (*FalseSchema) String() string {
	return "false schema"
}

// --

type RefCycle struct {
	URL              string
	KeywordLocation1 string
	KeywordLocation2 string
}

func (*RefCycle) KeywordPath() []string {
	return nil
}

func (k *RefCycle) String() string {
	return fmt.Sprintf("both %s and %s resolve to %q causing reference cycle", k.KeywordLocation1, k.KeywordLocation2, k.URL)
}

// --

type Type struct {
	Got  string
	Want []string
}

func (*Type) KeywordPath() []string {
	return []string{"type"}
}

func (k *Type) String() string {
	want := strings.Join(k.Want, " or ")
	return fmt.Sprintf("got %s, want %s", k.Got, want)
}

// --

type Enum struct {
	Got  any
	Want []any
}

// KeywordPath implements jsonschema.ErrorKind.
func (*Enum) KeywordPath() []string {
	return []string{"enum"}
}

func (k *Enum) String() string {
	allPrimitive := true
loop:
	for _, item := range k.Want {
		switch item.(type) {
		case []any, map[string]any:
			allPrimitive = false
			break loop
		}
	}
	if allPrimitive {
		if len(k.Want) == 1 {
			return fmt.Sprintf("value must be %s", display(k.Want[0]))
		}
		var want []string
		for _, v := range k.Want {
			want = append(want, display(v))
		}
		return fmt.Sprintf("value must be one of %s", strings.Join(want, ", "))
	}
	return "enum failed"
}

// --

type Const struct {
	Got  any
	Want any
}

func (*Const) KeywordPath() []string {
	return []string{"const"}
}

func (k *Const) String() string {
	switch want := k.Want.(type) {
	case []any, map[string]any:
		return "const failed"
	default:
		return fmt.Sprintf("value must be %s", display(want))
	}
}

// --

type Format struct {
	Got  any
	Want string
	Err  error
}

func (*Format) KeywordPath() []string {
	return []string{"format"}
}

func (k *Format) String() string {
	return fmt.Sprintf("%s is not valid %s: %v", display(k.Got), k.Want, k.Err)
}

// --

type Reference struct {
	Keyword string
	URL     string
}

func (k *Reference) KeywordPath() []string {
	return []string{k.Keyword}
}

func (*Reference) String() string {
	return "validation failed"
}

// --

type MinProperties struct {
	Got, Want int
}

func (*MinProperties) KeywordPath() []string {
	return []string{"minProperties"}
}

func (k *MinProperties) String() string {
	return fmt.Sprintf("minProperties: got %d, want %d", k.Got, k.Want)
}

// --

type MaxProperties struct {
	Got, Want int
}

func (*MaxProperties) KeywordPath() []string {
	return []string{"maxProperties"}
}

func (k *MaxProperties) String() string {
	return fmt.Sprintf("maxProperties: got %d, want %d", k.Got, k.Want)
}

// --

type MinItems struct {
	Got, Want int
}

func (*MinItems) KeywordPath() []string {
	return []string{"minItems"}
}

func (k *MinItems) String() string {
	return fmt.Sprintf("minItems: got %d, want %d", k.Got, k.Want)
}

// --

type MaxItems struct {
	Got, Want int
}

func (*MaxItems) KeywordPath() []string {
	return []string{"maxItems"}
}

func (k *MaxItems) String() string {
	return fmt.Sprintf("maxItems: got %d, want %d", k.Got, k.Want)
}

// --

type AdditionalItems struct {
	Count int
}

func (*AdditionalItems) KeywordPath() []string {
	return []string{"additionalItems"}
}

func (k *AdditionalItems) String() string {
	return fmt.Sprintf("last %d additionalItem(s) not allowed", k.Count)
}

// --

type Required struct {
	Missing []string
}

func (*Required) KeywordPath() []string {
	return []string{"required"}
}

func (k *Required) String() string {
	if len(k.Missing) == 1 {
		return fmt.Sprintf("missing property %s", quote(k.Missing[0]))
	}
	return fmt.Sprintf("missing properties %s", joinQuoted(k.Missing, ", "))
}

// --

type Dependency struct {
	Prop    string   // dependency of prop that failed
	Missing []string // missing props
}

func (k *Dependency) KeywordPath() []string {
	return []string{"dependency", k.Prop}
}

func (k *Dependency) String() string {
	return fmt.Sprintf("properties %s required, if %s exists", joinQuoted(k.Missing, ", "), quote(k.Prop))
}

// --

type DependentRequired struct {
	Prop    string   // dependency of prop that failed
	Missing []string // missing props
}

func (k *DependentRequired) KeywordPath() []string {
	return []string{"dependentRequired", k.Prop}
}

func (k *DependentRequired) String() string {
	return fmt.Sprintf("properties %s required, if %s exists", joinQuoted(k.Missing, ", "), quote(k.Prop))
}

// --

type AdditionalProperties struct {
	Properties []string
}

func (*AdditionalProperties) KeywordPath() []string {
	return []string{"additionalProperties"}
}

func (k *AdditionalProperties) String() string {
	return fmt.Sprintf("additional properties %s not allowed", joinQuoted(k.Properties, ", "))
}

// --

type PropertyNames struct {
	Property string
}

func (*PropertyNames) KeywordPath() []string {
	return []string{"propertyNames"}
}

func (k *PropertyNames) String() string {
	return fmt.Sprintf("invalid property %s", quote(k.Property))
}

// --

type UniqueItems struct {
	Duplicates [2]int
}

func (*UniqueItems) KeywordPath() []string {
	return []string{"uniqueItems"}
}

func (k *UniqueItems) String() string {
	return fmt.Sprintf("items at %d and %d are equal", k.Duplicates[0], k.Duplicates[1])
}

// --

type Contains struct{}

func (*Contains) KeywordPath() []string {
	return []string{"contains"}
}

func (*Contains) String() string {
	return "no items match contains schema"
}

// --

type MinContains struct {
	Got  []int
	Want int
}

func (*MinContains) KeywordPath() []string {
	return []string{"minContains"}
}

func (k *MinContains) String() string {
	if len(k.Got) == 0 {
		return fmt.Sprintf("min %d items required to match contains schema, but none matched", k.Want)
	} else {
		got := fmt.Sprintf("%v", k.Got)
		return fmt.Sprintf("min %d items required to match contains schema, but matched %d items at %v", k.Want, len(k.Got), got[1:len(got)-1])
	}
}

// --

type MaxContains struct {
	Got  []int
	Want int
}

func (*MaxContains) KeywordPath() []string {
	return []string{"maxContains"}
}

func (k *MaxContains) String() string {
	got := fmt.Sprintf("%v", k.Got)
	return fmt.Sprintf("max %d items required to match contains schema, but matched %d items at %v", k.Want, len(k.Got), got[1:len(got)-1])
}

// --

type MinLength struct {
	Got, Want int
}

func (*MinLength) KeywordPath() []string {
	return []string{"minLength"}
}

func (k *MinLength) String() string {
	return fmt.Sprintf("minLength: got %d, want %d", k.Got, k.Want)
}

// --

type MaxLength struct {
	Got, Want int
}

func (*MaxLength) KeywordPath() []string {
	return []string{"maxLength"}
}

func (k *MaxLength) String() string {
	return fmt.Sprintf("maxLength: got %d, want %d", k.Got, k.Want)
}

// --

type Pattern struct {
	Got  string
	Want string
}

func (*Pattern) KeywordPath() []string {
	return []string{"pattern"}
}

func (k *Pattern) String() string {
	return fmt.Sprintf("%s does not match pattern %s", quote(k.Got), quote(k.Want))
}

// --

type ContentEncoding struct {
	Want string
	Err  error
}

func (*ContentEncoding) KeywordPath() []string {
	return []string{"contentEncoding"}
}

func (k *ContentEncoding) String() string {
	return fmt.Sprintf("value is not %s encoded: %v", quote(k.Want), k.Err)
}

// --

type ContentMediaType struct {
	Got  []byte
	Want string
	Err  error
}

func (*ContentMediaType) KeywordPath() []string {
	return []string{"contentMediaType"}
}

func (k *ContentMediaType) String() string {
	return fmt.Sprintf("value if not of mediatype %s: %v", quote(k.Want), k.Err)
}

// --

type ContentSchema struct{}

func (ContentSchema) KeywordPath() []string {
	return []string{"contentSchema"}
}

func (ContentSchema) String() string {
	return "contentSchema failed"
}

// --

type Minimum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*Minimum) KeywordPath() []string {
	return []string{"minimum"}
}

func (k *Minimum) String() string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return fmt.Sprintf("minimum: got %v, want %v", got, want)
}

// --

type Maximum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*Maximum) KeywordPath() []string {
	return []string{"maximum"}
}

func (k *Maximum) String() string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return fmt.Sprintf("maximum: got %v, want %v", got, want)
}

// --

type ExclusiveMinimum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*ExclusiveMinimum) KeywordPath() []string {
	return []string{"exclusiveMinimum"}
}

func (k *ExclusiveMinimum) String() string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return fmt.Sprintf("exclusiveMinimum: got %v, want %v", got, want)
}

// --

type ExclusiveMaximum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*ExclusiveMaximum) KeywordPath() []string {
	return []string{"exclusiveMaximum"}
}

func (k *ExclusiveMaximum) String() string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return fmt.Sprintf("exclusiveMaximum: got %v, want %v", got, want)
}

// --

type MultipleOf struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*MultipleOf) KeywordPath() []string {
	return []string{"multipleOf"}
}

func (k *MultipleOf) String() string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return fmt.Sprintf("multipleOf: got %v, want %v", got, want)
}

// --

func quote(s string) string {
	s = fmt.Sprintf("%q", s)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s[1:len(s)-1] + "'"
}

func joinQuoted(arr []string, sep string) string {
	var sb strings.Builder
	for _, s := range arr {
		if sb.Len() > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(quote(s))
	}
	return sb.String()
}

// to be used only for primitive.
func display(v any) string {
	switch v := v.(type) {
	case string:
		return quote(v)
	case []any, map[string]any:
		return "value"
	default:
		return fmt.Sprintf("%v", v)
	}
}
