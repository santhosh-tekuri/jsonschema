package jsonschema

import (
	"fmt"
	"strconv"
	"strings"
)

type position uint

const (
	posSelf position = 1 << iota
	posProp
	posItem
)

type Draft struct {
	version       int
	url           string
	sch           *Schema
	id            string              // property name used to represent id
	subschemas    map[string]position // locations of subschemas
	vocabPrefix   string              // prefix used for vocabulary
	allVocabs     []string            // names of supported vocabs
	defaultVocabs []string            // names of default vocabs
}

var (
	Draft4 = &Draft{
		version: 4,
		url:     "http://json-schema.org/draft-04/schema",
		id:      "id",
		subschemas: map[string]position{
			// type agonistic
			"definitions": posProp,
			"not":         posSelf,
			"allOf":       posItem,
			"anyOf":       posItem,
			"oneOf":       posItem,
			// object
			"properties":           posProp,
			"additionalProperties": posSelf,
			"patternProperties":    posProp,
			// array
			"items":           posSelf | posItem,
			"additionalItems": posSelf,
			"dependencies":    posProp,
		},
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft6 = &Draft{
		version: 6,
		url:     "http://json-schema.org/draft-06/schema",
		id:      "$id",
		subschemas: joinMaps(Draft4.subschemas, map[string]position{
			"propertyNames": posSelf,
			"contains":      posSelf,
		}),
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft7 = &Draft{
		version: 7,
		url:     "http://json-schema.org/draft-07/schema",
		id:      "$id",
		subschemas: joinMaps(Draft6.subschemas, map[string]position{
			"if":   posSelf,
			"then": posSelf,
			"else": posSelf,
		}),
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft2019 = &Draft{
		version: 2019,
		url:     "https://json-schema.org/draft/2019-09/schema",
		id:      "$id",
		subschemas: joinMaps(Draft7.subschemas, map[string]position{
			"$defs":                 posProp,
			"dependentSchemas":      posProp,
			"unevaluatedProperties": posSelf,
			"unevaluatedItems":      posSelf,
			"contentSchema":         posSelf,
		}),
		vocabPrefix: "https://json-schema.org/draft/2019-09/vocab/",
		allVocabs: []string{
			"core",
			"applicator",
			"validation",
			"meta-data",
			"format",
			"content",
		},
		defaultVocabs: []string{"core", "applicator", "validation"},
	}

	Draft2020 = &Draft{
		version: 2020,
		url:     "https://json-schema.org/draft/2020-12/schema",
		id:      "$id",
		subschemas: joinMaps(Draft2019.subschemas, map[string]position{
			"prefixItems": posItem,
		}),
		vocabPrefix: "https://json-schema.org/draft/2020-12/vocab/",
		allVocabs: []string{
			"core",
			"applicator",
			"unevaluated",
			"validation",
			"meta-data",
			"format-annotation",
			"format-assertion",
			"content",
		},
		defaultVocabs: []string{"core", "applicator", "unevaluated", "validation"},
	}

	draftLatest = Draft2020
)

func init() {
	for _, d := range []*Draft{Draft4, Draft6, Draft7, Draft2019, Draft2020} {
		c := NewCompiler()
		c.AssertFormat()
		d.sch = c.MustCompile(d.url)
	}
}

func draftFromVersion(version int) *Draft {
	switch version {
	case 4:
		return Draft4
	case 6:
		return Draft6
	case 7:
		return Draft7
	case 2019:
		return Draft2019
	case 2020:
		return Draft2020
	default:
		return nil
	}
}

func draftFromURL(url string) *Draft {
	u, frag := split(url)
	if frag != "" {
		return nil
	}
	u, ok := strings.CutPrefix(u, "http://")
	if !ok {
		u, _ = strings.CutPrefix(u, "https://")
	}
	switch u {
	case "json-schema.org/schema":
		return draftLatest
	case "json-schema.org/draft/2020-12/schema":
		return Draft2020
	case "json-schema.org/draft/2019-09/schema":
		return Draft2019
	case "json-schema.org/draft-07/schema":
		return Draft7
	case "json-schema.org/draft-06/schema":
		return Draft6
	case "json-schema.org/draft-04/schema":
		return Draft4
	default:
		return nil
	}
}

func (d *Draft) getID(obj map[string]any) string {
	if d.version < 2019 {
		if _, ok := obj["$ref"]; ok {
			// All other properties in a "$ref" object MUST be ignored
			return ""
		}
	}

	id, ok := strVal(obj, d.id)
	if !ok {
		return ""
	}
	id, _ = split(id) // ignore fragment
	return id
}

func (d *Draft) collectAnchors(sch any, schPtr jsonPointer, res *resource, url url) error {
	obj, ok := sch.(map[string]any)
	if !ok {
		return nil
	}

	addAnchor := func(anchor anchor) error {
		ptr1, ok := res.anchors[anchor]
		if ok {
			if ptr1 == schPtr {
				// anchor with same root_ptr already exists
				return nil
			}
			return &DuplicateAnchorError{
				string(anchor), url.String(), string(ptr1), string(schPtr),
			}
		}
		res.anchors[anchor] = schPtr
		return nil
	}

	if d.version < 2019 {
		if _, ok := obj["$ref"]; ok {
			// All other properties in a "$ref" object MUST be ignored
			return nil
		}
		// anchor is specified in id
		if id, ok := strVal(obj, d.id); ok {
			_, frag, err := splitFragment(id)
			if err != nil {
				loc := urlPtr{url, schPtr}
				return &ParseAnchorError{loc.String()}
			}
			if anchor, ok := frag.convert().(anchor); ok {
				if err := addAnchor(anchor); err != nil {
					return err
				}
			}
		}
	}
	if d.version >= 2019 {
		if s, ok := strVal(obj, "$anchor"); ok {
			if err := addAnchor(anchor(s)); err != nil {
				return err
			}
		}
	}
	if d.version >= 2020 {
		if s, ok := strVal(obj, "$dynamicAnchor"); ok {
			if err := addAnchor(anchor(s)); err != nil {
				return err
			}
			res.dynamicAnchors = append(res.dynamicAnchors, anchor(s))
		}
	}

	return nil
}

func (d *Draft) collectResources(sch any, base url, schPtr jsonPointer, url url, resources map[jsonPointer]*resource) error {
	if _, ok := resources[schPtr]; ok {
		// resources are already collected
		return nil
	}
	if _, ok := sch.(bool); ok {
		if schPtr.isEmpty() {
			// root resource
			resources[schPtr] = newResource(schPtr, base)
		}
		return nil
	}
	obj, ok := sch.(map[string]any)
	if !ok {
		return nil
	}

	if sch, ok := obj["$schema"]; ok {
		if sch, ok := sch.(string); ok && sch != "" {
			if got := draftFromURL(sch); got != nil && got != d {
				loc := urlPtr{url, schPtr}
				return &MetaSchemaMismatchError{loc.String()}
			}
		}
	}

	var res *resource
	if id := d.getID(obj); id != "" {
		uf, err := base.join(id)
		if err != nil {
			loc := urlPtr{url, schPtr}
			return &ParseIDError{loc.String()}
		}
		base = uf.url
		res = newResource(schPtr, base)
	} else if schPtr.isEmpty() {
		// root resource
		res = newResource(schPtr, base)
	}

	if res != nil {
		for _, res := range resources {
			if res.id == base {
				return &DuplicateIDError{base.String(), url.String(), string(schPtr), string(res.ptr)}
			}
		}
		resources[schPtr] = res
	}

	// collect anchors into base resource
	for _, res := range resources {
		if res.id == base {
			// found base resource
			if err := d.collectAnchors(sch, schPtr, res, url); err != nil {
				return err
			}
			break
		}
	}

	for kw, pos := range d.subschemas {
		v, ok := obj[kw]
		if !ok {
			continue
		}
		if pos&posSelf != 0 {
			ptr := schPtr.append(kw)
			if err := d.collectResources(v, base, ptr, url, resources); err != nil {
				return err
			}
		}
		if pos&posItem != 0 {
			if arr, ok := v.([]any); ok {
				for i, item := range arr {
					ptr := schPtr.append2(kw, fmt.Sprint(i))
					if err := d.collectResources(item, base, ptr, url, resources); err != nil {
						return err
					}
				}
			}
		}
		if pos&posProp != 0 {
			if obj, ok := v.(map[string]any); ok {
				for pname, pvalue := range obj {
					ptr := schPtr.append2(kw, pname)
					if err := d.collectResources(pvalue, base, ptr, url, resources); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (d *Draft) isSubschema(ptr string) bool {
	if ptr == "" {
		return true
	}

	split := func(ptr string) (string, string) {
		ptr = ptr[1:] // rm `/` prefix
		if slash := strings.IndexByte(ptr, '/'); slash != -1 {
			return ptr[:slash], ptr[slash:]
		} else {
			return ptr, ""
		}
	}

	tok, ptr := split(ptr)
	if pos, ok := d.subschemas[tok]; ok {
		if pos&posSelf != 0 && d.isSubschema(ptr) {
			return true
		}
		if ptr != "" {
			if pos&posProp != 0 {
				_, ptr := split(ptr)
				if d.isSubschema(ptr) {
					return true
				}
			}
			if pos&posItem != 0 {
				tok, ptr := split(ptr)
				_, err := strconv.Atoi(tok)
				if err == nil && d.isSubschema(ptr) {
					return true
				}
			}
		}
	}

	return false
}

func (d *Draft) validate(up urlPtr, v any) error {
	err := d.sch.Validate(v)
	if err != nil {
		return &SchemaValidationError{URL: up.String(), Err: err}
	}
	return nil
}

// --

type ParseIDError struct {
	URL string
}

func (e *ParseIDError) Error() string {
	return fmt.Sprintf("error in parsing id at %q", e.URL)
}

// --

type ParseAnchorError struct {
	URL string
}

func (e *ParseAnchorError) Error() string {
	return fmt.Sprintf("error in parsing anchor at %q", e.URL)
}

// --

type DuplicateIDError struct {
	ID   string
	URL  string
	Ptr1 string
	Ptr2 string
}

func (e *DuplicateIDError) Error() string {
	return fmt.Sprintf("duplicate id %q in %q at %q and %q", e.ID, e.URL, e.Ptr1, e.Ptr2)
}

// --

type DuplicateAnchorError struct {
	Anchor string
	URL    string
	Ptr1   string
	Ptr2   string
}

func (e *DuplicateAnchorError) Error() string {
	return fmt.Sprintf("duplicate anchor %q in %q at %q and %q", e.Anchor, e.URL, e.Ptr1, e.Ptr2)
}

// --

func joinMaps(m1 map[string]position, m2 map[string]position) map[string]position {
	m := make(map[string]position)
	for k, v := range m1 {
		m[k] = v
	}
	for k, v := range m2 {
		m[k] = v
	}
	return m
}
