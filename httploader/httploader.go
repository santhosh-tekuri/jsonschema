// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package httploader implements loader.Loader for http/https url.
//
// The package is typically only imported for the side effect of
// registering its Loaders.
//
// To use httploader, link this package into your program:
//	import _ "github.com/santhosh-tekuri/jsonschema/httploader"
//
package httploader

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/santhosh-tekuri/jsonschema/loader"
)

type httpLoader struct{}

func (httpLoader) Load(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func init() {
	loader.Register("http", httpLoader{})
	loader.Register("https", httpLoader{})
}
