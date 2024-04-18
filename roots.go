package jsonschema

import (
	"fmt"
	gourl "net/url"
	"strings"
)

type roots struct {
	defaultDraft  *Draft
	roots         map[url]*root
	userResources map[url]any
	loader        URLLoader
	regexpEngine  RegexpEngine
}

func newRoots() *roots {
	return &roots{
		defaultDraft:  draftLatest,
		roots:         map[url]*root{},
		userResources: map[url]any{},
		loader:        FileLoader{},
		regexpEngine:  goRegexpCompile,
	}
}

func (rr *roots) loadURL(url url) (any, error) {
	v, err := loadMeta(url.String())
	if err != nil {
		return nil, err
	}
	if v != nil {
		return v, nil
	}
	if v, ok := rr.userResources[url]; ok {
		return v, nil
	}
	v, err = rr.loader.Load(url.String())
	if err != nil {
		return nil, &LoadURLError{URL: url.String(), Err: err}
	}
	return v, nil
}

func (rr *roots) orLoad(u url) (*root, error) {
	if r, ok := rr.roots[u]; ok {
		return r, nil
	}
	doc, err := rr.loadURL(u)
	if err != nil {
		return nil, err
	}
	return rr.addRoot(u, doc, make(map[url]struct{}))
}

func (rr *roots) addRoot(u url, doc any, cycle map[url]struct{}) (*root, error) {
	draft, vocabs, err := func() (*Draft, []string, error) {
		obj, ok := doc.(map[string]any)
		if !ok {
			return rr.defaultDraft, nil, nil
		}
		sch, ok := strVal(obj, "$schema")
		if !ok {
			return rr.defaultDraft, nil, nil
		}
		if draft := draftFromURL(sch); draft != nil {
			return draft, nil, nil
		}
		sch, _ = split(sch)
		if _, err := gourl.Parse(sch); err != nil {
			return nil, nil, &InvalidMetaSchemaURLError{u.String(), err}
		}
		schUrl := url(sch)
		if r, ok := rr.roots[schUrl]; ok {
			vocabs, err := r.getReqdVocabs()
			return r.draft, vocabs, err
		}
		if schUrl == u {
			return nil, nil, &UnsupportedDraftError{schUrl.String()}
		}
		if _, ok := cycle[schUrl]; ok {
			return nil, nil, &MetaSchemaCycleError{u.String()}
		}
		cycle[schUrl] = struct{}{}
		doc, err := rr.loadURL(schUrl)
		if err != nil {
			return nil, nil, err
		}
		r, err := rr.addRoot(schUrl, doc, cycle)
		if err != nil {
			return nil, nil, err
		}
		vocabs, err := r.getReqdVocabs()
		return r.draft, vocabs, err
	}()
	if err != nil {
		return nil, err
	}

	resources := map[jsonPointer]*resource{}
	if err := draft.collectResources(doc, u, "", u, resources); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(u.String(), "http://json-schema.org/") &&
		!strings.HasPrefix(u.String(), "https://json-schema.org/") {
		if err := draft.validate(urlPtr{u, ""}, doc, rr.regexpEngine); err != nil {
			return nil, err
		}
	}

	r := &root{
		url:        u,
		doc:        doc,
		draft:      draft,
		resources:  resources,
		metaVocabs: vocabs,
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
	if r.draft.isSubschema(string(up.ptr)) {
		return nil
	}
	v, err := up.lookup(r.doc)
	if err != nil {
		return err
	}
	if err := r.draft.validate(up, v, rr.regexpEngine); err != nil {
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
