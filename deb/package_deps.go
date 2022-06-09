package deb

import (
	"strings"
)

// PackageDependencies are various parsed dependencies
type PackageDependencies struct {
	Depends           []string
	BuildDepends      []string
	BuildDependsInDep []string
	PreDepends        []string
	Suggests          []string
	Recommends        []string
}

func parseDependencies(input Stanza, key string) []string {
	value := input.Get(key)
	if value == "" {
		return nil
	}

	input.Reset(key)

	value = strings.TrimSpace(value)
	if value == "" {
		// empty line is no depdencies
		return nil
	}

	result := strings.Split(value, ",")
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}
	return result
}
