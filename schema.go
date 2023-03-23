package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v5/msg"
)

// A Schema represents compiled version of json-schema.
type Schema struct {
	Location string // absolute location

	Draft          *Draft // draft used by schema.
	meta           *Schema
	vocab          []string
	dynamicAnchors []*Schema

	// type agnostic validations
	Format          string
	format          func(interface{}) bool
	Always          *bool // always pass/fail. used when booleans are used as schemas in draft-07.
	Ref             *Schema
	RecursiveAnchor bool
	RecursiveRef    *Schema
	DynamicAnchor   string
	DynamicRef      *Schema
	Types           []string      // allowed types.
	Constant        []interface{} // first element in slice is constant value. note: slice is used to capture nil constant.
	Enum            []interface{} // allowed values.
	Not             *Schema
	AllOf           []*Schema
	AnyOf           []*Schema
	OneOf           []*Schema
	If              *Schema
	Then            *Schema // nil, when If is nil.
	Else            *Schema // nil, when If is nil.

	// object validations
	MinProperties         int      // -1 if not specified.
	MaxProperties         int      // -1 if not specified.
	Required              []string // list of required properties.
	Properties            map[string]*Schema
	PropertyNames         *Schema
	RegexProperties       bool // property names must be valid regex. used only in draft4 as workaround in metaschema.
	PatternProperties     map[*regexp.Regexp]*Schema
	AdditionalProperties  interface{}            // nil or bool or *Schema.
	Dependencies          map[string]interface{} // map value is *Schema or []string.
	DependentRequired     map[string][]string
	DependentSchemas      map[string]*Schema
	UnevaluatedProperties *Schema

	// array validations
	MinItems         int // -1 if not specified.
	MaxItems         int // -1 if not specified.
	UniqueItems      bool
	Items            interface{} // nil or *Schema or []*Schema
	AdditionalItems  interface{} // nil or bool or *Schema.
	PrefixItems      []*Schema
	Items2020        *Schema // items keyword reintroduced in draft 2020-12
	Contains         *Schema
	ContainsEval     bool // whether any item in an array that passes validation of the contains schema is considered "evaluated"
	MinContains      int  // 1 if not specified
	MaxContains      int  // -1 if not specified
	UnevaluatedItems *Schema

	// string validations
	MinLength        int // -1 if not specified.
	MaxLength        int // -1 if not specified.
	Pattern          *regexp.Regexp
	ContentEncoding  string
	decoder          func(string) ([]byte, error)
	ContentMediaType string
	mediaType        func([]byte) error
	ContentSchema    *Schema

	// number validators
	Minimum          *big.Rat
	ExclusiveMinimum *big.Rat
	Maximum          *big.Rat
	ExclusiveMaximum *big.Rat
	MultipleOf       *big.Rat

	// annotations. captured only when Compiler.ExtractAnnotations is true.
	Title       string
	Description string
	Default     interface{}
	Comment     string
	ReadOnly    bool
	WriteOnly   bool
	Examples    []interface{}
	Deprecated  bool

	// user defined extensions
	Extensions map[string]ExtSchema
}

func (s *Schema) String() string {
	return s.Location
}

func newSchema(url, floc string, draft *Draft, doc interface{}) *Schema {
	// fill with default values
	s := &Schema{
		Location:      url + floc,
		Draft:         draft,
		MinProperties: -1,
		MaxProperties: -1,
		MinItems:      -1,
		MaxItems:      -1,
		MinContains:   1,
		MaxContains:   -1,
		MinLength:     -1,
		MaxLength:     -1,
	}

	if doc, ok := doc.(map[string]interface{}); ok {
		if ra, ok := doc["$recursiveAnchor"]; ok {
			if ra, ok := ra.(bool); ok {
				s.RecursiveAnchor = ra
			}
		}
		if da, ok := doc["$dynamicAnchor"]; ok {
			if da, ok := da.(string); ok {
				s.DynamicAnchor = da
			}
		}
	}
	return s
}

func (s *Schema) hasVocab(name string) bool {
	if s == nil { // during bootstrap
		return true
	}
	if name == "core" {
		return true
	}
	for _, url := range s.vocab {
		if url == "https://json-schema.org/draft/2019-09/vocab/"+name {
			return true
		}
		if url == "https://json-schema.org/draft/2020-12/vocab/"+name {
			return true
		}
	}
	return false
}

