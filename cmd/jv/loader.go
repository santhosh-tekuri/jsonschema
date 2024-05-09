package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func newLoader(mappings map[string]string, insecure bool, cacert string) (jsonschema.URLLoader, error) {
	httpLoader := HTTPLoader(http.Client{
		Timeout: 15 * time.Second,
	})
	if cacert != "" {
		pem, err := os.ReadFile(cacert)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(pem)
		httpLoader.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: caCertPool},
		}
	} else if insecure {
		httpLoader.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &JVLoader{
		mappings: mappings,
		fallback: jsonschema.SchemeURLLoader{
			"file":  FileLoader{},
			"http":  &httpLoader,
			"https": &httpLoader,
		}}, nil
}

// --

type JVLoader struct {
	mappings map[string]string
	fallback jsonschema.URLLoader
}

func (l *JVLoader) Load(url string) (any, error) {
	for prefix, dir := range l.mappings {
		if suffix, ok := strings.CutPrefix(url, prefix); ok {
			return loadFile(filepath.Join(dir, suffix))
		}
	}
	return l.fallback.Load(url)
}

func loadFile(path string) (any, error) {
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

// --

type FileLoader struct{}

func (l FileLoader) Load(url string) (any, error) {
	path, err := jsonschema.FileLoader{}.ToFile(url)
	if err != nil {
		return nil, err
	}
	return loadFile(path)
}

// --

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
