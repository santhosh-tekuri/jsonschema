# jsonschema v4.0.0

[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)
[![GoDoc](https://godoc.org/github.com/santhosh-tekuri/jsonschema?status.svg)](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4)
[![Go Report Card](https://goreportcard.com/badge/github.com/santhosh-tekuri/jsonschema)](https://goreportcard.com/report/github.com/santhosh-tekuri/jsonschema)
[![Build Status](https://github.com/santhosh-tekuri/jsonschema/actions/workflows/go.yaml/badge.svg)](https://github.com/santhosh-tekuri/jsonschema/actions/workflows/go.yaml)
[![codecov.io](https://codecov.io/github/santhosh-tekuri/jsonschema/coverage.svg?branch=master)](https://codecov.io/github/santhosh-tekuri/jsonschema?branch=master)

Package jsonschema provides json-schema compilation and validation.

### Features:
 - implements
   [draft 2020-12](https://json-schema.org/specification-links.html#2020-12),
   [draft 2019-09](https://json-schema.org/specification-links.html#draft-2019-09-formerly-known-as-draft-8),
   [draft-7](https://json-schema.org/specification-links.html#draft-7),
   [draft-6](https://json-schema.org/specification-links.html#draft-6),
   [draft-4](https://json-schema.org/specification-links.html#draft-4)
 - fully compliant with [JSON-Schema-Test-Suite](https://github.com/json-schema-org/JSON-Schema-Test-Suite), (excluding some optional)
   - list of optioanl tests that are excluded can be found in schema_test.go(variable [skipTests](https://github.com/santhosh-tekuri/jsonschema/blob/master/schema_test.go#L30))
 - validates schemas against meta-schema
 - full support of remote references
 - support of recursive references between schemas
 - detects infinite loop in schemas
 - thread safe validation
 - rich, intutive hierarchial error messages with json-pointers to exact location
 - supports enabling format and content Assertions in draft2019-09
   - make Compiler.AssertFormat, Compiler.AssertContent true
 - compiled schema can be introspected. easier to develop tools like generating go structs given schema
 - supports user-defined keywords via [extensions](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-Extension)
 - implements following formats (supports [user-defined](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-UserDefinedFormat))
   - date-time, date, time, duration
   - uuid, hostname, email
   - ip-address, ipv4, ipv6
   - uri, uriref, uri-template(limited validation)
   - json-pointer, relative-json-pointer
   - regex, format
 - implements following contentEncoding (supports [user-defined](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-UserDefinedContent))
   - base64
 - implements following contentMediaType (supports [user-defined](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-UserDefinedContent))
   - application/json
 - can load from files/http/https/[string](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-FromString)/[]byte/io.Reader (suports [user-defined](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4/#example-package-UserDefinedLoader))


see examples in [godoc](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v4)

The schema is compiled against the version specified in `$schema` property.
If `$schema` property is missing, it uses latest draft which currently is draft7.
You can force to use specific version, when `$schema` is missing, as follows:

```go
compiler := jsonschema.NewCompiler()
compler.Draft = jsonschema.Draft4
```

you can also validate go value using `schema.ValidateInterface(interface{})` method.  
but the argument should not be user-defined struct.

This package supports loading json-schema from filePath and fileURL.

To load json-schema from HTTPURL, add following import:

```go
import _ "github.com/santhosh-tekuri/jsonschema/v4/httploader"
```

## Rich Errors

The ValidationError returned by Validate method contains detailed context to understand why and where the error is.

schema.json:
```json
{
      "$ref": "t.json#/definitions/employee"
}
```

t.json:
```json
{
    "definitions": {
        "employee": {
            "type": "string"
        }
    }
}
```

doc.json:
```json
1
```

Validating `doc.json` with `schema.json`, gives following ValidationError:
```
I[#] S[#] doesn't validate with "schema.json#"
  I[#] S[#/$ref] doesn't valide with "t.json#/definitions/employee"
    I[#] S[#/definitions/employee/type] expected string, but got number
```

Here `I` stands for instance document and `S` stands for schema document.  
The json-fragments that caused error in instance and schema documents are represented using json-pointer notation.  
Nested causes are printed with indent.

## CLI

```bash
jv [-draft INT] <schema-file> [<json-doc>]...
```

if no `<json-doc>` arguments are passed, it simply validates the `<schema-file>`.  
if `$schema` attribute is missing in schema, it uses draft7. this can be overriden by passing `-draft` flag

exit-code is 1, if there are any validation errors

## Validating YAML Document

since yaml supports non-string keys, such yaml documents are rendered as invalid json documents.  
yaml parser returns `map[interface{}]interface{}` for object, whereas json parser returns `map[string]interafce{}`.  
this package accepts only `map[string]interface{}`, so we need to manually convert them to `map[string]interface{}`

https://play.golang.org/p/sJy1qY7dXgA

the above example shows how to validate yaml document with jsonschema.  
the convertion explained above is implemented by `toStringKeys` function