// Validate validates given doc, against the json-schema s.
//
// the v must be the raw json value. for number precision
// unmarshal with json.UseNumber().
//
// returns *ValidationError if v does not confirm with schema s.
// returns InfiniteLoopError if it detects loop during validation.
// returns InvalidJSONTypeError if it detects any non json value in v.
func (s *Schema) Validate(v interface{}) (err error) {
	return s.validateValue(v, "")
}

func (s *Schema) validateValue(v interface{}, vloc string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case InfiniteLoopError, InvalidJSONTypeError:
				err = r.(error)
			default:
				panic(r)
			}
		}
	}()
	if _, err := s.validate(nil, 0, "", v, vloc); err != nil {
		ve := ValidationError{
			KeywordLocation:         "",
			AbsoluteKeywordLocation: s.Location,
			InstanceLocation:        vloc,
			Message:                 msg.Schema{Want: s.Location},
		}
		return ve.causes(err)
	}
	return nil
}

// validate validates given value v with this schema.
func (s *Schema) validate(scope []schemaRef, vscope int, spath string, v interface{}, vloc string) (result validationResult, err error) {
	validationError := func(keywordPath string, msg fmt.Stringer) *ValidationError {
		return &ValidationError{
			KeywordLocation:         keywordLocation(scope, keywordPath),
			AbsoluteKeywordLocation: joinPtr(s.Location, keywordPath),
			InstanceLocation:        vloc,
			Message:                 msg,
		}
	}

	sref := schemaRef{spath, s, false}
	if err := checkLoop(scope[len(scope)-vscope:], sref); err != nil {
		panic(err)
	}
	scope = append(scope, sref)
	vscope++

	// populate result
	switch v := v.(type) {
	case map[string]interface{}:
		result.unevalProps = make(map[string]struct{})
		for pname := range v {
			result.unevalProps[pname] = struct{}{}
		}
	case []interface{}:
		result.unevalItems = make(map[int]struct{})
		for i := range v {
			result.unevalItems[i] = struct{}{}
		}
	}

	validate := func(sch *Schema, schPath string, v interface{}, vpath string) error {
		vloc := vloc
		if vpath != "" {
			vloc += "/" + vpath
		}
		_, err := sch.validate(scope, 0, schPath, v, vloc)
		return err
	}

	validateInplace := func(sch *Schema, schPath string) error {
		vr, err := sch.validate(scope, vscope, schPath, v, vloc)
		if err == nil {
			// update result
			for pname := range result.unevalProps {
				if _, ok := vr.unevalProps[pname]; !ok {
					delete(result.unevalProps, pname)
				}
			}
			for i := range result.unevalItems {
				if _, ok := vr.unevalItems[i]; !ok {
					delete(result.unevalItems, i)
				}
			}
		}
		return err
	}

	if s.Always != nil {
		if !*s.Always {
			return result, validationError("", msg.False{})
		}
		return result, nil
	}

	if len(s.Types) > 0 {
		vType := jsonType(v)
		matched := false
		for _, t := range s.Types {
			if vType == t {
				matched = true
				break
			} else if t == "integer" && vType == "number" {
				num, _ := new(big.Rat).SetString(fmt.Sprint(v))
				if num.IsInt() {
					matched = true
					break
				}
			}
		}
		if !matched {
			return result, validationError("type", msg.Type{Got: vType, Want: s.Types})
		}
	}

	var errors []error

	if len(s.Constant) > 0 {
		if !equals(v, s.Constant[0]) {
			errors = append(errors, validationError("const", msg.Const{Got: v, Want: s.Constant[0]}))
		}
	}

	if len(s.Enum) > 0 {
		matched := false
		for _, item := range s.Enum {
			if equals(v, item) {
				matched = true
				break
			}
		}
		if !matched {
			errors = append(errors, validationError("enum", msg.Enum{Got: v, Want: s.Enum}))
		}
	}

	if s.format != nil && !s.format(v) {
		errors = append(errors, validationError("format", msg.Format{Got: v, Want: s.Format}))
	}

	switch v := v.(type) {
	case map[string]interface{}:
		if s.MinProperties != -1 && len(v) < s.MinProperties {
			errors = append(errors, validationError("minProperties", msg.MinProperties{Got: len(v), Want: s.MinProperties}))
		}
		if s.MaxProperties != -1 && len(v) > s.MaxProperties {
			errors = append(errors, validationError("maxProperties", msg.MaxProperties{Got: len(v), Want: s.MaxProperties}))
		}
		if len(s.Required) > 0 {
			var missing []string
			for _, pname := range s.Required {
				if _, ok := v[pname]; !ok {
					missing = append(missing, pname)
				}
			}
			if len(missing) > 0 {
				errors = append(errors, validationError("required", msg.Required{Want: missing}))
			}
		}

		for pname, sch := range s.Properties {
			if pvalue, ok := v[pname]; ok {
				delete(result.unevalProps, pname)
				if err := validate(sch, "properties/"+escape(pname), pvalue, escape(pname)); err != nil {
					errors = append(errors, err)
				}
			}
		}

		if s.PropertyNames != nil {
			for pname := range v {
				if err := validate(s.PropertyNames, "propertyNames", pname, escape(pname)); err != nil {
					errors = append(errors, err)
				}
			}
		}

		if s.RegexProperties {
			for pname := range v {
				if !isRegex(pname) {
					errors = append(errors, validationError("", msg.Format{Got: pname, Want: "regex"}))
				}
			}
		}
		for pattern, sch := range s.PatternProperties {
			for pname, pvalue := range v {
				if pattern.MatchString(pname) {
					delete(result.unevalProps, pname)
					if err := validate(sch, "patternProperties/"+escape(pattern.String()), pvalue, escape(pname)); err != nil {
						errors = append(errors, err)
					}
				}
			}
		}
		if s.AdditionalProperties != nil {
			if allowed, ok := s.AdditionalProperties.(bool); ok {
				if !allowed && len(result.unevalProps) > 0 {
					errors = append(errors, validationError("additionalProperties", msg.AdditionalProperties{Got: result.unevalPnames()}))
				}
			} else {
				schema := s.AdditionalProperties.(*Schema)
				for pname := range result.unevalProps {
					if pvalue, ok := v[pname]; ok {
						if err := validate(schema, "additionalProperties", pvalue, escape(pname)); err != nil {
							errors = append(errors, err)
						}
					}
				}
			}
			result.unevalProps = nil
		}
		for dname, dvalue := range s.Dependencies {
			if _, ok := v[dname]; ok {
				switch dvalue := dvalue.(type) {
				case *Schema:
					if err := validateInplace(dvalue, "dependencies/"+escape(dname)); err != nil {
						errors = append(errors, err)
					}
				case []string:
					for i, pname := range dvalue {
						if _, ok := v[pname]; !ok {
							errors = append(errors, validationError("dependencies/"+escape(dname)+"/"+strconv.Itoa(i), msg.DependentRequired{Got: dname, Want: pname}))
						}
					}
				}
			}
		}
		for dname, dvalue := range s.DependentRequired {
			if _, ok := v[dname]; ok {
				for i, pname := range dvalue {
					if _, ok := v[pname]; !ok {
						errors = append(errors, validationError("dependentRequired/"+escape(dname)+"/"+strconv.Itoa(i), msg.DependentRequired{Got: dname, Want: pname}))
					}
				}
			}
		}
		for dname, sch := range s.DependentSchemas {
			if _, ok := v[dname]; ok {
				if err := validateInplace(sch, "dependentSchemas/"+escape(dname)); err != nil {
					errors = append(errors, err)
				}
			}
		}

	case []interface{}:
		if s.MinItems != -1 && len(v) < s.MinItems {
			errors = append(errors, validationError("minItems", msg.MinItems{Got: len(v), Want: s.MinItems}))
		}
		if s.MaxItems != -1 && len(v) > s.MaxItems {
			errors = append(errors, validationError("maxItems", msg.MaxItems{Got: len(v), Want: s.MaxItems}))
		}
		if s.UniqueItems {
			for i := 1; i < len(v); i++ {
				for j := 0; j < i; j++ {
					if equals(v[i], v[j]) {
						errors = append(errors, validationError("uniqueItems", msg.UniqueItems{Got: [2]int{j, i}}))
					}
				}
			}
		}

		// items + additionalItems
		switch items := s.Items.(type) {
		case *Schema:
			for i, item := range v {
				if err := validate(items, "items", item, strconv.Itoa(i)); err != nil {
					errors = append(errors, err)
				}
			}
			result.unevalItems = nil
		case []*Schema:
			for i, item := range v {
				if i < len(items) {
					delete(result.unevalItems, i)
					if err := validate(items[i], "items/"+strconv.Itoa(i), item, strconv.Itoa(i)); err != nil {
						errors = append(errors, err)
					}
				} else if sch, ok := s.AdditionalItems.(*Schema); ok {
					delete(result.unevalItems, i)
					if err := validate(sch, "additionalItems", item, strconv.Itoa(i)); err != nil {
						errors = append(errors, err)
					}
				} else {
					break
				}
			}
			if additionalItems, ok := s.AdditionalItems.(bool); ok {
				if additionalItems {
					result.unevalItems = nil
				} else if len(v) > len(items) {
					errors = append(errors, validationError("additionalItems", msg.AdditionalItems{Got: len(v), Want: len(items)}))
				}
			}
		}

		// prefixItems + items
		for i, item := range v {
			if i < len(s.PrefixItems) {
				delete(result.unevalItems, i)
				if err := validate(s.PrefixItems[i], "prefixItems/"+strconv.Itoa(i), item, strconv.Itoa(i)); err != nil {
					errors = append(errors, err)
				}
			} else if s.Items2020 != nil {
				delete(result.unevalItems, i)
				if err := validate(s.Items2020, "items", item, strconv.Itoa(i)); err != nil {
					errors = append(errors, err)
				}
			} else {
				break
			}
		}

		// contains + minContains + maxContains
		if s.Contains != nil && (s.MinContains != -1 || s.MaxContains != -1) {
			var matched []int
			var causes []error
			for i, item := range v {
				if err := validate(s.Contains, "contains", item, strconv.Itoa(i)); err != nil {
					causes = append(causes, err)
				} else {
					matched = append(matched, i)
					if s.ContainsEval {
						delete(result.unevalItems, i)
					}
				}
			}
			if s.MinContains != -1 && len(matched) < s.MinContains {
				errors = append(errors, validationError("minContains", msg.MinContains{Got: matched, Want: s.MinContains}).add(causes...))
			}
			if s.MaxContains != -1 && len(matched) > s.MaxContains {
				errors = append(errors, validationError("maxContains", msg.MaxContains{Got: matched, Want: s.MaxContains}))
			}
		}

	case string:
		// minLength + maxLength
		if s.MinLength != -1 || s.MaxLength != -1 {
			length := utf8.RuneCount([]byte(v))
			if s.MinLength != -1 && length < s.MinLength {
				errors = append(errors, validationError("minLength", msg.MinLength{Got: length, Want: s.MinLength}))
			}
			if s.MaxLength != -1 && length > s.MaxLength {
				errors = append(errors, validationError("maxLength", msg.MaxLength{Got: length, Want: s.MaxLength}))
			}
		}

		if s.Pattern != nil && !s.Pattern.MatchString(v) {
			errors = append(errors, validationError("pattern", msg.Pattern{Got: v, Want: s.Pattern.String()}))
		}

		// contentEncoding + contentMediaType
		if s.decoder != nil || s.mediaType != nil {
			decoded := s.ContentEncoding == ""
			var content []byte
			if s.decoder != nil {
				b, err := s.decoder(v)
				if err != nil {
					errors = append(errors, validationError("contentEncoding", msg.ContentEncoding{Got: v, Want: s.ContentEncoding}))
				} else {
					content, decoded = b, true
				}
			}
			if decoded && s.mediaType != nil {
				if s.decoder == nil {
					content = []byte(v)
				}
				if err := s.mediaType(content); err != nil {
					errors = append(errors, validationError("contentMediaType", msg.ContentMediaType{Got: content, Want: s.ContentMediaType}))
				}
			}
			if decoded && s.ContentSchema != nil {
				contentJSON, err := unmarshal(bytes.NewReader(content))
				if err != nil {
					errors = append(errors, validationError("contentSchema", msg.ContentSchema{Got: content}))
				} else {
					err := validate(s.ContentSchema, "contentSchema", contentJSON, "")
					if err != nil {
						errors = append(errors, err)
					}
				}
			}
		}

	case json.Number, float32, float64, int, int8, int32, int64, uint, uint8, uint32, uint64:
		// lazy convert to *big.Rat to avoid allocation
		var numVal *big.Rat
		num := func() *big.Rat {
			if numVal == nil {
				numVal, _ = new(big.Rat).SetString(fmt.Sprint(v))
			}
			return numVal
		}
		if s.Minimum != nil && num().Cmp(s.Minimum) < 0 {
			errors = append(errors, validationError("minimum", msg.Minimum{Got: v, Want: s.Minimum}))
		}
		if s.ExclusiveMinimum != nil && num().Cmp(s.ExclusiveMinimum) <= 0 {
			errors = append(errors, validationError("exclusiveMinimum", msg.ExclusiveMinimum{Got: v, Want: s.ExclusiveMinimum}))
		}
		if s.Maximum != nil && num().Cmp(s.Maximum) > 0 {
			errors = append(errors, validationError("maximum", msg.Maximum{Got: v, Want: s.Maximum}))
		}
		if s.ExclusiveMaximum != nil && num().Cmp(s.ExclusiveMaximum) >= 0 {
			errors = append(errors, validationError("exclusiveMaximum", msg.ExclusiveMaximum{Got: v, Want: s.ExclusiveMaximum}))
		}
		if s.MultipleOf != nil {
			if q := new(big.Rat).Quo(num(), s.MultipleOf); !q.IsInt() {
				errors = append(errors, validationError("multipleOf", msg.MultipleOf{Got: v, Want: s.MultipleOf}))
			}
		}
	}

	// $ref + $recursiveRef + $dynamicRef
	validateRef := func(sch *Schema, refPath string) error {
		if sch != nil {
			if err := validateInplace(sch, refPath); err != nil {
				var url = sch.Location
				if s.url() == sch.url() {
					url = sch.loc()
				}
				return validationError(refPath, msg.Schema{Want: url}).causes(err)
			}
		}
		return nil
	}
	if err := validateRef(s.Ref, "$ref"); err != nil {
		errors = append(errors, err)
	}
	if s.RecursiveRef != nil {
		sch := s.RecursiveRef
		if sch.RecursiveAnchor {
			// recursiveRef based on scope
			for _, e := range scope {
				if e.schema.RecursiveAnchor {
					sch = e.schema
					break
				}
			}
		}
		if err := validateRef(sch, "$recursiveRef"); err != nil {
			errors = append(errors, err)
		}
	}
	if s.DynamicRef != nil {
		sch := s.DynamicRef
		if sch.DynamicAnchor != "" {
			// dynamicRef based on scope
			for i := len(scope) - 1; i >= 0; i-- {
				sr := scope[i]
				if sr.discard {
					break
				}
				for _, da := range sr.schema.dynamicAnchors {
					if da.DynamicAnchor == s.DynamicRef.DynamicAnchor && da != s.DynamicRef {
						sch = da
						break
					}
				}
			}
		}
		if err := validateRef(sch, "$dynamicRef"); err != nil {
			errors = append(errors, err)
		}
	}

	if s.Not != nil && validateInplace(s.Not, "not") == nil {
		errors = append(errors, validationError("not", msg.Not{}))
	}

	if len(s.AllOf) > 0 {
		var failed []int
		var causes []error
		for i, sch := range s.AllOf {
			if err := validateInplace(sch, "allOf/"+strconv.Itoa(i)); err != nil {
				failed = append(failed, i)
				causes = append(causes, err)
			}
		}
		if len(failed) > 0 {
			errors = append(errors, validationError("allOf", msg.AllOf{Got: failed}).add(causes...))
		}
	}

	if len(s.AnyOf) > 0 {
		matched := false
		var causes []error
		for i, sch := range s.AnyOf {
			if err := validateInplace(sch, "anyOf/"+strconv.Itoa(i)); err == nil {
				matched = true
			} else {
				causes = append(causes, err)
			}
		}
		if !matched {
			errors = append(errors, validationError("anyOf", msg.AnyOf{}).add(causes...))
		}
	}

	if len(s.OneOf) > 0 {
		matched := -1
		var causes []error
		for i, sch := range s.OneOf {
			if err := validateInplace(sch, "oneOf/"+strconv.Itoa(i)); err == nil {
				if matched == -1 {
					matched = i
				} else {
					errors = append(errors, validationError("oneOf", msg.OneOf{Got: []int{matched, i}}))
					break
				}
			} else {
				causes = append(causes, err)
			}
		}
		if matched == -1 {
			errors = append(errors, validationError("oneOf", msg.OneOf{}).add(causes...))
		}
	}

	// if + then + else
	if s.If != nil {
		err := validateInplace(s.If, "if")
		// "if" leaves dynamic scope
		scope[len(scope)-1].discard = true
		if err == nil {
			if s.Then != nil {
				if err := validateInplace(s.Then, "then"); err != nil {
					errors = append(errors, validationError("then", msg.Then{}).add(err))
				}
			}
		} else {
			if s.Else != nil {
				if err := validateInplace(s.Else, "else"); err != nil {
					errors = append(errors, validationError("else", msg.Else{}).add(err))
				}
			}
		}
		// restore dynamic scope
		scope[len(scope)-1].discard = false
	}

	for _, ext := range s.Extensions {
		if err := ext.Validate(ValidationContext{result, validate, validateInplace, validationError}, v); err != nil {
			errors = append(errors, err)
		}
	}

	// unevaluatedProperties + unevaluatedItems
	switch v := v.(type) {
	case map[string]interface{}:
		if s.UnevaluatedProperties != nil {
			for pname := range result.unevalProps {
				if pvalue, ok := v[pname]; ok {
					if err := validate(s.UnevaluatedProperties, "unevaluatedProperties", pvalue, escape(pname)); err != nil {
						errors = append(errors, err)
					}
				}
			}
			result.unevalProps = nil
		}
	case []interface{}:
		if s.UnevaluatedItems != nil {
			for i := range result.unevalItems {
				if err := validate(s.UnevaluatedItems, "unevaluatedItems", v[i], strconv.Itoa(i)); err != nil {
					errors = append(errors, err)
				}
			}
			result.unevalItems = nil
		}
	}

	switch len(errors) {
	case 0:
		return result, nil
	case 1:
		return result, errors[0]
	default:
		return result, validationError("", msg.Empty{}).add(errors...) // empty message, used just for wrapping
	}
}

