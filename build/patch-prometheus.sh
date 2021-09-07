#!/usr/bin/env bash

function prometheus_patch_deployment() {
  file=${HELM_DIR}/istio-telemetry/prometheus/templates/deployment.yaml
  sed_wrap -i -e '/      containers:/ a\
          # OAuth proxy\
        - name: prometheus-proxy\
{{- if contains "/" .Values.global.oauthproxy.image }}\
          image: {{ .Values.global.oauthproxy.image }}\
{{- else }}\
          image: {{ .Values.global.oauthproxy.hub }}/{{ .Values.global.oauthproxy.image }}:{{ .Values.global.oauthproxy.tag }}\
{{- end }}\
          imagePullPolicy: {{ .Values.global.oauthproxy.imagePullPolicy }}\
          ports:\
          - containerPort: 3001\
            name: https\
            protocol: TCP\
          readinessProbe:\
            failureThreshold: 3\
            periodSeconds: 10\
            successThreshold: 1\
            httpGet:\
              path: /oauth/healthz\
              port: https\
              scheme: HTTPS\
            timeoutSeconds: 1\
          resources:\
{{- if .Values.global.oauthproxy.resources }}\
{{ toYaml .Values.global.oauthproxy.resources | indent 12 }}\
{{- else }}\
{{ toYaml .Values.global.defaultResources | indent 12 }}\
{{- end }}\
          terminationMessagePath: /dev/termination-log\
          terminationMessagePolicy: File\
          volumeMounts:\
          - mountPath: /etc/tls/private\
            name: secret-prometheus-tls\
          - mountPath: /etc/proxy/htpasswd\
            name: secret-htpasswd\
          - mountPath: /etc/proxy/secrets\
            name: secret-prometheus-proxy\
          - mountPath: /etc/pki/ca-trust/extracted/pem/\
            name: trusted-ca-bundle\
            readOnly: true\
          args:\
          - -provider=openshift\
          - -https-address=:3001\
          - -http-address=\
          - -email-domain=*\
          - -upstream=http://localhost:9090\
          - -htpasswd-file=/etc/proxy/htpasswd/auth\
          - -display-htpasswd-form=false\
          - '\''-openshift-sar={"namespace": "{{ .Release.Namespace }}", "resource": "pods", "verb": "get"}'\''\
          - -client-secret-file=/var/run/secrets/kubernetes.io/serviceaccount/token\
          - -openshift-service-account=prometheus\
          - -cookie-secret-file=/etc/proxy/secrets/session_secret\
          - -tls-cert=/etc/tls/private/tls.crt\
          - -tls-key=/etc/tls/private/tls.key\
          - -openshift-ca=/etc/pki/tls/cert.pem\
          - -openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt' $file
  sed_wrap -i -e '/      volumes:/ a\
      # OAuth proxy\
      - name: secret-prometheus-tls\
        secret:\
          defaultMode: 420\
          secretName: prometheus-tls\
      - name: secret-htpasswd\
        secret:\
          defaultMode: 420\
          secretName: htpasswd\
      - name: secret-prometheus-proxy\
        secret:\
          defaultMode: 420\
          secretName: prometheus-proxy\
      - name: trusted-ca-bundle\
        configMap:\
          defaultMode: 420\
          items:\
            - key: ca-bundle.crt\
              path: tls-ca-bundle.pem\
          name: trusted-ca-bundle\
          optional: true' $file
  sed_wrap -i -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: prometheus\
\1containers:\2/' $file
  sed_wrap -i -e 's/^\(.*\)\(- .--config.file.*\)$/\1\2\
\1- --discovery.member-roll-name=default\
\1- --discovery.member-roll-namespace={{ .Release.Namespace }}/' $file

  sed_wrap -i -r -e 's/.*image:.*"\{\{ .*\.hub \}\}\/\{\{ .*\.image \}\}\:\{\{ .*\.tag \}\}".*$/{{- if contains "\/" .Values.prometheus.image }}\
          image: "{{ .Values.prometheus.image }}"\
{{- else }}\
          image: "{{ .Values.global.hub }}\/{{ .Values.prometheus.image }}:{{ .Values.global.tag }}"\
{{- end }}/' $file

	sed_wrap -i "/storage.tsdb.retention.*/a\ \ \ \ \ \ \ \ \ \ \ \ - \'--storage.tsdb.path=/prometheus\'" ${HELM_DIR}/istio-telemetry/prometheus/templates/deployment.yaml

  # Fix for MAISTRA-746, can be removed when we move to Istio-1.2
  sed_wrap -i -e '/^spec:/ a\
  strategy:\
    rollingUpdate:\
      maxSurge: 25%\
      maxUnavailable: 25%' $file
  
  sed_wrap -i -e '/        - name: istio-proxy/ a\
          resources:\
{{- if .Values.global.proxy.resources }}\
{{ toYaml .Values.global.proxy.resources | indent 12 }}\
{{- else }}\
{{ toYaml .Values.global.defaultResources | indent 12 }}\
{{- end }}' ${HELM_DIR}/istio-telemetry/prometheus/templates/deployment.yaml
}

