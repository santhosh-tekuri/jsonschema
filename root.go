package jsonschema

import (
	"fmt"
	"slices"
	"strings"
)

type root struct {
	url                 url
	doc                 any
	resources           map[jsonPointer]*resource
	subschemasProcessed map[jsonPointer]struct{}
}

func (r *root) rootResource() *resource {
	res, ok := r.resources[""]
	if !ok {
		panic(&Bug{fmt.Sprintf("root resource should exist for %q", r.url)})
	}
	return res
}

func (r *root) resource(ptr jsonPointer) *resource {
	for {
		if res, ok := r.resources[ptr]; ok {
			return res
		}
		slash := strings.LastIndexByte(string(ptr), '/')
		if slash == -1 {
			break
		}
		ptr = ptr[:slash]
	}
	return r.rootResource()
}

func (r *root) resolveFragmentIn(frag fragment, res *resource) (urlPtr, error) {
	var ptr jsonPointer
	switch f := frag.convert().(type) {
	case jsonPointer:
		ptr = res.ptr.concat(f)
	case anchor:
		aptr, ok := res.anchors[f]
		if !ok {
			return urlPtr{}, &AnchorNotFoundError{
				URL:       r.url.String(),
				Reference: (&urlFrag{res.id, frag}).String(),
			}
		}
		ptr = aptr
	}
	return urlPtr{r.url, ptr}, nil
}

func (r *root) resolveFragment(frag fragment) (urlPtr, error) {
	return r.resolveFragmentIn(frag, r.rootResource())
}

// resovles urlFrag to urlPtr from root.
// returns nil if it is external.
func (r *root) resolve(uf urlFrag) (*urlPtr, error) {
	var res *resource
	if uf.url == r.url {
		res = r.rootResource()
	} else {
		// look for resource with id==uf.url
		for _, v := range r.resources {
			if v.id == uf.url {
				res = v
				break
			}
		}
		if res == nil {
			return nil, nil // external url
		}
	}
	up, err := r.resolveFragmentIn(uf.frag, res)
	return &up, err
}

func (r *root) collectResources(loader *defaultLoader, vocabularies map[string]*Vocabulary, sch any, base url, schPtr jsonPointer, fallback dialect) error {
	if _, ok := r.subschemasProcessed[schPtr]; ok {
		return nil
	}
	if err := r._collectResources(loader, vocabularies, sch, base, schPtr, fallback); err != nil {
		return err
	}
	r.subschemasProcessed[schPtr] = struct{}{}
	return nil
}

func (r *root) _collectResources(loader *defaultLoader, vocabularies map[string]*Vocabulary, sch any, base url, schPtr jsonPointer, fallback dialect) error {
	if _, ok := sch.(bool); ok {
		if schPtr.isEmpty() {
			// root resource
			res := newResource(schPtr, base)
			res.dialect = fallback
			r.resources[schPtr] = res
		}
		return nil
	}
	obj, ok := sch.(map[string]any)
	if !ok {
		return nil
	}

	hasSchema := false
	if sch, ok := obj["$schema"]; ok {
		if _, ok := sch.(string); ok {
			hasSchema = true
		}
	}

	draft, err := loader.getDraft(urlPtr{r.url, schPtr}, sch, fallback.draft, map[url]struct{}{})
	if err != nil {
		return err
	}
	id := draft.getID(obj)
	if id == "" && !schPtr.isEmpty() {
		// ignore $schema
		draft = fallback.draft
		hasSchema = false
		id = draft.getID(obj)
	}

	var res *resource
	if id != "" {
		uf, err := base.join(id)
		if err != nil {
			loc := urlPtr{r.url, schPtr}
			return &ParseIDError{loc.String()}
		}
		base = uf.url
		res = newResource(schPtr, base)
	} else if schPtr.isEmpty() {
		// root resource
		res = newResource(schPtr, base)
	}

	if res != nil {
		found := false
		for _, res := range r.resources {
			if res.id == base {
				found = true
				if res.ptr != schPtr {
					return &DuplicateIDError{base.String(), r.url.String(), string(schPtr), string(res.ptr)}
				}
			}
		}
		if !found {
			if hasSchema {
				vocabs, err := loader.getMetaVocabs(sch, draft, vocabularies)
				if err != nil {
					return err
				}
				res.dialect = dialect{draft, vocabs}
			} else {
				res.dialect = fallback
			}
			r.resources[schPtr] = res
		}
	}

	// collect anchors into base resource
	for _, res := range r.resources {
		if res.id == base {
			// found base resource
			if err := r.collectAnchors(sch, schPtr, res); err != nil {
				return err
			}
			break
		}
	}

	// process subschemas
	subschemas := map[jsonPointer]any{}
	draft.subschemas.collect(obj, schPtr, subschemas)
	for ptr, v := range subschemas {
		if err := r.collectResources(loader, vocabularies, v, base, ptr, fallback); err != nil {
			return err
		}
	}

	return nil
}

