package jsonschema

import (
	"fmt"
	"slices"
	"strings"
)

type Position uint

const (
	PosProp Position = 0
	PosItem Position = 1
)

type SchemaPosition []Position

func (sp SchemaPosition) collect(v any, ptr jsonPointer, target map[jsonPointer]any) {
	if len(sp) == 0 {
		target[ptr] = v
		return
	}
	p, sp := sp[0], sp[1:]
	switch p {
	case PosProp:
		if obj, ok := v.(map[string]any); ok {
			for pname, pvalue := range obj {
				ptr := ptr.append(pname)
				sp.collect(pvalue, ptr, target)
			}
		}
	case PosItem:
		if arr, ok := v.([]any); ok {
			for i, item := range arr {
				ptr := ptr.append(fmt.Sprint(i))
				sp.collect(item, ptr, target)
			}
		}
	}
}

type Subschemas map[string][]SchemaPosition

func (ss Subschemas) collect(obj map[string]any, ptr jsonPointer, target map[jsonPointer]any) {
	for kw, spp := range ss {
		v, ok := obj[kw]
		if !ok {
			continue
		}
		ptr := ptr.append(kw)
		for _, sp := range spp {
			sp.collect(v, ptr, target)
		}
	}
}

type Draft struct {
	version       int
	url           string
	sch           *Schema
	id            string             // property name used to represent id
	subschemas    Subschemas         // locations of subschemas
	vocabPrefix   string             // prefix used for vocabulary
	allVocabs     map[string]*Schema // names of supported vocabs with its schemas
	defaultVocabs []string           // names of default vocabs
}

func (d *Draft) String() string {
	return d.url
}

var (
	Draft4 = &Draft{
		version: 4,
		url:     "http://json-schema.org/draft-04/schema",
		id:      "id",
		subschemas: Subschemas{
			// type agonistic
			"definitions": {{PosProp}},
			"not":         {{}},
			"allOf":       {{PosItem}},
			"anyOf":       {{PosItem}},
			"oneOf":       {{PosItem}},
			// object
			"properties":           {{PosProp}},
			"additionalProperties": {{}},
			"patternProperties":    {{PosProp}},
			// array
			"items":           {{}, {PosItem}},
			"additionalItems": {{}},
			"dependencies":    {{PosProp}},
		},
		vocabPrefix:   "",
		allVocabs:     map[string]*Schema{},
		defaultVocabs: []string{},
	}

	Draft6 = &Draft{
		version: 6,
		url:     "http://json-schema.org/draft-06/schema",
		id:      "$id",
		subschemas: joinMaps(Draft4.subschemas, Subschemas{
			"propertyNames": {{}},
			"contains":      {{}},
		}),
		vocabPrefix:   "",
		allVocabs:     map[string]*Schema{},
		defaultVocabs: []string{},
	}

	Draft7 = &Draft{
		version: 7,
		url:     "http://json-schema.org/draft-07/schema",
		id:      "$id",
		subschemas: joinMaps(Draft6.subschemas, Subschemas{
			"if":   {{}},
			"then": {{}},
			"else": {{}},
		}),
		vocabPrefix:   "",
		allVocabs:     map[string]*Schema{},
		defaultVocabs: []string{},
	}

	Draft2019 = &Draft{
		version: 2019,
		url:     "https://json-schema.org/draft/2019-09/schema",
		id:      "$id",
		subschemas: joinMaps(Draft7.subschemas, Subschemas{
			"$defs":                 {{PosProp}},
			"dependentSchemas":      {{PosProp}},
			"unevaluatedProperties": {{}},
			"unevaluatedItems":      {{}},
			"contentSchema":         {{}},
		}),
		vocabPrefix: "https://json-schema.org/draft/2019-09/vocab/",
		allVocabs: map[string]*Schema{
			"core":       nil,
			"applicator": nil,
			"validation": nil,
			"meta-data":  nil,
			"format":     nil,
			"content":    nil,
		},
		defaultVocabs: []string{"core", "applicator", "validation"},
	}

	Draft2020 = &Draft{
		version: 2020,
		url:     "https://json-schema.org/draft/2020-12/schema",
		id:      "$id",
		subschemas: joinMaps(Draft2019.subschemas, Subschemas{
			"prefixItems": {{PosItem}},
		}),
		vocabPrefix: "https://json-schema.org/draft/2020-12/vocab/",
		allVocabs: map[string]*Schema{
			"core":              nil,
			"applicator":        nil,
			"unevaluated":       nil,
			"validation":        nil,
			"meta-data":         nil,
			"format-annotation": nil,
			"format-assertion":  nil,
			"content":           nil,
		},
		defaultVocabs: []string{"core", "applicator", "unevaluated", "validation"},
	}

	draftLatest = Draft2020
)

