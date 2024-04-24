package jsonschema

import (
	"fmt"
	gourl "net/url"
	"strings"
)

type roots struct {
	defaultDraft *Draft
	roots        map[url]*root
	loader       defaultLoader
	regexpEngine RegexpEngine
	vocabularies map[string]*Vocabulary
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
	return rr.addRoot(u, doc, make(map[url]struct{}))
}

func (rr *roots) getMeta(up urlPtr, doc any, cycle map[url]struct{}) (meta, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return meta{rr.defaultDraft, nil}, nil
	}
	sch, ok := strVal(obj, "$schema")
	if !ok {
		return meta{rr.defaultDraft, nil}, nil
	}
	if draft := draftFromURL(sch); draft != nil {
		return meta{draft, nil}, nil
	}
	sch, _ = split(sch)
	if _, err := gourl.Parse(sch); err != nil {
		return meta{}, &InvalidMetaSchemaURLError{up.String(), err}
	}
	schUrl := url(sch)
	if r, ok := rr.roots[schUrl]; ok {
		vocabs, err := r.getReqdVocabs()
		return meta{r.draft, vocabs}, err
	}
	if schUrl == up.url {
		return meta{}, &UnsupportedDraftError{schUrl.String()}
	}
	if _, ok := cycle[schUrl]; ok {
		return meta{}, &MetaSchemaCycleError{schUrl.String()}
	}
	cycle[schUrl] = struct{}{}
	doc, err := rr.loader.load(schUrl)
	if err != nil {
		return meta{}, err
	}
	r, err := rr.addRoot(schUrl, doc, cycle)
	if err != nil {
		return meta{}, err
	}
	vocabs, err := r.getReqdVocabs()
	return meta{r.draft, vocabs}, err
}

func (rr *roots) addRoot(u url, doc any, cycle map[url]struct{}) (*root, error) {
	meta, err := rr.getMeta(urlPtr{u, ""}, doc, cycle)
	if err != nil {
		return nil, err
	}
	r := &root{
		url:                 u,
		doc:                 doc,
		draft:               meta.draft,
		resources:           map[jsonPointer]*resource{},
		metaVocabs:          meta.vocabs,
		subschemasProcessed: map[jsonPointer]struct{}{},
	}
	if err := r.collectResources(doc, u, ""); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(u.String(), "http://json-schema.org/") &&
		!strings.HasPrefix(u.String(), "https://json-schema.org/") {
		if err := r.validate("", doc, rr.regexpEngine); err != nil {
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
	if err := r.validate(up.ptr, v, rr.regexpEngine); err != nil {
		return err
	}
	return r.addSubschema(up.ptr)
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
