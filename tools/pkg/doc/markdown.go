package doc

import (
	"text/template"
)

var markdownTemplate *template.Template

func init() {
	markdownTemplate = template.Must(template.New("MarkdownTemplate").Funcs(templateFuncMap).Parse(markdownTemplateText))
}

const markdownTemplateText = `
{{ define "CRDSummary" -}}
{{ $input := . -}}
# Custom Resources

{{ range $group := $input.Groups -}}
## {{ $group }}

{{ $groupVersions := index $input.GroupVersions $group -}}
{{ range $gv := $groupVersions -}}
### {{ $gv.Version }}

{{ $kinds := index $input.Kinds $gv -}}
{{ range $kind := $kinds -}}
{{ $schema := index $input.Schemas $kind -}}
[{{ $kind.Kind }}]({{ fileForKind $kind "md" }})
: {{ $schema.Description }}

{{ end }}
{{- end }}
{{- end }}
{{- end }}




{{- define "Type" }}

# {{ .Name }}

Used by [{{ .Kind.Kind }}({{ .Kind.Group }}/{{ .Kind.Version }})]({{ fileForKind .Kind "md" }})

{{ template "SchemaDetails" dict "Root" .Root "Schema" .Schema "Name" .Name "Kind" .Kind "Locations" .Locations "Seen" .Seen "Level" 1 "Depth" .Depth }}

{{- end }}




{{- define "Kind" }}

# {{ .Kind.Kind }} - {{ .Kind.Group }}/{{ .Kind.Version }}

{{ template "SchemaDetails" dict "Root" .Root "Schema" .Schema "Name" .Kind.Kind "Kind" .Kind "Locations" .Locations "Seen" .Seen "Level" 1 "Depth" .Depth }}

{{- end }}




{{ define "SchemaDetails" -}}
{{ packageForSchema .Schema }}

{{ if .Schema.Description -}}
{{ .Schema.Description }}

{{ end -}}
{{ $input := . -}}
{{ if (or .Schema.Properties .Schema.AllOf) -}}
| Field | Description | Type |
|------ | ----------- | ---- |
{{ template "TableRows" dict "Schema" $input.Schema "Root" $input.Root "Kind" $input.Kind "Locations" $input.Locations "External" (eq $input.Level $input.Depth) -}}
{{ else -}}
Type: {{ template "FieldType" dict "Field" .Schema "Kind" $input.Kind "Root" $input.Root "Locations" $input.Locations "External" (eq $input.Level $input.Depth) -}}
{{- end }}

{{ template "NestedProperties" dict "Root" $input.Root "Schema" $input.Schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $input.Level "Depth" $input.Depth -}}
{{ end -}}




{{ define "NestedProperties" -}}
{{ $input := . -}}
{{ if or (eq .Depth 0) (lt .Level .Depth) -}}
    {{ range $embeddedSchema := .Schema.AllOf -}}
{{ template "NestedProperties" dict "Root" $input.Root "Schema" (resolveSchema $input.Root $embeddedSchema) "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $input.Level "Depth" $input.Depth -}}
    {{ end -}}
{{ $level := incInt .Level -}}
    {{ range $name, $schema := .Schema.Properties -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" $schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $level "Depth" $input.Depth -}}
    {{ end -}}
    {{ if .Schema.AdditionalProperties -}}
        {{ if .Schema.AdditionalProperties.Schema -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" .Schema.AdditionalProperties.Schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $level "Depth" $input.Depth -}}
        {{ end -}}
    {{ end -}}
    {{ if .Schema.Items -}}
        {{ if .Schema.Items.Schema -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" .Schema.Items.Schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $level "Depth" $input.Depth -}}
        {{ else -}}
            {{ range $itemType := .Schema.Items.JSONSchemas -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" $itemType "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $level "Depth" $input.Depth -}}
            {{ end -}}
        {{ end -}}
    {{ end -}}
{{ end -}}
{{ end -}}




{{ define "NestedProperty" -}}
{{ $input := . -}}
{{ if .Schema.Ref -}}
    {{ if not ($input.Seen.Has .Schema.Ref) -}}
{{ $unused := $input.Seen.Insert .Schema.Ref -}}
        {{ if eq (packageForSchema $input.Root) (packageForSchema (resolveSchema $input.Root .Schema)) -}}
## {{ refName .Schema.Ref }}
{{ template "SchemaDetails" dict "Root" $input.Root "Schema" (resolveSchema $input.Root .Schema) "Name" (refName .Schema.Ref) "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" .Level "Depth" $input.Depth -}}
        {{ end -}}
    {{ end -}}
{{ else -}}
    {{ if .Schema.AdditionalProperties -}}
        {{ if .Schema.AdditionalProperties.Schema -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" .Schema.AdditionalProperties.Schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $input.Level "Depth" $input.Depth -}}
        {{ end -}}
    {{ end -}}
    {{ if .Schema.Items -}}
        {{ if .Schema.Items.Schema -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" .Schema.Items.Schema "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $input.Level "Depth" $input.Depth -}}
        {{ else -}}
            {{ range $itemType := .Schema.Items.JSONSchemas -}}
{{ template "NestedProperty" dict "Root" $input.Root "Schema" $itemType "Kind" $input.Kind "Locations" $input.Locations "Seen" $input.Seen "Level" $input.Level "Depth" $input.Depth -}}
            {{ end -}}
        {{ end -}}
    {{ end -}}
{{ end -}}
{{ end -}}



{{ define "TableRows" -}}
{{ $input := . -}}
{{ range $inline := .Schema.AllOf -}}
{{ template "TableRows" dict "Schema" (resolveSchema $input.Root $inline) "Root" $input.Root "Kind" $input.Kind "Locations" $input.Locations "External" $input.External -}}
{{ end -}}
{{ range $name, $property := .Schema.Properties -}}
| {{ $name }} | {{ $property.Description }} | {{ template "FieldType" dict "Field" $property "Kind" $input.Kind "Root" $input.Root "Locations" $input.Locations "External" $input.External }} |
{{ end -}}
{{ end -}}




{{ define "FieldType" -}}
{{ $input := . -}}
{{ if .Field.Ref -}}
    {{ if .External -}}
        {{ $location := index .Locations (resolveSchema .Root .Field).ID -}}
        {{ if $location -}}
[{{ refName .Field.Ref }}]({{ $location }})
        {{- else -}}
{{ refName .Field.Ref -}}
        {{ end -}}
    {{- else -}}
[{{ refName .Field.Ref }}](#{{ refName .Field.Ref }})
    {{- end -}}
{{ else -}}
    {{ if eq .Field.Type "array" -}}
        {{ if .Field.Items.Schema -}}
[]{{ template "FieldType" dict "Field" .Field.Items.Schema "Kind" $input.Kind "Root" $input.Root "Locations" $input.Locations "External" .External -}}
        {{ else -}}
{{ $input := . -}}
{{ range $itemType := .Field.Items.JSONSchemas -}}
  []{{ template "FieldType" dict "Field" $itemType "Kind" $input.Kind "Root" $input.Root "Locations" $input.Locations "External" $input.External -}}
{{ end -}}
        {{ end -}}
    {{ else -}}
        {{ if eq .Field.Type "object" -}}
            {{ if .Field.AdditionalProperties -}}
                {{ if .Field.AdditionalProperties.Schema -}}
map[string]{{ template "FieldType" dict "Field" .Field.AdditionalProperties.Schema "Kind" $input.Kind "Root" $input.Root "Locations" $input.Locations "External" .External -}}
                {{- else -}}
object
                {{- end -}}
            {{ else -}}
object
            {{- end -}}
        {{ else -}}
{{ .Field.Type -}}
        {{ end -}}
    {{ end -}}
{{ end -}}
{{ end }}
`