func (r *root) addSubschema(loader *defaultLoader, vocabularies map[string]*Vocabulary, ptr jsonPointer) error {
	v, err := (&urlPtr{r.url, ptr}).lookup(r.doc)
	if err != nil {
		return err
	}
	base := r.resource(ptr)
	baseURL := base.id
	if err := r.collectResources(loader, vocabularies, v, baseURL, ptr, base.dialect); err != nil {
		return err
	}

	// collect anchors
	if _, ok := r.resources[ptr]; !ok {
		res := r.resource(ptr)
		if err := r.collectAnchors(v, ptr, res); err != nil {
			return err
		}
	}
	return nil
}

func (r *root) collectAnchors(sch any, schPtr jsonPointer, res *resource) error {
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
				string(anchor), r.url.String(), string(ptr1), string(schPtr),
			}
		}
		res.anchors[anchor] = schPtr
		return nil
	}

	if res.dialect.draft.version < 2019 {
		if _, ok := obj["$ref"]; ok {
			// All other properties in a "$ref" object MUST be ignored
			return nil
		}
		// anchor is specified in id
		if id, ok := strVal(obj, res.dialect.draft.id); ok {
			_, frag, err := splitFragment(id)
			if err != nil {
				loc := urlPtr{r.url, schPtr}
				return &ParseAnchorError{loc.String()}
			}
			if anchor, ok := frag.convert().(anchor); ok {
				if err := addAnchor(anchor); err != nil {
					return err
				}
			}
		}
	}
	if res.dialect.draft.version >= 2019 {
		if s, ok := strVal(obj, "$anchor"); ok {
			if err := addAnchor(anchor(s)); err != nil {
				return err
			}
		}
	}
	if res.dialect.draft.version >= 2020 {
		if s, ok := strVal(obj, "$dynamicAnchor"); ok {
			if err := addAnchor(anchor(s)); err != nil {
				return err
			}
			res.dynamicAnchors = append(res.dynamicAnchors, anchor(s))
		}
	}

	return nil
}

func (r *root) clone() *root {
	processed := map[jsonPointer]struct{}{}
	for k := range r.subschemasProcessed {
		processed[k] = struct{}{}
	}
	resources := map[jsonPointer]*resource{}
	for k, v := range r.resources {
		resources[k] = v.clone()
	}
	return &root{
		url:                 r.url,
		doc:                 r.doc,
		resources:           resources,
		subschemasProcessed: processed,
	}
}

// --

type resource struct {
	ptr            jsonPointer
	id             url
	dialect        dialect
	anchors        map[anchor]jsonPointer
	dynamicAnchors []anchor
}

func newResource(ptr jsonPointer, id url) *resource {
	return &resource{ptr: ptr, id: id, anchors: make(map[anchor]jsonPointer)}
}

func (res *resource) clone() *resource {
	anchors := map[anchor]jsonPointer{}
	for k, v := range res.anchors {
		anchors[k] = v
	}
	return &resource{
		ptr:            res.ptr,
		id:             res.id,
		dialect:        res.dialect,
		anchors:        anchors,
		dynamicAnchors: slices.Clone(res.dynamicAnchors),
	}
}

//--

type UnsupportedVocabularyError struct {
	URL        string
	Vocabulary string
}

func (e *UnsupportedVocabularyError) Error() string {
	return fmt.Sprintf("unsupported vocabulary %q in %q", e.Vocabulary, e.URL)
}

// --

type AnchorNotFoundError struct {
	URL       string
	Reference string
}

func (e *AnchorNotFoundError) Error() string {
	return fmt.Sprintf("anchor in %q not found in schema %q", e.Reference, e.URL)
}
