// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
	_ "github.com/santhosh-tekuri/jsonschema/httploader"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "args: <json-schema> <json-file>")
		os.Exit(1)
	}

	schema, err := jsonschema.Compile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "jsonschema is invalid. reason:")
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	doc, err := ioutil.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "json-file is invalid. reason:")
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	err = schema.Validate(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "json-file does not conform to the schema specified. reason:")
		fmt.Fprintln(os.Stderr, err)
		return
	}
}
