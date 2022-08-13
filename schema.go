package jsonschema

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A Schema represents compiled version of json-schema.
type Schema struct {
	Location string // absolute location

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
	enumError       string        // error message for enum fail. captured here to avoid constructing error message every time.
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

func newSchema(url, floc string, doc interface{}) *Schema {
	// fill with default values
	s := &Schema{
		Location:      url + floc,
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
	if _, err := s.validate(nil, 0, "", v, vloc); err != nil {
		if !IsValidationError(err) {
			return err
		}
		ve := ValidationError{
			KeywordLocation:         "",
			AbsoluteKeywordLocation: s.Location,
			InstanceLocation:        vloc,
			Message:                 fmt.Sprintf("doesn't validate with %s", s.Location),
		}
		return ve.causes(err)
	}
	return nil
}

// validate validates given value v with this schema.
func (s *Schema) validate(scope []schemaRef, vscope int, spath string, v interface{}, vloc string) (result validationResult, err error) {
	validationError := func(keywordPath string, format string, a ...interface{}) *ValidationError {
		return &ValidationError{
			KeywordLocation:         keywordLocation(scope, keywordPath),
			AbsoluteKeywordLocation: joinPtr(s.Location, keywordPath),
			InstanceLocation:        vloc,
			Message:                 fmt.Sprintf(format, a...),
		}
	}

	sref := schemaRef{spath, s, false}
	if err = checkLoop(scope[len(scope)-vscope:], sref); err != nil {
		return result, err
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
		if err != nil && !IsValidationError(err) {
			return err
		}
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
			return result, validationError("", "not allowed")
		}
		return result, nil
	}
	var vType string
	if len(s.Types) > 0 {
		vType, err = jsonType(v)
		if err != nil {
			return result, err
		}
		matched := false
		for _, t := range s.Types {
			if vType == t {
				matched = true
				break
			} else if t == "integer" && vType == "number" {
				num, ok := new(big.Rat).SetString(fmt.Sprint(v))
				if !ok {
					return result, invalidJSONTypeError(v)
				}
				if num.IsInt() {
					matched = true
					break
				}
			}
		}
		if !matched {
			return result, validationError("type", "expected %s, but got %s", strings.Join(s.Types, " or "), vType)
		}
	}

	var errs []error

	if len(s.Constant) > 0 {
		eq, err := equals(v, s.Constant[0])
		if err != nil {
			return result, err
		}
		typ, err := jsonType(s.Constant[0])
		if err != nil {
			return result, err
		}
		if !eq {
			switch typ {
			case "object", "array":
				errs = append(errs, validationError("const", "const failed"))
			default:
				errs = append(errs, validationError("const", "value must be %#v", s.Constant[0]))
			}
		}
	}

	if len(s.Enum) > 0 {
		matched := false
		var err error
		var eq bool
		for _, item := range s.Enum {
			eq, err = equals(v, item)
			if err != nil {
				return result, err
			}
			if eq {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, validationError("enum", s.enumError))
		}
	}

	if s.format != nil && !s.format(v) {
		val := v
		if v, ok := v.(string); ok {
			val = quote(v)
		}
		errs = append(errs, validationError("format", "%v is not valid %s", val, quote(s.Format)))
	}

	switch v := v.(type) {
	case map[string]interface{}:
		if s.MinProperties != -1 && len(v) < s.MinProperties {
			errs = append(errs, validationError("minProperties", "minimum %d properties allowed, but found %d properties", s.MinProperties, len(v)))
		}
		if s.MaxProperties != -1 && len(v) > s.MaxProperties {
			errs = append(errs, validationError("maxProperties", "maximum %d properties allowed, but found %d properties", s.MaxProperties, len(v)))
		}
		if len(s.Required) > 0 {
			var missing []string
			for _, pname := range s.Required {
				if _, ok := v[pname]; !ok {
					missing = append(missing, quote(pname))
				}
			}
			if len(missing) > 0 {
				errs = append(errs, validationError("required", "missing properties: %s", strings.Join(missing, ", ")))
			}
		}

		for pname, sch := range s.Properties {
			if pvalue, ok := v[pname]; ok {
				delete(result.unevalProps, pname)
				if err = validate(sch, "properties/"+escape(pname), pvalue, escape(pname)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			}
		}

		if s.PropertyNames != nil {
			for pname := range v {
				if err = validate(s.PropertyNames, "propertyNames", pname, escape(pname)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			}
		}

		if s.RegexProperties {
			for pname := range v {
				if !isRegex(pname) {
					errs = append(errs, validationError("", "patternProperty %s is not valid regex", quote(pname)))
				}
			}
		}

		for pattern, sch := range s.PatternProperties {
			for pname, pvalue := range v {
				if pattern.MatchString(pname) {
					delete(result.unevalProps, pname)
					if err = validate(sch, "patternProperties/"+escape(pattern.String()), pvalue, escape(pname)); err != nil {
						if IsValidationError(err) {
							errs = append(errs, err)
						} else {
							return result, err
						}
					}
				}
			}
		}
		if s.AdditionalProperties != nil {
			if allowed, ok := s.AdditionalProperties.(bool); ok {
				if !allowed && len(result.unevalProps) > 0 {
					errs = append(errs, validationError("additionalProperties", "additionalProperties %s not allowed", result.unevalPnames()))
				}
			} else {
				schema := s.AdditionalProperties.(*Schema)
				for pname := range result.unevalProps {
					if pvalue, ok := v[pname]; ok {
						if err = validate(schema, "additionalProperties", pvalue, escape(pname)); err != nil {
							if IsValidationError(err) {
								errs = append(errs, err)
							} else {
								return result, err
							}
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
						if IsValidationError(err) {
							errs = append(errs, err)
						} else {
							return result, err
						}
					}
				case []string:
					for i, pname := range dvalue {
						if _, ok := v[pname]; !ok {
							errs = append(errs, validationError("dependencies/"+escape(dname)+"/"+strconv.Itoa(i), "property %s is required, if %s property exists", quote(pname), quote(dname)))
						}
					}
				}
			}
		}
		for dname, dvalue := range s.DependentRequired {
			if _, ok := v[dname]; ok {
				for i, pname := range dvalue {
					if _, ok := v[pname]; !ok {
						errs = append(errs, validationError("dependentRequired/"+escape(dname)+"/"+strconv.Itoa(i), "property %s is required, if %s property exists", quote(pname), quote(dname)))
					}
				}
			}
		}
		for dname, sch := range s.DependentSchemas {
			if _, ok := v[dname]; ok {
				if err := validateInplace(sch, "dependentSchemas/"+escape(dname)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			}
		}

	case []interface{}:
		if s.MinItems != -1 && len(v) < s.MinItems {
			errs = append(errs, validationError("minItems", "minimum %d items required, but found %d items", s.MinItems, len(v)))
		}
		if s.MaxItems != -1 && len(v) > s.MaxItems {
			errs = append(errs, validationError("maxItems", "maximum %d items required, but found %d items", s.MaxItems, len(v)))
		}
		if s.UniqueItems {
			for i := 1; i < len(v); i++ {
				for j := 0; j < i; j++ {
					eq, err := equals(v[i], v[j])
					if err != nil {
						return result, err
					}
					if eq {
						errs = append(errs, validationError("uniqueItems", "items at index %d and %d are equal", j, i))
					}
				}
			}
		}

		// items + additionalItems
		switch items := s.Items.(type) {
		case *Schema:
			for i, item := range v {
				if err = validate(items, "items", item, strconv.Itoa(i)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			}
			result.unevalItems = nil
		case []*Schema:
			for i, item := range v {
				if i < len(items) {
					delete(result.unevalItems, i)
					if err = validate(items[i], "items/"+strconv.Itoa(i), item, strconv.Itoa(i)); err != nil {
						if IsValidationError(err) {
							errs = append(errs, err)
						} else {
							return result, err
						}
					}
				} else if sch, ok := s.AdditionalItems.(*Schema); ok {
					delete(result.unevalItems, i)
					if err = validate(sch, "additionalItems", item, strconv.Itoa(i)); err != nil {
						if IsValidationError(err) {
							errs = append(errs, err)
						} else {
							return result, err
						}
					}
				} else {
					break
				}
			}
			if additionalItems, ok := s.AdditionalItems.(bool); ok {
				if additionalItems {
					result.unevalItems = nil
				} else if len(v) > len(items) {
					errs = append(errs, validationError("additionalItems", "only %d items are allowed, but found %d items", len(items), len(v)))
				}
			}
		}

		// prefixItems + items
		for i, item := range v {
			if i < len(s.PrefixItems) {
				delete(result.unevalItems, i)
				if err = validate(s.PrefixItems[i], "prefixItems/"+strconv.Itoa(i), item, strconv.Itoa(i)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			} else if s.Items2020 != nil {
				delete(result.unevalItems, i)
				if err = validate(s.Items2020, "items", item, strconv.Itoa(i)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			} else {
				break
			}
		}

		// contains + minContains + maxContains
		if s.Contains != nil && (s.MinContains != -1 || s.MaxContains != -1) {
			matched := 0
			var causes []error
			for i, item := range v {
				if err = validate(s.Contains, "contains", item, strconv.Itoa(i)); err != nil {
					if IsValidationError(err) {
						causes = append(causes, err)
					} else {
						return result, err
					}
				} else {
					matched++
					if s.ContainsEval {
						delete(result.unevalItems, i)
					}
				}
			}
			if s.MinContains != -1 && matched < s.MinContains {
				errs = append(errs, validationError("minContains", "valid must be >= %d, but got %d", s.MinContains, matched).add(causes...))
			}
			if s.MaxContains != -1 && matched > s.MaxContains {
				errs = append(errs, validationError("maxContains", "valid must be <= %d, but got %d", s.MaxContains, matched))
			}
		}

	case string:
		// minLength + maxLength
		if s.MinLength != -1 || s.MaxLength != -1 {
			length := utf8.RuneCount([]byte(v))
			if s.MinLength != -1 && length < s.MinLength {
				errs = append(errs, validationError("minLength", "length must be >= %d, but got %d", s.MinLength, length))
			}
			if s.MaxLength != -1 && length > s.MaxLength {
				errs = append(errs, validationError("maxLength", "length must be <= %d, but got %d", s.MaxLength, length))
			}
		}

		if s.Pattern != nil && !s.Pattern.MatchString(v) {
			errs = append(errs, validationError("pattern", "does not match pattern %s", quote(s.Pattern.String())))
		}

		// contentEncoding + contentMediaType
		if s.decoder != nil || s.mediaType != nil {
			decoded := s.ContentEncoding == ""
			var content []byte
			if s.decoder != nil {
				b, err := s.decoder(v)
				if err != nil {
					errs = append(errs, validationError("contentEncoding", "%s is not %s encoded", quote(v), s.ContentEncoding))
				} else {
					content, decoded = b, true
				}
			}
			if decoded && s.mediaType != nil {
				if s.decoder == nil {
					content = []byte(v)
				}
				if err := s.mediaType(content); err != nil {
					errs = append(errs, validationError("contentMediaType", "value is not of mediatype %s", quote(s.ContentMediaType)))
				}
			}
		}

	case json.Number, float64, int, int32, int64:
		// lazy convert to *big.Rat to avoid allocation
		val, ok := new(big.Rat).SetString(fmt.Sprint(v))
		if !ok {
			return result, invalidJSONTypeError(v)
		}
		f64 := func(r *big.Rat) float64 {
			f, _ := r.Float64()
			return f
		}
		if s.Minimum != nil && val.Cmp(s.Minimum) < 0 {
			errs = append(errs, validationError("minimum", "must be >= %v but found %v", f64(s.Minimum), v))
		}
		if s.ExclusiveMinimum != nil && val.Cmp(s.ExclusiveMinimum) <= 0 {
			errs = append(errs, validationError("exclusiveMinimum", "must be > %v but found %v", f64(s.ExclusiveMinimum), v))
		}
		if s.Maximum != nil && val.Cmp(s.Maximum) > 0 {
			errs = append(errs, validationError("maximum", "must be <= %v but found %v", f64(s.Maximum), v))
		}
		if s.ExclusiveMaximum != nil && val.Cmp(s.ExclusiveMaximum) >= 0 {
			errs = append(errs, validationError("exclusiveMaximum", "must be < %v but found %v", f64(s.ExclusiveMaximum), v))
		}
		if s.MultipleOf != nil {
			if q := new(big.Rat).Quo(val, s.MultipleOf); !q.IsInt() {
				errs = append(errs, validationError("multipleOf", "%v not multipleOf %v", v, f64(s.MultipleOf)))
			}
		}
	}

	// $ref + $recursiveRef + $dynamicRef
	validateRef := func(sch *Schema, refPath string) error {
		if sch != nil {
			if err := validateInplace(sch, refPath); err != nil {
				if IsValidationError(err) {
					errs = append(errs, err)
				} else {
					return err
				}
				url := sch.Location
				if s.url() == sch.url() {
					url = sch.loc()
				}
				return validationError(refPath, "doesn't validate with %s", quote(url)).causes(err)
			}
		}
		return nil
	}
	if err = validateRef(s.Ref, "$ref"); err != nil {
		if IsValidationError(err) {
			errs = append(errs, err)
		} else {
			return result, err
		}
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
			if IsValidationError(err) {
				errs = append(errs, err)
			} else {
				return result, err
			}
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
			if IsValidationError(err) {
				errs = append(errs, err)
			} else {
				return result, err
			}
		}
	}
	if s.Not != nil {
		err = validateInplace(s.Not, "not")
		if err == nil {
			errs = append(errs, validationError("not", "not failed"))
		}
		if err != nil && !IsValidationError(err) {
			return result, err
		}
	}

	for i, sch := range s.AllOf {
		schPath := "allOf/" + strconv.Itoa(i)
		if err := validateInplace(sch, schPath); err != nil {
			if IsValidationError(err) {
				errs = append(errs, validationError(schPath, "allOf failed").add(err))
			} else {
				return result, err
			}
		}
	}

	if len(s.AnyOf) > 0 {
		matched := false
		var causes []error
		for i, sch := range s.AnyOf {
			if err := validateInplace(sch, "anyOf/"+strconv.Itoa(i)); err == nil {
				matched = true
			} else {
				if IsValidationError(err) {
					causes = append(causes, err)
				} else {
					return result, err
				}
			}
		}
		if !matched {
			errs = append(errs, validationError("anyOf", "anyOf failed").add(causes...))
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
					errs = append(errs, validationError("oneOf", "valid against schemas at indexes %d and %d", matched, i))
					break
				}
			} else {
				if IsValidationError(err) {
					causes = append(causes, err)
				} else {
					return result, err
				}
			}
		}
		if matched == -1 {
			errs = append(errs, validationError("oneOf", "oneOf failed").add(causes...))
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
					if IsValidationError(err) {
						errs = append(errs, validationError("then", "if-then failed").add(err))
					} else {
						return result, err
					}
				}
			}
		} else {
			if s.Else != nil {
				if err := validateInplace(s.Else, "else"); err != nil {
					if IsValidationError(err) {
						errs = append(errs, validationError("else", "if-else failed").add(err))
					} else {
						return result, err
					}
				}
			}
		}
		// restore dynamic scope
		scope[len(scope)-1].discard = false
	}

	for _, ext := range s.Extensions {
		if err := ext.Validate(ValidationContext{result, validate, validateInplace, validationError}, v); err != nil {
			if IsValidationError(err) {
				errs = append(errs, err)
			} else {
				return result, err
			}
		}
	}

	// UnevaluatedProperties + UnevaluatedItems
	switch v := v.(type) {
	case map[string]interface{}:
		if s.UnevaluatedProperties != nil {
			for pname := range result.unevalProps {
				if pvalue, ok := v[pname]; ok {
					if err := validate(s.UnevaluatedProperties, "UnevaluatedProperties", pvalue, escape(pname)); err != nil {
						if IsValidationError(err) {
							errs = append(errs, err)
						} else {
							return result, err
						}
					}
				}
			}
			result.unevalProps = nil
		}
	case []interface{}:
		if s.UnevaluatedItems != nil {
			for i := range result.unevalItems {
				if err := validate(s.UnevaluatedItems, "UnevaluatedItems", v[i], strconv.Itoa(i)); err != nil {
					if IsValidationError(err) {
						errs = append(errs, err)
					} else {
						return result, err
					}
				}
			}
			result.unevalItems = nil
		}
	}

	switch len(errs) {
	case 0:
		return result, nil
	case 1:
		return result, errs[0]
	default:
		return result, validationError("", "").add(errs...) // empty message, used just for wrapping
	}
}

type validationResult struct {
	unevalProps map[string]struct{}
	unevalItems map[int]struct{}
}

func (vr validationResult) unevalPnames() string {
	pnames := make([]string, 0, len(vr.unevalProps))
	for pname := range vr.unevalProps {
		pnames = append(pnames, quote(pname))
	}
	return strings.Join(pnames, ", ")
}

// jsonType returns the json type of given value v.
//
// It panics if the given value is not valid json value
func jsonType(v interface{}) (string, error) {
	switch v.(type) {
	case nil:
		return "null", nil
	case bool:
		return "boolean", nil
	case json.Number, float64, int, int32, int64:
		return "number", nil
	case string:
		return "string", nil
	case []interface{}:
		return "array", nil
	case map[string]interface{}:
		return "object", nil
	default:
		return "", invalidJSONTypeError(v)
	}
}

// equals tells if given two json values are equal or not.
func equals(v1, v2 interface{}) (bool, error) {
	v1Type, err := jsonType(v1)
	if err != nil {
		return false, err
	}
	v2Type, err := jsonType(v2)
	if err != nil {
		return false, err
	}
	if v1Type != v2Type {
		return false, nil
	}
	switch v1Type {
	case "array":
		arr1, arr2 := v1.([]interface{}), v2.([]interface{})
		if len(arr1) != len(arr2) {
			return false, nil
		}
		for i := range arr1 {
			eq, err := equals(arr1[i], arr2[i])
			if err != nil {
				return false, err
			}
			if !eq {
				return false, nil
			}
		}
		return true, nil
	case "object":
		obj1, obj2 := v1.(map[string]interface{}), v2.(map[string]interface{})
		if len(obj1) != len(obj2) {
			return false, nil
		}
		for k, v1 := range obj1 {
			if v2, ok := obj2[k]; ok {
				eq, err := equals(v1, v2)
				if err != nil {
					return false, err
				}
				if !eq {
					return false, nil
				}
			} else {
				return false, nil
			}
		}
		return true, nil
	case "number":
		num1, ok1 := new(big.Rat).SetString(fmt.Sprint(v1))
		num2, ok2 := new(big.Rat).SetString(fmt.Sprint(v2))
		if !ok1 {
			return false, invalidJSONTypeError(v1)
		}
		if !ok2 {
			return false, invalidJSONTypeError(v2)
		}
		return num1.Cmp(num2) == 0, nil
	default:
		return v1 == v2, nil
	}
}

// escape converts given token to valid json-pointer token
func escape(token string) string {
	token = strings.Replace(token, "~", "~0", -1)
	token = strings.Replace(token, "/", "~1", -1)
	return url.PathEscape(token)
}
