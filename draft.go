package jsonschema

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
)

// A Draft represents json-schema draft
type Draft struct {
	version      int
	meta         *Schema
	id           string   // property name used to represent schema id.
	boolSchema   bool     // is boolean valid schema
	vocab        []string // built-in vocab
	defaultVocab []string // vocabs when $vocabulary is not used
	subschemas   map[string]position
}

func (d *Draft) URL() string {
	switch d.version {
	case 2020:
		return "https://json-schema.org/draft/2020-12/schema"
	case 2019:
		return "https://json-schema.org/draft/2019-09/schema"
	case 7:
		return "https://json-schema.org/draft-07/schema"
	case 6:
		return "https://json-schema.org/draft-06/schema"
	case 4:
		return "https://json-schema.org/draft-04/schema"
	}
	return ""
}

func (d *Draft) String() string {
	return fmt.Sprintf("Draft%d", d.version)
}

func (d *Draft) loadMeta(url string) {
	c := NewCompiler()
	c.AssertFormat = true
	d.meta = c.MustCompile(url)
	d.meta.meta = d.meta
}

func (d *Draft) getID(sch interface{}) string {
	m, ok := sch.(map[string]interface{})
	if !ok {
		return ""
	}
	if _, ok := m["$ref"]; ok && d.version <= 7 {
		// $ref prevents a sibling id from changing the base uri
		return ""
	}
	v, ok := m[d.id]
	if !ok {
		return ""
	}
	id, ok := v.(string)
	if !ok {
		return ""
	}
	return id
}

func (d *Draft) resolveID(base string, sch interface{}) (string, error) {
	id, _ := split(d.getID(sch)) // strip fragment
	if id == "" {
		return "", nil
	}
	url, err := resolveURL(base, id)
	url, _ = split(url) // strip fragment
	return url, err
}

func (d *Draft) anchors(sch interface{}) []string {
	m, ok := sch.(map[string]interface{})
	if !ok {
		return nil
	}

	var anchors []string

	// before draft2019, anchor is specified in id
	_, f := split(d.getID(m))
	if f != "#" {
		anchors = append(anchors, f[1:])
	}

	if v, ok := m["$anchor"]; ok && d.version >= 2019 {
		anchors = append(anchors, v.(string))
	}
	if v, ok := m["$dynamicAnchor"]; ok && d.version >= 2020 {
		anchors = append(anchors, v.(string))
	}
	return anchors
}

