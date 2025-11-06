package jsonschema

import (
	"fmt"
	"hash/maphash"
	"math/big"
	gourl "net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/message"
)

// --

type url (string)

func (u url) String() string {
	return string(u)
}

func (u url) join(ref string) (*urlFrag, error) {
	base, err := gourl.Parse(string(u))
	if err != nil {
		return nil, &ParseURLError{URL: u.String(), Err: err}
	}

	ref, frag, err := splitFragment(ref)
	if err != nil {
		return nil, err
	}
	refURL, err := gourl.Parse(ref)
	if err != nil {
		return nil, &ParseURLError{URL: ref, Err: err}
	}
	resolved := base.ResolveReference(refURL)

	// see https://github.com/golang/go/issues/66084 (net/url: ResolveReference ignores Opaque value)
	if !refURL.IsAbs() && base.Opaque != "" {
		resolved.Opaque = base.Opaque
	}

	return &urlFrag{url: url(resolved.String()), frag: frag}, nil
}

// --

type jsonPointer string

func escape(tok string) string {
	tok = strings.ReplaceAll(tok, "~", "~0")
	tok = strings.ReplaceAll(tok, "/", "~1")
	return tok
}

func unescape(tok string) (string, bool) {
	tilde := strings.IndexByte(tok, '~')
	if tilde == -1 {
		return tok, true
	}
	sb := new(strings.Builder)
	for {
		sb.WriteString(tok[:tilde])
		tok = tok[tilde+1:]
		if tok == "" {
			return "", false
		}
		switch tok[0] {
		case '0':
			sb.WriteByte('~')
		case '1':
			sb.WriteByte('/')
		default:
			return "", false
		}
		tok = tok[1:]
		tilde = strings.IndexByte(tok, '~')
		if tilde == -1 {
			sb.WriteString(tok)
			break
		}
	}
	return sb.String(), true
}

func (ptr jsonPointer) isEmpty() bool {
	return string(ptr) == ""
}

func (ptr jsonPointer) concat(next jsonPointer) jsonPointer {
	return jsonPointer(fmt.Sprintf("%s%s", ptr, next))
}

func (ptr jsonPointer) append(tok string) jsonPointer {
	return jsonPointer(fmt.Sprintf("%s/%s", ptr, escape(tok)))
}

func (ptr jsonPointer) append2(tok1, tok2 string) jsonPointer {
	return jsonPointer(fmt.Sprintf("%s/%s/%s", ptr, escape(tok1), escape(tok2)))
}

// --

type anchor string

// --

type fragment string

func decode(frag string) (string, error) {
	return gourl.PathUnescape(frag)
}

// avoids escaping /.
func encode(frag string) string {
	var sb strings.Builder
	for i, tok := range strings.Split(frag, "/") {
		if i > 0 {
			sb.WriteByte('/')
		}
		sb.WriteString(gourl.PathEscape(tok))
	}
	return sb.String()
}

func splitFragment(str string) (string, fragment, error) {
	u, f := split(str)
	f, err := decode(f)
	if err != nil {
		return "", fragment(""), &ParseURLError{URL: str, Err: err}
	}
	return u, fragment(f), nil
}

func split(str string) (string, string) {
	hash := strings.IndexByte(str, '#')
	if hash == -1 {
		return str, ""
	}
	return str[:hash], str[hash+1:]
}

func (frag fragment) convert() any {
	str := string(frag)
	if str == "" || strings.HasPrefix(str, "/") {
		return jsonPointer(str)
	}
	return anchor(str)
}

// --

type urlFrag struct {
	url  url
	frag fragment
}