function prometheus_patch_service() {
  sed_wrap -i -e '/port: 9090/ a\
    targetPort: 3001' \
    -e '/^{{- if .Values.prometheus.service }}/,/^{{- end }}/ {
      s/^{{- if .Values.prometheus.service }}/{{- if .Values.prometheus.service.nodePort }}\
{{- if .Values.prometheus.service.nodePort.enabled }}/
      /{{- end }}/a\
\{\{- end \}\}
      }' ${HELM_DIR}/istio-telemetry/prometheus/templates/service.yaml
}

function prometheus_patch_values() {
  # TODO: currently, upstream does not include the image value, so we're inserting it here (and removing hub and tag to make this values.yaml consistent with the others
  sed_wrap -i -e 's/hub:.*$/image: prometheus/g' \
         -e '/tag:.*$/d' ${HELM_DIR}/istio-telemetry/prometheus/values.yaml
}

function prometheus_patch_service_account() {
  sed_wrap -i -e '/name: prometheus/ a\
  annotations:\
    serviceaccounts.openshift.io/oauth-redirectreference.primary: '\''{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"prometheus"}}'\'' '\
    ${HELM_DIR}/istio-telemetry/prometheus/templates/serviceaccount.yaml
}

function prometheus_patch_misc() {
  sed_wrap -i -e '/nodes/d' \
         -e '/rules:/ a\
- apiGroups: ["maistra.io"]\
\  resources: ["servicemeshmemberrolls"]\
\  verbs: ["get", "list", "watch"]' \
         ${HELM_DIR}/istio-telemetry/prometheus/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio-telemetry/prometheus/templates/clusterrolebindings.yaml
}

function prometheus_patch_configmap() {
  sed_wrap -i -e "/job_name: 'kubernetes-apiservers'/,/^$/ c\
\    # config removed" ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml
  sed_wrap -i -e "/job_name: 'kubernetes-nodes'/,/^$/ c\
\    # config removed"  ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml
  sed_wrap -i -e "/job_name: 'kubernetes-cadvisor'/,/^$/ c\
\    # config removed" ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml

  # MAISTRA-748: Exclude scraping of prometheus itself on the oauth port
  sed_wrap -i -e '/job_name: '\''kubernetes-service-endpoints'\''/,/target_label: kubernetes_name$/ {
    /target_label: kubernetes_name$/ a\
      - source_labels: [__meta_kubernetes_service_name, __meta_kubernetes_pod_container_port_number]\
        regex: prometheus;3001\
        action: drop
  }' ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml

  sed_wrap -i -e "s/\(regex: istiod\)/\1.*/" ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml
  sed_wrap -i -e "s/\(regex: istio-telemetry\)/\1.*/" ${HELM_DIR}/istio-telemetry/prometheus/templates/configmap.yaml

}

function prometheusPatch() {
  echo "Patching Prometheus"

  prometheus_patch_deployment
  prometheus_patch_service
  prometheus_patch_service_account
  prometheus_patch_misc
  prometheus_patch_configmap
}

prometheusPatch

