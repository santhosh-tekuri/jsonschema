package jsonschema

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// A Draft represents json-schema specification.
type Draft struct {
	version       int
	url           string
	id            string       // property name used to represent id
	subschemas    []SchemaPath // locations of subschemas
	vocabPrefix   string       // prefix used for vocabulary
	allVocabs     []string     // names of supported vocabs
	defaultVocabs []string     // names of default vocabs

	metaOnce    sync.Once
	sch         *Schema            // metaschema, compiled on first use
	vocabSchema map[string]*Schema // metaschema of each vocab, compiled on first use
}

// String returns the specification url.
func (d *Draft) String() string {
	return d.url
}

var (
	Draft4 = &Draft{
		version: 4,
		url:     "http://json-schema.org/draft-04/schema",
		id:      "id",
		subschemas: []SchemaPath{
			// type agonistic
			schemaPath("definitions/*"),
			schemaPath("not"),
			schemaPath("allOf/[]"),
			schemaPath("anyOf/[]"),
			schemaPath("oneOf/[]"),
			// object
			schemaPath("properties/*"),
			schemaPath("additionalProperties"),
			schemaPath("patternProperties/*"),
			// array
			schemaPath("items"),
			schemaPath("items/[]"),
			schemaPath("additionalItems"),
			schemaPath("dependencies/*"),
		},
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft6 = &Draft{
		version: 6,
		url:     "http://json-schema.org/draft-06/schema",
		id:      "$id",
		subschemas: joinSubschemas(Draft4.subschemas,
			schemaPath("propertyNames"),
			schemaPath("contains"),
		),
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft7 = &Draft{
		version: 7,
		url:     "http://json-schema.org/draft-07/schema",
		id:      "$id",
		subschemas: joinSubschemas(Draft6.subschemas,
			schemaPath("if"),
			schemaPath("then"),
			schemaPath("else"),
		),
		vocabPrefix:   "",
		allVocabs:     []string{},
		defaultVocabs: []string{},
	}

	Draft2019 = &Draft{
		version: 2019,
		url:     "https://json-schema.org/draft/2019-09/schema",
		id:      "$id",
		subschemas: joinSubschemas(Draft7.subschemas,
			schemaPath("$defs/*"),
			schemaPath("dependentSchemas/*"),
			schemaPath("unevaluatedProperties"),
			schemaPath("unevaluatedItems"),
			schemaPath("contentSchema"),
		),
		vocabPrefix:   "https://json-schema.org/draft/2019-09/vocab/",
		allVocabs:     []string{"core", "applicator", "validation", "meta-data", "format", "content"},
		defaultVocabs: []string{"core", "applicator", "validation"},
	}

	Draft2020 = &Draft{
		version: 2020,
		url:     "https://json-schema.org/draft/2020-12/schema",
		id:      "$id",
		subschemas: joinSubschemas(Draft2019.subschemas,
			schemaPath("prefixItems/[]"),
		),
		vocabPrefix: "https://json-schema.org/draft/2020-12/vocab/",
		allVocabs: []string{
			"core", "applicator", "unevaluated", "validation",
			"meta-data", "format-annotation", "format-assertion", "content",
		},
		defaultVocabs: []string{"core", "applicator", "unevaluated", "validation"},
	}

	draftLatest = Draft2020
)

// meta returns the draft's metaschema and the metaschema of each of its
// vocabs. They are compiled on first use rather than in init, so a program
// pays only for the drafts it uses, and nothing if it compiles no schema.
func (d *Draft) meta() (*Schema, map[string]*Schema) {
	d.metaOnce.Do(func() {
		c := NewCompiler()
		c.AssertFormat()
		d.sch = c.MustCompile(d.url)
		d.vocabSchema = make(map[string]*Schema, len(d.allVocabs))
		for _, name := range d.allVocabs {
			d.vocabSchema[name] = c.MustCompile(strings.TrimSuffix(d.url, "schema") + "meta/" + name)
		}
	})
	return d.sch, d.vocabSchema
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
			if slices.Contains(d.allVocabs, name) {
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
	if !slices.Contains(vocabs, "core") {
		vocabs = append(vocabs, "core")
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
	draftSch, vocabSchema := d.draft.meta()
	vocabs := d.activeVocabs(assertVocabs, vocabularies)
	if vocabs == nil {
		return draftSch
	}

	var allOf []*Schema
	for _, vocab := range vocabs {
		sch := vocabSchema[vocab]
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
		sch := vocabSchema["core"]
		if sch == nil {
			sch = draftSch
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

func joinSubschemas(a1 []SchemaPath, a2 ...SchemaPath) []SchemaPath {
	var a []SchemaPath
	a = append(a, a1...)
	a = append(a, a2...)
	return a
}
