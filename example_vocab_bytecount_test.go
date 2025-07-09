package jsonschema_test

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/message"
)

// SchemaExt --

type ByteCount struct {
	maxBytes *int
	minBytes *int
}

func (s *ByteCount) Validate(ctx *jsonschema.ValidatorContext, v any) {
	str, ok := v.(string)
	if !ok {
		return
	}
	if s.maxBytes != nil && len(str) > *s.maxBytes {
		ctx.AddError(&MaxBytes{Got: len(str), Want: *s.maxBytes})
	}
	if s.minBytes != nil && len(str) < *s.minBytes {
		ctx.AddError(&MinBytes{Got: len(str), Want: *s.maxBytes})
	}
}

// Vocab --

func ByteCountVocab() *jsonschema.Vocabulary {
	url := "http://example.com/meta/byte-count"
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"properties": {
			"maxBytes": { "type": "integer", "minimum": 0 },
			"minBytes": { "type": "integer", "minimum": 0 }
		}
	}`))
	if err != nil {
		log.Fatal(err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, schema); err != nil {
		log.Fatal(err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		log.Fatal(err)
	}

	return &jsonschema.Vocabulary{
		URL:     url,
		Schema:  sch,
		Compile: compileByteCount,
	}
}

func numValue(obj map[string]any, prop string) *int {
	v, ok := obj[prop]
	if !ok {
		return nil
	}
	switch v.(type) {
	case json.Number, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if n, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
			return &n
		}
	}
	return nil
}

func compileByteCount(ctx *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
	max := numValue(obj, "maxBytes")
	min := numValue(obj, "minBytes")
	if max == nil && min == nil {
		return nil, nil
	}
	return &ByteCount{maxBytes: max, minBytes: min}, nil
}

// ErrorKind --

type MaxBytes struct {
	Got, Want int
}

func (*MaxBytes) KeywordPath() []string {
	return []string{"maxBytes"}
}

func (k *MaxBytes) LocalizedString(p *message.Printer) string {
	return p.Sprintf("maxBytes: got %d, want %d", k.Got, k.Want)
}

type MinBytes struct {
	Got, Want int
}

func (*MinBytes) KeywordPath() []string {
	return []string{"minBytes"}
}

func (k *MinBytes) LocalizedString(p *message.Printer) string {
	return p.Sprintf("minBytes: got %d, want %d", k.Got, k.Want)
}

// Example --

func Example_vocab_bytecount() {
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"properties": {
			"name": {
				"type": "string",
				"maxBytes": 5
			}
		}
	}`))
	if err != nil {
		log.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`
		{ "name": "helloworld" }
	`))
	if err != nil {
		log.Fatal(err)
	}

	c := jsonschema.NewCompiler()
	c.AssertVocabs()
	c.RegisterVocabulary(ByteCountVocab())
	if err := c.AddResource("schema.json", schema); err != nil {
		log.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		log.Fatal(err)
	}

	err = sch.Validate(inst)
	fmt.Println("valid:", err == nil)
	// Output:
	// valid: false
}
