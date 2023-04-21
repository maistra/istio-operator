{{- define "sdNamespaces" }}
namespaces:
  names:
{{- if gt (len .Values.prometheus.scrapingNamespaces) 0 }}
{{ toYaml .Values.prometheus.scrapingNamespaces | indent 2 }}
{{- else }}
  - {{ .Values.global.istioNamespace }}
{{- end }}
{{- end }}
