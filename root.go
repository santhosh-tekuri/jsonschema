package jsonschema

import (
	"fmt"
	"slices"
	"strings"
)

type root struct {
	url        url
	doc        any
	draft      *Draft
	resources  map[jsonPointer]*resource
	metaVocabs []string // nil means use draft
}

func (r *root) hasVocab(name string) bool {
	if name == "core" || r.draft.version < 2019 {
		return true
	}
	if r.metaVocabs != nil {
		return slices.Contains(r.metaVocabs, name)
	}
	return slices.Contains(r.draft.defaultVocabs, name)
}

func (r *root) getReqdVocabs() ([]string, error) {
	if r.draft.version < 2019 {
		return nil, nil
	}
	obj, ok := r.doc.(map[string]any)
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
		name, ok := strings.CutPrefix(vocab, r.draft.vocabPrefix)
		if !ok {
			return nil, &UnsupportedVocabularyError{r.url.String(), vocab}
		}
		if !slices.Contains(vocabs, name) {
			vocabs = append(vocabs, name)
		}
	}
	return vocabs, nil
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

func (r *root) baseURL(ptr jsonPointer) url {
	return r.resource(ptr).id
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

func (r *root) addSubschema(ptr jsonPointer) error {
	v, err := (&urlPtr{r.url, ptr}).lookup(r.doc)
	if err != nil {
		return err
	}
	baseURL := r.baseURL(ptr)
	if err := r.draft.collectResources(v, baseURL, ptr, r.url, r.resources); err != nil {
		return err
	}

	// collect anchors
	if _, ok := r.resources[ptr]; !ok {
		res := r.resource(ptr)
		if err := r.draft.collectAnchors(v, ptr, res, r.url); err != nil {
			return err
		}
	}
	return nil
}

func (r *root) validate(ptr jsonPointer, v any, regexpEngine RegexpEngine) error {
	up := urlPtr{r.url, ptr}
	if r.metaVocabs == nil {
		return r.draft.validate(up, v, regexpEngine)
	}

	// validate only with the vocabs listed in metaschema
	if err := r.draft.allVocabs["core"].validate(v, regexpEngine); err != nil {
		return &SchemaValidationError{URL: up.String(), Err: err}
	}
	for _, vocab := range r.metaVocabs {
		if err := r.draft.allVocabs[vocab].validate(v, regexpEngine); err != nil {
			return &SchemaValidationError{URL: up.String(), Err: err}
		}
	}
	return nil
}

// --

type resource struct {
	ptr            jsonPointer
	id             url
	anchors        map[anchor]jsonPointer
	dynamicAnchors []anchor
}

func newResource(ptr jsonPointer, id url) *resource {
	return &resource{ptr: ptr, id: id, anchors: make(map[anchor]jsonPointer)}
}

//--

type meta struct {
	draft  *Draft
	vocabs []string
}

// --

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
