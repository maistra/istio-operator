package doc

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"text/template"

	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var templateFuncMap = template.FuncMap{
	"refName":                  refName,
	"headingID":                headingID,
	"resolveSchema":            resolveSchema,
	"dict":                     dict,
	"fileForKind":              fileForKind,
	"packageForSchema":         packageForSchema,
	"incInt":                   incInt,
	"fileForExternalReference": fileForExternalReference,
}

func fileForKind(kind schema.GroupVersionKind, format string) string {
	return fmt.Sprintf("%s_%s_%s.%s", kind.Group, kind.Kind, kind.Version, format)
}

func fileForExternalReference(kind schema.GroupVersionKind, ref, format string) string {
	return fmt.Sprintf("%s_%s_%s_%s.%s", kind.Group, kind.Kind, refName(ref), kind.Version, format)
}

func packageForSchema(schema *apiext.JSONSchemaProps) string {
	id, err := url.Parse(schema.ID)
	if err != nil {
		panic(err)
	}
	return path.Dir(id.Opaque)
}

func incInt(val int) int {
	return val + 1
}

func refName(ref string) string {
	ref = path.Base(ref)
	s := strings.Split(ref, "_")
	return s[len(s)-1]
}

func headingID(ref string) string {
	return path.Base(ref)
}

func resolveSchema(base *apiext.JSONSchemaProps, rawProps interface{}) *apiext.JSONSchemaProps {
	if rawProps == nil || base == nil {
		return nil
	}
	var props *apiext.JSONSchemaProps
	switch typedProps := rawProps.(type) {
	case apiext.JSONSchemaProps:
		props = &typedProps
	case *apiext.JSONSchemaProps:
		props = typedProps
	default:
		panic(fmt.Errorf("props must be apiext.JSONSchemaProps"))
	}
	if props.Ref != nil {
		refName := path.Base(*props.Ref)
		ret := base.Definitions[refName]
		return &ret
	}
	return props
}

func dict(values ...interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for index, size := 0, len(values)-1; index < size; index += 2 {
		if key, ok := values[index].(string); ok {
			ret[key] = values[index+1]
		} else {
			panic(fmt.Errorf("expected string value for key, got: %T, %#v", values[index], values[index]))
		}
	}
	return ret
}
