{{- define "sdNamespaces" }}
{{- if gt (len .Values.prometheus.scrapingNamespaces) 0 }}
namespaces:
  names:
{{ toYaml .Values.prometheus.scrapingNamespaces | indent 2 }}
{{- end }}
{{- end }}
