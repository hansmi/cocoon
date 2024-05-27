package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type envMap map[string]*string

func readEnvFile(path string) (envMap, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening environment file %s: %w", path, err)
	}

	defer fh.Close()

	raw, err := io.ReadAll(fh)
	if err != nil {
		return nil, fmt.Errorf("reading environment file %s: %w", path, err)
	}

	var values envMap

	if err := yaml.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parsing environment YAML read from %s: %w", path, err)
	}

	return values, nil
}

func combineEnviron(base envMap, files []string, literal []string) (envMap, error) {
	environ := maps.Clone(base)

	if len(base) == 0 {
		environ = envMap{}
	}

	for _, i := range files {
		entries, err := readEnvFile(i)
		if err != nil {
			return nil, err
		}

		for variable, value := range entries {
			environ[variable] = value
		}
	}

	for _, i := range literal {
		if variable, value, hasValue := strings.Cut(i, "="); hasValue {
			environ[variable] = &value
		} else {
			environ[variable] = nil
		}
	}

	return environ, nil
}