// listSubschemas collects subschemas in r into rr.
func (d *Draft) listSubschemas(r *resource, base string, rr map[string]*resource) error {
	add := func(loc string, sch interface{}) error {
		url, err := d.resolveID(base, sch)
		if err != nil {
			return err
		}
		floc := r.floc + "/" + loc
		sr := &resource{url: url, floc: floc, doc: sch}
		rr[floc] = sr

		base := base
		if url != "" {
			base = url
		}
		return d.listSubschemas(sr, base, rr)
	}

	sch, ok := r.doc.(map[string]interface{})
	if !ok {
		return nil
	}
	for kw, pos := range d.subschemas {
		v, ok := sch[kw]
		if !ok {
			continue
		}
		if pos&self != 0 {
			switch v := v.(type) {
			case map[string]interface{}:
				if err := add(kw, v); err != nil {
					return err
				}
			case bool:
				if d.boolSchema {
					if err := add(kw, v); err != nil {
						return err
					}
				}
			}
		}
		if pos&item != 0 {
			if v, ok := v.([]interface{}); ok {
				for i, item := range v {
					if err := add(kw+"/"+strconv.Itoa(i), item); err != nil {
						return err
					}
				}
			}
		}
		if pos&prop != 0 {
			if v, ok := v.(map[string]interface{}); ok {
				for pname, pval := range v {
					if err := add(kw+"/"+escape(pname), pval); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// isVocab tells whether url is built-in vocab.
func (d *Draft) isVocab(url string) bool {
	for _, v := range d.vocab {
		if url == v {
			return true
		}
	}
	return false
}

type position uint

const (
	self position = 1 << iota
	prop
	item
)

// supported drafts
var (
	Draft4    = &Draft{version: 4, id: "id", boolSchema: false}
	Draft6    = &Draft{version: 6, id: "$id", boolSchema: true}
	Draft7    = &Draft{version: 7, id: "$id", boolSchema: true}
	Draft2019 = &Draft{
		version:    2019,
		id:         "$id",
		boolSchema: true,
		vocab: []string{
			"https://json-schema.org/draft/2019-09/vocab/core",
			"https://json-schema.org/draft/2019-09/vocab/applicator",
			"https://json-schema.org/draft/2019-09/vocab/validation",
			"https://json-schema.org/draft/2019-09/vocab/meta-data",
			"https://json-schema.org/draft/2019-09/vocab/format",
			"https://json-schema.org/draft/2019-09/vocab/content",
		},
		defaultVocab: []string{
			"https://json-schema.org/draft/2019-09/vocab/core",
			"https://json-schema.org/draft/2019-09/vocab/applicator",
			"https://json-schema.org/draft/2019-09/vocab/validation",
		},
	}
	Draft2020 = &Draft{
		version:    2020,
		id:         "$id",
		boolSchema: true,
		vocab: []string{
			"https://json-schema.org/draft/2020-12/vocab/core",
			"https://json-schema.org/draft/2020-12/vocab/applicator",
			"https://json-schema.org/draft/2020-12/vocab/unevaluated",
			"https://json-schema.org/draft/2020-12/vocab/validation",
			"https://json-schema.org/draft/2020-12/vocab/meta-data",
			"https://json-schema.org/draft/2020-12/vocab/format-annotation",
			"https://json-schema.org/draft/2020-12/vocab/format-assertion",
			"https://json-schema.org/draft/2020-12/vocab/content",
		},
		defaultVocab: []string{
			"https://json-schema.org/draft/2020-12/vocab/core",
			"https://json-schema.org/draft/2020-12/vocab/applicator",
			"https://json-schema.org/draft/2020-12/vocab/unevaluated",
			"https://json-schema.org/draft/2020-12/vocab/validation",
		},
	}

	latest = Draft2020
)

func findDraft(url string) *Draft {
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}
	if strings.HasSuffix(url, "#") || strings.HasSuffix(url, "#/") {
		url = url[:strings.IndexByte(url, '#')]
	}
	switch url {
	case "https://json-schema.org/schema":
		return latest
	case "https://json-schema.org/draft/2020-12/schema":
		return Draft2020
	case "https://json-schema.org/draft/2019-09/schema":
		return Draft2019
	case "https://json-schema.org/draft-07/schema":
		return Draft7
	case "https://json-schema.org/draft-06/schema":
		return Draft6
	case "https://json-schema.org/draft-04/schema":
		return Draft4
	}
	return nil
}

//go:embed metaschemas
var metaFiles embed.FS

func init() {
	subschemas := map[string]position{
		// type agnostic
		"definitions": prop,
		"not":         self,
		"allOf":       item,
		"anyOf":       item,
		"oneOf":       item,
		// object
		"properties":           prop,
		"additionalProperties": self,
		"patternProperties":    prop,
		// array
		"items":           self | item,
		"additionalItems": self,
		"dependencies":    prop,
	}
	Draft4.subschemas = clone(subschemas)

	subschemas["propertyNames"] = self
	subschemas["contains"] = self
	Draft6.subschemas = clone(subschemas)

	subschemas["if"] = self
	subschemas["then"] = self
	subschemas["else"] = self
	Draft7.subschemas = clone(subschemas)

	subschemas["$defs"] = prop
	subschemas["dependentSchemas"] = prop
	subschemas["unevaluatedProperties"] = self
	subschemas["unevaluatedItems"] = self
	subschemas["contentSchema"] = self
	Draft2019.subschemas = clone(subschemas)

	subschemas["prefixItems"] = item
	Draft2020.subschemas = clone(subschemas)

	Draft4.loadMeta("http://json-schema.org/draft-04/schema")
	Draft6.loadMeta("http://json-schema.org/draft-06/schema")
	Draft7.loadMeta("http://json-schema.org/draft-07/schema")
	Draft2019.loadMeta("https://json-schema.org/draft/2019-09/schema")
	Draft2020.loadMeta("https://json-schema.org/draft/2020-12/schema")
}

func clone(m map[string]position) map[string]position {
	mm := make(map[string]position)
	for k, v := range m {
		mm[k] = v
	}
	return mm
}