func startsWithWindowsDrive(s string) bool {
	if s != "" && strings.HasPrefix(s[1:], `:\`) {
		return (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')
	}
	return false
}

func absolute(input string) (*urlFrag, error) {
	u, frag, err := splitFragment(input)
	if err != nil {
		return nil, err
	}

	// if windows absolute file path, convert to file url
	// because: net/url parses driver name as scheme
	if runtime.GOOS == "windows" && startsWithWindowsDrive(u) {
		u = "file:///" + filepath.ToSlash(u)
	}

	gourl, err := gourl.Parse(u)
	if err != nil {
		return nil, &ParseURLError{URL: input, Err: err}
	}
	if gourl.IsAbs() {
		return &urlFrag{url(u), frag}, nil
	}

	// avoid filesystem api in wasm
	if runtime.GOOS != "js" {
		abs, err := filepath.Abs(u)
		if err != nil {
			return nil, &ParseURLError{URL: input, Err: err}
		}
		u = abs
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	u = "file://" + filepath.ToSlash(u)

	_, err = gourl.Parse(u)
	if err != nil {
		return nil, &ParseURLError{URL: input, Err: err}
	}
	return &urlFrag{url: url(u), frag: frag}, nil
}

func (uf *urlFrag) String() string {
	return fmt.Sprintf("%s#%s", uf.url, encode(string(uf.frag)))
}

// --

type urlPtr struct {
	url url
	ptr jsonPointer
}

func (up *urlPtr) lookup(v any) (any, error) {
	for _, tok := range strings.Split(string(up.ptr), "/")[1:] {
		tok, ok := unescape(tok)
		if !ok {
			return nil, &InvalidJsonPointerError{up.String()}
		}
		switch val := v.(type) {
		case map[string]any:
			if pvalue, ok := val[tok]; ok {
				v = pvalue
				continue
			}
		case []any:
			if index, err := strconv.Atoi(tok); err == nil {
				if index >= 0 && index < len(val) {
					v = val[index]
					continue
				}
			}
		}
		return nil, &JSONPointerNotFoundError{up.String()}
	}
	return v, nil
}

func (up *urlPtr) format(tok string) string {
	return fmt.Sprintf("%s#%s/%s", up.url, encode(string(up.ptr)), encode(escape(tok)))
}

func (up *urlPtr) String() string {
	return fmt.Sprintf("%s#%s", up.url, encode(string(up.ptr)))
}

// --

func minInt(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func strVal(obj map[string]any, prop string) (string, bool) {
	v, ok := obj[prop]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func isInteger(num any) bool {
	rat, ok := new(big.Rat).SetString(fmt.Sprint(num))
	return ok && rat.IsInt()
}

// quote returns single-quoted string.
// used for embedding quoted strings in json.
func quote(s string) string {
	s = fmt.Sprintf("%q", s)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s[1:len(s)-1] + "'"
}

func equals(v1, v2 any) (bool, ErrorKind) {
	t1 := typeOf(v1)
	t2 := typeOf(v2)

	if t1 != t2 {
		return false, nil
	}

	switch t1 {
	case objectType:
		rv1 := reflect.ValueOf(v1)
		rv2 := reflect.ValueOf(v2)
		if rv1.Len() != rv2.Len() {
			return false, nil
		}
		iter := rv1.MapRange()
		for iter.Next() {
			k := iter.Key()
			val1 := iter.Value().Interface()
			val2Reflect := rv2.MapIndex(k)
			if !val2Reflect.IsValid() {
				return false, nil
			}
			val2 := val2Reflect.Interface()
			if ok, errKind := equals(val1, val2); !ok || errKind != nil {
				return ok, errKind
			}
		}
		return true, nil
	case arrayType:
		rv1 := reflect.ValueOf(v1)
		rv2 := reflect.ValueOf(v2)
		if rv1.Len() != rv2.Len() {
			return false, nil
		}
		for i := 0; i < rv1.Len(); i++ {
			if ok, errKind := equals(rv1.Index(i).Interface(), rv2.Index(i).Interface()); !ok || errKind != nil {
				return ok, errKind
			}
		}
		return true, nil
	case nullType:
		return true, nil
	case booleanType:
		return reflect.ValueOf(v1).Bool() == reflect.ValueOf(v2).Bool(), nil
	case stringType:
		return reflect.ValueOf(v1).String() == reflect.ValueOf(v2).String(), nil
	case numberType:
		num1, ok1 := new(big.Rat).SetString(fmt.Sprint(v1))
		num2, ok2 := new(big.Rat).SetString(fmt.Sprint(v2))
		return ok1 && ok2 && num1.Cmp(num2) == 0, nil
	default:
		return false, &kind.InvalidJsonValue{Value: v1}
	}
}

func duplicates(arrVal reflect.Value) (int, int, ErrorKind) {
	arrLen := arrVal.Len()
	if arrLen <= 20 {
		for i := 1; i < arrLen; i++ {
			for j := 0; j < i; j++ {
				if ok, k := equals(arrVal.Index(i).Interface(), arrVal.Index(j).Interface()); ok || k != nil {
					return j, i, k
				}
			}
		}
		return -1, -1, nil
	}

	m := make(map[uint64][]int)
	h := new(maphash.Hash)
	for i := 0; i < arrLen; i++ {
		item := arrVal.Index(i).Interface()
		h.Reset()
		writeHash(item, h)
		hash := h.Sum64()
		indexes, ok := m[hash]
		if ok {
			for _, j := range indexes {
				if ok, k := equals(item, arrVal.Index(j).Interface()); ok || k != nil {
					return j, i, k
				}
			}
		}
		indexes = append(indexes, i)
		m[hash] = indexes
	}
	return -1, -1, nil
}

func writeHash(v any, h *maphash.Hash) ErrorKind {
	t := typeOf(v)
	switch t {
	case objectType:
		_ = h.WriteByte(0)
		rv := reflect.ValueOf(v)
		// Collect all keys and sort them as strings
		keys := make([]string, 0, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			keys = append(keys, fmt.Sprint(iter.Key().Interface()))
		}
		slices.Sort(keys)
		for _, keyStr := range keys {
			writeHash(keyStr, h)
			// Find the value for this key
			iter2 := rv.MapRange()
			for iter2.Next() {
				if fmt.Sprint(iter2.Key().Interface()) == keyStr {
					writeHash(iter2.Value().Interface(), h)
					break
				}
			}
		}
	case arrayType:
		_ = h.WriteByte(1)
		rv := reflect.ValueOf(v)
		for i := 0; i < rv.Len(); i++ {
			writeHash(rv.Index(i).Interface(), h)
		}
	case nullType:
		_ = h.WriteByte(2)
	case booleanType:
		_ = h.WriteByte(3)
		if reflect.ValueOf(v).Bool() {
			_ = h.WriteByte(1)
		} else {
			_ = h.WriteByte(0)
		}
	case stringType:
		_ = h.WriteByte(4)
		_, _ = h.WriteString(reflect.ValueOf(v).String())
	case numberType:
		_ = h.WriteByte(5)
		num, _ := new(big.Rat).SetString(fmt.Sprint(v))
		_, _ = h.Write(num.Num().Bytes())
		_, _ = h.Write(num.Denom().Bytes())
	default:
		return &kind.InvalidJsonValue{Value: v}
	}
	return nil
}

// --

type ParseURLError struct {
	URL string
	Err error
}

func (e *ParseURLError) Error() string {
	return fmt.Sprintf("error in parsing %q: %v", e.URL, e.Err)
}

// --

type InvalidJsonPointerError struct {
	URL string
}

func (e *InvalidJsonPointerError) Error() string {
	return fmt.Sprintf("invalid json-pointer %q", e.URL)
}

// --

type JSONPointerNotFoundError struct {
	URL string
}

func (e *JSONPointerNotFoundError) Error() string {
	return fmt.Sprintf("json-pointer in %q not found", e.URL)
}

// --

type SchemaValidationError struct {
	URL string
	Err error
}

func (e *SchemaValidationError) Error() string {
	return fmt.Sprintf("%q is not valid against metaschema: %v", e.URL, e.Err)
}

// --

// LocalizableError is an error whose message is localizable.
func LocalizableError(format string, args ...any) error {
	return &localizableError{format, args}
}

type localizableError struct {
	msg  string
	args []any
}

func (e *localizableError) Error() string {
	return fmt.Sprintf(e.msg, e.args...)
}

func (e *localizableError) LocalizedError(p *message.Printer) string {
	return p.Sprintf(e.msg, e.args...)
}