func init() {
	c := NewCompiler()
	c.AssertFormat()
	for _, d := range []*Draft{Draft4, Draft6, Draft7, Draft2019, Draft2020} {
		d.sch = c.MustCompile(d.url)
		for name := range d.allVocabs {
			d.allVocabs[name] = c.MustCompile(strings.TrimSuffix(d.url, "schema") + "meta/" + name)
		}
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

func (d *Draft) getVocabs(url url, doc any, vocabularies map[string]*Vocabulary) ([]string, error) {
	if d.version < 2019 {
		return nil, nil
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, nil
	}
	v, ok := obj["$vocabulary"]
	if !ok {
		return nil, nil
	}
	obj, ok = v.(map[string]any)
	if !ok {
		return nil, nil
	}

	var vocabs []string
	for vocab, reqd := range obj {
		if reqd, ok := reqd.(bool); !ok || !reqd {
			continue
		}
		name, ok := strings.CutPrefix(vocab, d.vocabPrefix)
		if ok {
			if _, ok := d.allVocabs[name]; ok {
				if !slices.Contains(vocabs, name) {
					vocabs = append(vocabs, name)
					continue
				}
			}
		}
		if _, ok := vocabularies[vocab]; !ok {
			return nil, &UnsupportedVocabularyError{url.String(), vocab}
		}
		if !slices.Contains(vocabs, vocab) {
			vocabs = append(vocabs, vocab)
		}
	}
	return vocabs, nil
}

// --

type dialect struct {
	draft  *Draft
	vocabs []string // nil means use draft.defaultVocabs
}

func (d *dialect) hasVocab(name string) bool {
	if name == "core" || d.draft.version < 2019 {
		return true
	}
	if d.vocabs != nil {
		return slices.Contains(d.vocabs, name)
	}
	return slices.Contains(d.draft.defaultVocabs, name)
}

func (d *dialect) activeVocabs(assertVocabs bool, vocabularies map[string]*Vocabulary) []string {
	if len(vocabularies) == 0 {
		return d.vocabs
	}
	if d.draft.version < 2019 {
		assertVocabs = true
	}
	if !assertVocabs {
		return d.vocabs
	}
	var vocabs []string
	if d.vocabs == nil {
		vocabs = slices.Clone(d.draft.defaultVocabs)
	} else {
		vocabs = slices.Clone(d.vocabs)
	}
	for vocab := range vocabularies {
		if !slices.Contains(vocabs, vocab) {
			vocabs = append(vocabs, vocab)
		}
	}
	return vocabs
}

func (d *dialect) getSchema(assertVocabs bool, vocabularies map[string]*Vocabulary) *Schema {
	vocabs := d.activeVocabs(assertVocabs, vocabularies)
	if vocabs == nil {
		return d.draft.sch
	}

	var allOf []*Schema
	for _, vocab := range vocabs {
		sch := d.draft.allVocabs[vocab]
		if sch == nil {
			if v, ok := vocabularies[vocab]; ok {
				sch = v.Schema
			}
		}
		if sch != nil {
			allOf = append(allOf, sch)
		}
	}
	if !slices.Contains(vocabs, "core") {
		sch := d.draft.allVocabs["core"]
		if sch == nil {
			sch = d.draft.sch
		}
		allOf = append(allOf, sch)
	}
	sch := &Schema{
		Location:     "urn:mem:metaschema",
		up:           urlPtr{url("urn:mem:metaschema"), ""},
		DraftVersion: d.draft.version,
		AllOf:        allOf,
	}
	sch.resource = sch
	if sch.DraftVersion >= 2020 {
		sch.DynamicAnchor = "meta"
		sch.dynamicAnchors = map[string]*Schema{
			"meta": sch,
		}
	}
	return sch
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

func joinMaps(m1 Subschemas, m2 Subschemas) Subschemas {
	m := Subschemas{}
	for k, v := range m1 {
		m[k] = v
	}
	for k, v := range m2 {
		m[k] = v
	}
	return m
}