type validationResult struct {
	unevalProps map[string]struct{}
	unevalItems map[int]struct{}
}

func (vr validationResult) unevalPnames() []string {
	pnames := make([]string, 0, len(vr.unevalProps))
	for pname := range vr.unevalProps {
		pnames = append(pnames, pname)
	}
	return pnames
}

// jsonType returns the json type of given value v.
//
// It panics if the given value is not valid json value
func jsonType(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number, float32, float64, int, int8, int32, int64, uint, uint8, uint32, uint64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	}
	panic(InvalidJSONTypeError(fmt.Sprintf("%T", v)))
}

// equals tells if given two json values are equal or not.
func equals(v1, v2 interface{}) bool {
	v1Type := jsonType(v1)
	if v1Type != jsonType(v2) {
		return false
	}
	switch v1Type {
	case "array":
		arr1, arr2 := v1.([]interface{}), v2.([]interface{})
		if len(arr1) != len(arr2) {
			return false
		}
		for i := range arr1 {
			if !equals(arr1[i], arr2[i]) {
				return false
			}
		}
		return true
	case "object":
		obj1, obj2 := v1.(map[string]interface{}), v2.(map[string]interface{})
		if len(obj1) != len(obj2) {
			return false
		}
		for k, v1 := range obj1 {
			if v2, ok := obj2[k]; ok {
				if !equals(v1, v2) {
					return false
				}
			} else {
				return false
			}
		}
		return true
	case "number":
		num1, _ := new(big.Rat).SetString(fmt.Sprint(v1))
		num2, _ := new(big.Rat).SetString(fmt.Sprint(v2))
		return num1.Cmp(num2) == 0
	default:
		return v1 == v2
	}
}

// escape converts given token to valid json-pointer token
func escape(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	token = strings.ReplaceAll(token, "/", "~1")
	return url.PathEscape(token)
}
