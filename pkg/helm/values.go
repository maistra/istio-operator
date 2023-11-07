package helm

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type HelmValues map[string]interface{}

func (h HelmValues) GetString(key string) (string, bool, error) {
	return unstructured.NestedString(h, toKeys(key)...)
}

func (h HelmValues) Set(key string, val string) error {
	return unstructured.SetNestedField(h, val, toKeys(key)...)
}

func toKeys(key string) []string {
	return strings.Split(key, ".")
}
