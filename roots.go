package jsonschema

import (
	"fmt"
	"strings"
)

type roots struct {
	defaultDraft *Draft
	roots        map[url]*root
	loader       defaultLoader
	regexpEngine RegexpEngine
	vocabularies map[string]*Vocabulary
	assertVocabs bool
}

func newRoots() *roots {
	return &roots{
		defaultDraft: draftLatest,
		roots:        map[url]*root{},
		loader: defaultLoader{
			docs:   map[url]any{},
			loader: FileLoader{},
		},
		regexpEngine: goRegexpCompile,
		vocabularies: map[string]*Vocabulary{},
	}
}

func (rr *roots) orLoad(u url) (*root, error) {
	if r, ok := rr.roots[u]; ok {
		return r, nil
	}
	doc, err := rr.loader.load(u)
	if err != nil {
		return nil, err
	}
	return rr.addRoot(u, doc)
}

func (rr *roots) addRoot(u url, doc any) (*root, error) {
	r := &root{
		url:                 u,
		doc:                 doc,
		resources:           map[jsonPointer]*resource{},
		subschemasProcessed: map[jsonPointer]struct{}{},
	}
	if err := rr.collectResources(r, doc, u, "", dialect{rr.defaultDraft, nil}); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(u.String(), "http://json-schema.org/") &&
		!strings.HasPrefix(u.String(), "https://json-schema.org/") {
		if err := rr.validate(r, doc, ""); err != nil {
			return nil, err
		}
	}

	rr.roots[u] = r
	return r, nil
}

func (rr *roots) resolveFragment(uf urlFrag) (urlPtr, error) {
	r, err := rr.orLoad(uf.url)
	if err != nil {
		return urlPtr{}, err
	}
	return r.resolveFragment(uf.frag)
}

func (rr *roots) ensureSubschema(up urlPtr) error {
	r, err := rr.orLoad(up.url)
	if err != nil {
		return err
	}
	if _, ok := r.subschemasProcessed[up.ptr]; ok {
		return nil
	}
	v, err := up.lookup(r.doc)
	if err != nil {
		return err
	}
	rClone := r.clone()
	if err := rr.addSubschema(rClone, up.ptr); err != nil {
		return err
	}
	if err := rr.validate(rClone, v, up.ptr); err != nil {
		return err
	}
	rr.roots[r.url] = rClone
	return nil
}

func (rr *roots) validate(r *root, v any, ptr jsonPointer) error {
	dialect := r.resource(ptr).dialect
	meta := dialect.getSchema(rr.assertVocabs, rr.vocabularies)
	if err := meta.validate(v, rr.regexpEngine, meta, r.resources, rr.assertVocabs, rr.vocabularies); err != nil {
		up := urlPtr{r.url, ptr}
		return &SchemaValidationError{URL: up.String(), Err: err}
	}
	return nil
}

// --

type InvalidMetaSchemaURLError struct {
	URL string
	Err error
}

func (e *InvalidMetaSchemaURLError) Error() string {
	return fmt.Sprintf("invalid $schema in %q: %v", e.URL, e.Err)
}

// --

type UnsupportedDraftError struct {
	URL string
}

func (e *UnsupportedDraftError) Error() string {
	return fmt.Sprintf("draft %q is not supported", e.URL)
}

// --

type MetaSchemaCycleError struct {
	URL string
}

func (e *MetaSchemaCycleError) Error() string {
	return fmt.Sprintf("cycle in resolving $schema in %q", e.URL)
}

// --

type MetaSchemaMismatchError struct {
	URL string
}

func (e *MetaSchemaMismatchError) Error() string {
	return fmt.Sprintf("$schema in %q does not match with $schema in root", e.URL)
}
