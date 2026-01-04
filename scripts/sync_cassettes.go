package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type cassetteFile struct {
	Interactions []interaction `yaml:"interactions"`
	Version      int           `yaml:"version"`
}

type interaction struct {
	ID       int         `yaml:"id"`
	Request  cassetteReq `yaml:"request"`
	Response interface{} `yaml:"response"`
}

type cassetteReq struct {
	Body          string              `yaml:"body"`
	ContentLength int                 `yaml:"content_length"`
	Headers       map[string][]string `yaml:"headers"`
	Host          string              `yaml:"host"`
	Method        string              `yaml:"method"`
	Proto         string              `yaml:"proto"`
	ProtoMajor    int                 `yaml:"proto_major"`
	ProtoMinor    int                 `yaml:"proto_minor"`
	URL           string              `yaml:"url"`
	Form          interface{}         `yaml:"form,omitempty"`
	MultipartForm interface{}         `yaml:"multipart_form,omitempty"`
	Trailer       interface{}         `yaml:"trailer,omitempty"`
	TLS           interface{}         `yaml:"tls,omitempty"`
	TransferEnc   interface{}         `yaml:"transfer_encoding,omitempty"`
	Close         interface{}         `yaml:"close,omitempty"`
	RemoteAddr    interface{}         `yaml:"remote_addr,omitempty"`
	ContentType   interface{}         `yaml:"content_type,omitempty"`
}

func main() {
	files, err := filepath.Glob("/tmp/crush-vcr-mismatch/*_request.json")
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		fmt.Println("no mismatch files found")
		return
	}
	updated := 0
	for _, f := range files {
		if err := syncFile(f); err != nil {
			fmt.Fprintf(os.Stderr, "sync %s: %v\n", f, err)
			continue
		}
		updated++
	}
	fmt.Printf("updated %d cassette(s)\n", updated)
}

func syncFile(requestPath string) error {
	reqBody, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	var cassetteBody []byte
	cassettePathJSON := strings.TrimSuffix(requestPath, "_request.json") + "_cassette.json"
	if data, err := os.ReadFile(cassettePathJSON); err == nil {
		cassetteBody = data
	}
	testName := strings.TrimSuffix(filepath.Base(requestPath), "_request.json")
	if !strings.HasPrefix(testName, "TestCoderAgent_") {
		return nil
	}
	base := strings.TrimPrefix(testName, "TestCoderAgent_")
	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return fmt.Errorf("unexpected test name: %s", testName)
	}
	model := parts[0]
	caseName := strings.Join(parts[1:], "_")
	cassettePath := filepath.Join("internal/agent/testdata/TestCoderAgent", model, caseName+".yaml")

	data, err := os.ReadFile(cassettePath)
	if err != nil {
		return err
	}
	var cas cassetteFile
	if err := yaml.Unmarshal(data, &cas); err != nil {
		return fmt.Errorf("unmarshal cassette %s: %w", cassettePath, err)
	}
	if len(cas.Interactions) == 0 {
		return fmt.Errorf("no interactions in %s", cassettePath)
	}
	target := 0
	if len(cassetteBody) > 0 {
		for idx, inter := range cas.Interactions {
			if inter.Request.Body == string(cassetteBody) {
				target = idx
				break
			}
		}
	}

	cas.Interactions[target].Request.Body = string(reqBody)
	cas.Interactions[target].Request.ContentLength = len(reqBody)

	out, err := yaml.Marshal(&cas)
	if err != nil {
		return err
	}
	return os.WriteFile(cassettePath, out, 0o644)
}
