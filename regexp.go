package jsonschema

import "regexp"

// Regexp is an interface for working with regular expressions.
type Regexp interface {
	MustCompile(expr string)
	Compile(expr string) error
	MatchString(s string) bool
	String() string
}

// RegexpProvider greates a new unitialized regular expression.
type RegexpProvider func() Regexp

var newRegexp RegexpProvider = func() Regexp {
	return &defaultRegexp{}
}

// SetRegexpProvider can be called to change the default regular expression provider.
//
// By default, the standard library regexp package is used.
func SetRegexpProvider(f RegexpProvider) {
	newRegexp = f
}

type defaultRegexp struct {
	re *regexp.Regexp
}

var _ Regexp = (*defaultRegexp)(nil)

func (r *defaultRegexp) MustCompile(expr string) {
	r.re = regexp.MustCompile(expr)
}

func (r *defaultRegexp) Compile(expr string) error {
	re, err := regexp.Compile(expr)
	r.re = re
	return err
}

func (r *defaultRegexp) MatchString(s string) bool {
	return r.re.MatchString(s)
}

func (r *defaultRegexp) String() string {
	return r.re.String()
}
