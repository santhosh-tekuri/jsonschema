package jsonschema

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	gourl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// URLLoader knows how to load json from given url.
type URLLoader interface {
	// Load loads json from given absolute url.
	Load(url string) (any, error)
}

// --

type FileLoader struct{}

func (l FileLoader) Load(url string) (any, error) {
	path, err := l.ToFile(url)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return UnmarshalJSON(f)
}

func (l FileLoader) ToFile(url string) (string, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("invalid file url: %s", u)
	}
	path := u.Path
	if runtime.GOOS == "windows" {
		path = strings.TrimPrefix(path, "/")
		path = filepath.FromSlash(path)
	}
	return path, nil
}

// --

// SchemeURLLoader delegates to other [URLLoaders]
// based on url scheme.
type SchemeURLLoader map[string]URLLoader

func (l SchemeURLLoader) Load(url string) (any, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return nil, err
	}
	ll, ok := l[u.Scheme]
	if !ok {
		return nil, &UnsupportedURLSchemeError{u.String()}
	}
	return ll.Load(url)
}

// --

//go:embed metaschemas
var metaFS embed.FS

func loadMeta(url string) (any, error) {
	u, meta := strings.CutPrefix(url, "http://json-schema.org/")
	if !meta {
		u, meta = strings.CutPrefix(url, "https://json-schema.org/")
	}
	if meta {
		if u == "schema" {
			return loadMeta(draftLatest.url)
		}
		f, err := metaFS.Open("metaschemas/" + u)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, nil
			}
			return nil, err
		}
		return UnmarshalJSON(f)
	}
	return nil, nil
}

// --

type LoadURLError struct {
	URL string
	Err error
}

func (e *LoadURLError) Error() string {
	return fmt.Sprintf("failing loading %q: %v", e.URL, e.Err)
}

// --

type UnsupportedURLSchemeError struct {
	url string
}

func (e *UnsupportedURLSchemeError) Error() string {
	return fmt.Sprintf("no URLLoader registered for %q", e.url)
}

// --

// UnmarshalJSON unmarshals into [any] without losing
// number precision using [json.Number].
func UnmarshalJSON(r io.Reader) (any, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err == nil || err != io.EOF {
		return nil, fmt.Errorf("invalid character after top-level value")
	}
	return doc, nil
}
