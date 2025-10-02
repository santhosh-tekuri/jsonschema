package jsonschema_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaExt --

type openapiDiscriminator struct {
	pname   string
	mapping map[string]*jsonschema.Schema
}

func (d *openapiDiscriminator) Validate(ctx *jsonschema.ValidatorContext, v any) {
	obj, ok := v.(map[string]any)
	if !ok {
		return
	}
	pvalue, ok := obj[d.pname]
	if !ok {
		return
	}
	value, ok := pvalue.(string)
	if !ok {
		return
	}
	sch := d.mapping[value]
	if sch == nil {
		return
	}
	if err := ctx.Validate(sch, v, nil); err != nil {
		ctx.AddErr(err)
	} else {
		ctx.EvaluatedProp(d.pname)
	}
}

// Vocab --

func openapiDiscriminatorVocab() *jsonschema.Vocabulary {
	url := "http://example.com/meta/openapi/discriminator"
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"properties" : {
			"discriminator": {
				"type": "object",
		        "required": ["propertyName", "mapping"],
				"properties": {
					"propertyName": { "type": "string" },
					"mapping": {
		            	"type": "object",
						"patternProperties": {
							".*": { "type": "string" }
						}
		            }
				}
			}
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
		URL:    url,
		Schema: sch,
		Subschemas: []jsonschema.SchemaPath{
			{jsonschema.Prop("discriminator"), jsonschema.AllProp{}, jsonschema.AllProp{}},
		},
		Compile: compileOpenapiDiscriminator,
	}
}

func compileOpenapiDiscriminator(ctx *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
	v, ok := obj["discriminator"]
	if !ok {
		return nil, nil
	}
	d, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	pname := d["propertyName"].(string)
	mapping := map[string]*jsonschema.Schema{}
	for val, ref := range d["mapping"].(map[string]any) {
		sch, err := ctx.EnqueueRef(ref.(string))
		if err != nil {
			return nil, err
		}
		mapping[val] = sch
	}
	return &openapiDiscriminator{pname, mapping}, nil
}

// Example --

func Example_vocab_openapiDiscriminator() {
	// if kind is fish, swimmingSpeed is required
	// if kind is dog, runningSpeed is required
	openapiDoc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"components": {
			"schemas": {
				"dog": {
					"type": "object",
					"properties": {
						"runningSpeed": { "type": "number" }
					},
					"required": ["runningSpeed"]
				},
				"fish": {
					"type": "object",
					"properties": {
						"swimmingSpeed": { "type": "number" }
					},
					"required": ["swimmingSpeed"]
				},
				"Pet": {
					"type": "object",
					"properties": {
						"kind": { "type": "string" }
					},
					"required": ["kind"],
					"discriminator": {
						"propertyName": "kind",
						"mapping": {
							"fish": "#/components/schemas/fish",
							"dog": "#/components/schemas/dog"
						}
					}
				}
			}
		}
	}`))
	if err != nil {
		log.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`{
		"kind": "fish",
		"runningSpeed": 5
	}`))
	if err != nil {
		log.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertVocabs()
	c.RegisterVocabulary(openapiDiscriminatorVocab())
	if err := c.AddResource("openapidoc.json", openapiDoc); err != nil {
		log.Fatal(err)
	}
	sch, err := c.Compile("openapidoc.json#/components/schemas/Pet")
	if err != nil {
		log.Fatal(err)
	}

	err = sch.Validate(inst)
	fmt.Println("valid:", err == nil)
	// Output:
	// valid: false
}
