package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func newLoader(insecure bool) jsonschema.URLLoader {
	httpLoader := HTTPLoader(http.Client{
		Timeout: 15 * time.Second,
	})
	if insecure {
		httpLoader.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return jsonschema.SchemeURLLoader{
		"file":  FileLoader{},
		"http":  &httpLoader,
		"https": &httpLoader,
	}
}

type FileLoader struct{}

func (l FileLoader) Load(url string) (any, error) {
	path, err := jsonschema.FileLoader{}.ToFile(url)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if ext := filepath.Ext(path); ext == ".yaml" || ext == ".yml" {
		var v any
		err := yaml.NewDecoder(f).Decode(&v)
		return v, err
	}
	return jsonschema.UnmarshalJSON(f)
}

type HTTPLoader http.Client

func (l *HTTPLoader) Load(url string) (any, error) {
	client := (*http.Client)(l)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()

	isYAML := strings.HasSuffix(url, ".yaml") || strings.HasSuffix(url, ".yml")
	if !isYAML {
		ctype := resp.Header.Get("Content-Type")
		isYAML = strings.HasSuffix(ctype, "/yaml") || strings.HasSuffix(ctype, "-yaml")
	}
	if isYAML {
		var v any
		err := yaml.NewDecoder(resp.Body).Decode(&v)
		return v, err
	}
	return jsonschema.UnmarshalJSON(resp.Body)
}
