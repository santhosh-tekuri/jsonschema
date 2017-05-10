// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package jsonschema provides draft4 json-schema compilation and validation.

An implementation of JSON Schema, based on IETF's draft v4

An example of using this package:

	schemea, err := jsonschema.Compile("schemas/purchaseOrder.json")
	if err != nil {
		return err
	}
	if err = schema.Validate("purchaseOrder.json"); err != nil {
		return err
	}

This package supports loading json-schema from filePath and fileURL.

To load json-schema from HTTPURL, add following import:

	import _ "github.com/santhosh-tekuri/jsonschema/httploader"

Loading from urls for other schemes (such as ftp), can be plugged in. see package jsonschema/httploader
for an example

To load json-schema from in-memory:

	data := []byte(`{"type": "string"}`)
	url := "sch.json"
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, data); err != nil {
		return err
	}
	schemea, err := jsonschema.Compile(url)
	if err != nil {
		return err
	}
	if err = schema.Validate("doc.json"); err != nil {
		return err
	}

This package supports json string formats: date-time, hostname, email, ip-address, ipv4, ipv6, uri, uriref, regex.
Developers can define their own formats using package jsonschema/formats.

The ValidationError returned by Validate method contains detailed context to understand why and where the error is.

*/
package jsonschema
