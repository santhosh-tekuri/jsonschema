package jsonschema_test

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Define a schema for the new keyword itself. This schema will be used to
// validate appearance of the keyword in a schema before passing it to the
// compiler. This example defines the `powerOf` property of a JSON object in a
// schema as an integer greater than 0. This value will be used to ensure that
// numbers the schema applies to are powers of the value.
var powerOfMeta = jsonschema.MustCompileString("powerOf.json", `{
	"properties" : {
		"powerOf": {
			"type": "integer",
			"exclusiveMinimum": 0
		}
	}
}`)

// Define an objext to use for compiling. All the interesting logic lives in the
// Compile method.
type powerOfCompiler struct{}

// Compile is called after the metaschema has been applied to the appearance of
// the keyword, so we know the value is an integer greater than 0. It reads the
// value of the powerOf property from m, which is part of the schema definion.
// It then ensures that the schema has a valid value for the keyword. In our
// example, we need to ensure that the schema value is an int64, so we convert
// from a JSON number and return an error if it's not a valid value. If it is
// valid, return an ExtSchema value, here defined by powerOfSchema. This is the
// compiled representation of the keyword value from the schema, to be used for
// validation of documents. If the powerOf property does not appear in m, there
// is nothing to compile.
func (powerOfCompiler) Compile(ctx jsonschema.CompilerContext, m map[string]interface{}) (jsonschema.ExtSchema, error) {
	if pow, ok := m["powerOf"]; ok {
		n, err := pow.(json.Number).Int64()
		return powerOfSchema(n), err
	}

	// nothing to compile, return nil
	return nil, nil
}

// Define a type to represent the compiled form of the keyword parsed from a
// schema. In our case the value is just an int64 value, but more complicated
// keywords might require a more complex type like a struct.
type powerOfSchema int64

// Validate examines the value v to ensure that it is valid according to the
// schema s. In our case, we need to validate that the value from the document
// is a power of the powerOf value specified in the schema. So we convert the
// value to an int64 and return an error if it is not evenly divisible by the
// schema's powerOf value.
func (s powerOfSchema) Validate(ctx jsonschema.ValidationContext, v interface{}) error {
	switch v.(type) {
	case json.Number, float64, int, int32, int64:
		pow := int64(s)
		n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		for n%pow == 0 {
			n = n / pow
		}
		if n != 1 {
			return ctx.Error("powerOf", "%v not powerOf %v", v, pow)
		}
		return nil
	default:
		return nil
	}
}

func Example_extension() {
	// Create the compiler and register the powerOf Keyword. Use powerOfMeta to
	// validate its appearance in a schema.
	c := jsonschema.NewCompiler()
	c.RegisterExtension("powerOf", powerOfMeta, powerOfCompiler{})

	// Define a schema with the keyword and a JSON value to validate with that
	// schema.
	schema := `{"powerOf": 10}`
	instance := `100`

	// Compile the schema.
	if err := c.AddResource("schema.json", strings.NewReader(schema)); err != nil {
		log.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		log.Fatalf("%#v", err)
	}

	// Convert the instance JSON value. We use an interface{} to ensure that
	// valid JSON parses cleanly even if the value is not a number. Feel free
	// to use a more defined type if it suits your needs.
	var v interface{}
	if err := json.Unmarshal([]byte(instance), &v); err != nil {
		log.Fatal(err)
	}

	// Validate the JSON value against the schema! Since the schema defines a
	// powerOf 10 and the value is 100, it is valid and this method returns nil.
	if err = sch.Validate(v); err != nil {
		log.Fatalf("%#v", err)
	}
	// Output:
}
