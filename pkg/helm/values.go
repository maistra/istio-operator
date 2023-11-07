package helm

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type HelmValues map[string]any

// GetString returns the string value of a nested field.
// Returns false if value is not found and an error if not a string.
func (h HelmValues) GetString(key string) (string, bool, error) {
	return unstructured.NestedString(h, toKeys(key)...)
}

// Set sets the value of a nested field to a deep copy of the value provided.
// Returns an error if value cannot be set because one of the nesting levels is not a map[string]any.
func (h HelmValues) Set(key string, val interface{}) error {
	return unstructured.SetNestedField(h, val, toKeys(key)...)
}

func toKeys(key string) []string {
	return strings.Split(key, ".")
}
