#!/usr/bin/env bash

function grafana_patch_deployment() {
  file=${HELM_DIR}/istio-telemetry/grafana/templates/deployment.yaml
  sed_wrap -i -e '/      containers:/ a\
          # OAuth proxy\
        - name: grafana-proxy\
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
            name: secret-grafana-tls\
          - mountPath: /etc/proxy/htpasswd\
            name: secret-htpasswd\
          - mountPath: /etc/proxy/secrets\
            name: secret-grafana-proxy\
          - mountPath: /etc/pki/ca-trust/extracted/pem/\
            name: trusted-ca-bundle\
            readOnly: true\
          args:\
          - -provider=openshift\
          - -https-address=:3001\
          - -http-address=\
          - -email-domain=*\
          - -upstream=http://localhost:3000\
          - -htpasswd-file=/etc/proxy/htpasswd/auth\
          - -display-htpasswd-form=false\
          - '\''-openshift-sar={"namespace": "{{ .Release.Namespace }}", "resource": "pods", "verb": "get"}'\''\
          - -client-secret-file=/var/run/secrets/kubernetes.io/serviceaccount/token\
          - -openshift-service-account=grafana\
          - -cookie-secret-file=/etc/proxy/secrets/session_secret\
          - -tls-cert=/etc/tls/private/tls.crt\
          - -tls-key=/etc/tls/private/tls.key\
          - -openshift-ca=/etc/pki/tls/cert.pem\
          - -openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt' $file
  sed_wrap -i -e '/      volumes:/ a\
      # OAuth proxy\
      - name: secret-grafana-tls\
        secret:\
          defaultMode: 420\
          secretName: grafana-tls\
      - name: secret-htpasswd\
        secret:\
          defaultMode: 420\
          secretName: htpasswd\
      - name: secret-grafana-proxy\
        secret:\
          defaultMode: 420\
          secretName: grafana-proxy\
      - name: trusted-ca-bundle\
        configMap:\
          defaultMode: 420\
          items:\
            - key: ca-bundle.crt\
              path: tls-ca-bundle.pem\
          name: trusted-ca-bundle\
          optional: true' $file
  sed_wrap -i -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: grafana\
\1containers:\2/' $file
  sed_wrap -i -e '/- if .*\.security\.enabled/,/- end/ { d }' $file
  sed_wrap -i -e 's/^\(\( *\)-.*GF_PATHS_DATA.*\)$/\2- name: GF_AUTH_BASIC_ENABLED\
\2  value: "false"\
\2- name: GF_AUTH_PROXY_ENABLED\
\2  value: "true"\
\2- name: GF_AUTH_PROXY_AUTO_SIGN_UP\
\2  value: "true"\
\2- name: GF_AUTH_PROXY_WHITELIST\
\2  value: 127.0.0.0\/24,::1\
\2- name: GF_AUTH_PROXY_HEADERS\
\2  value: Email:X-Forwarded-Email\
\2- name: GF_AUTH_PROXY_HEADER_NAME\
\2  value: X-Forwarded-User\
\2- name: GF_USERS_AUTO_ASSIGN_ORG_ROLE\
\2  value: Admin\
\1/' $file
  sed_wrap -i -e 's+^ *image: "{{ .*\.image\.repository }}:{{ .*\.image\.tag }}"+{{- if contains "\/" .Values.grafana.image }}\
          image: "{{ .Values.grafana.image }}"\
{{- else }}\
          image: "{{ .Values.global.hub }}\/{{ .Values.grafana.image }}:{{ .Values.global.tag }}"\
{{- end }}+' $file

  sed_wrap -i -e '/securityContext/,/fsGroup/d' $file

  # Fix for MAISTRA-746, can be removed when we move to Istio-1.2
  sed_wrap -i -e '/^spec:/ a\
  strategy:\
    rollingUpdate:\
      maxSurge: 25%\
      maxUnavailable: 25%' $file
}

function grafana_patch_service() {
  sed_wrap -i -e 's/      targetPort: 3000/      targetPort: 3001/' ${HELM_DIR}/istio-telemetry/grafana/templates/service.yaml
}

function grafana_patch_values() {
  file=${HELM_DIR}/istio-telemetry/grafana/values.yaml
  # add annotations and enable ingress
  sed_wrap -i -e 's|  annotations: {}|  annotations:\n    service.alpha.openshift.io/serving-cert-secret-name: grafana-tls|' $file
  sed_wrap -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' $file
  sed_wrap -i -e 's+http://prometheus:9090+https://prometheus:9090+' $file
  sed_wrap -i -e 's/\(\( *\)access: proxy\)/\1\
\2basicAuth: true\
\2basicAuthPassword: ""\
\2basicAuthUser: internal\
\2version: 1/' $file
  sed_wrap -i -e 's+^\(\( *\)timeInterval.*\)$+\1\
\2# we should be using the CA cert in /var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt\
\2tlsSkipVerify: true+' $file
}

function GrafanaPatch() {
  echo "Patching Grafana"

  grafana_patch_deployment
	grafana_patch_values
  grafana_patch_service
}

GrafanaPatch
