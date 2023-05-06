{{- define "sdNamespaces" }}
namespaces:
  names:
  - {{ .Values.global.istioNamespace }}
{{- if gt (len .Values.prometheus.scrapingNamespaces) 0 }}
{{ toYaml .Values.prometheus.scrapingNamespaces | indent 2 }}
{{- end }}
