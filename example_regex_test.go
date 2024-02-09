package jsonschema_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var schema = `{
	"type": "string",
	"pattern": "^\\cc$"
}`
var instance = `"\u0003"`

func Example_userDefinedRegex() {
	compiler := jsonschema.NewCompiler()
	compiler.CompileRegex = func(s string) (jsonschema.Regexp, error) {
		re, err := regexp2.Compile(s, regexp2.ECMAScript)
		if err != nil {
			return nil, err
		}
		return ecmaRegex{re}, nil
	}

	if err := compiler.AddResource("schema.json", strings.NewReader(schema)); err != nil {
		panic(err)
	}
	sch, err := compiler.Compile("schema.json")
	if err != nil {
		panic(err)
	}

	var inst interface{}
	if err := json.Unmarshal([]byte(instance), &inst); err != nil {
		panic(err)
	}
	if err := sch.Validate(inst); err != nil {
		fmt.Println(err)
	}
	// Output:
}

type ecmaRegex struct {
	re *regexp2.Regexp
}

func (re ecmaRegex) MatchString(s string) bool {
	matched, err := re.re.MatchString(s)
	return err == nil && matched
}

func (re ecmaRegex) String() string {
	return re.re.String()
}
