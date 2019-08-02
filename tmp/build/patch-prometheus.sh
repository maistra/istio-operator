#!/usr/bin/env bash

function prometheus_patch_deployment() {
  sed -i -e '/      containers:/ a\
          # OAuth proxy\
        - name: prometheus-proxy\
          image: {{ .Values.global.oauthproxy.hub }}/{{ .Values.global.oauthproxy.image }}:{{ .Values.global.oauthproxy.tag }}\
          imagePullPolicy: {{ .Values.global.oauthproxy.imagePullPolicy }}\
          ports:\
          - containerPort: 3001\
            name: https\
            protocol: TCP\
          readinessProbe:\
            failureThreshold: 3\
            periodSeconds: 10\
            successThreshold: 1\
            tcpSocket:\
              port: https\
            timeoutSeconds: 1\
          resources: {}\
          terminationMessagePath: /dev/termination-log\
          terminationMessagePolicy: File\
          volumeMounts:\
          - mountPath: /etc/tls/private\
            name: secret-prometheus-tls\
          - mountPath: /etc/proxy/htpasswd\
            name: secret-htpasswd\
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
          - -cookie-secret=SECRET\
          - -tls-cert=/etc/tls/private/tls.crt\
          - -tls-key=/etc/tls/private/tls.key\
          - -openshift-ca=/etc/pki/tls/cert.pem\
          - -openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt' \
      -e '/      volumes:/ a\
      # OAuth proxy\
      - name: secret-prometheus-tls\
        secret:\
          defaultMode: 420\
          secretName: prometheus-tls\
      - name: secret-htpasswd\
        secret:\
          defaultMode: 420\
          secretName: htpasswd' \
      -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: prometheus\
\1containers:\2/' \
  ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml

  sed -i -r -e 's/.*image:.*prometheus.*$/{{- if contains "\/" .Values.image }}\
          image: "{{ .Values.image }}"\
{{- else }}\
          image: "{{ .Values.global.hub }}\/{{ .Values.image }}:{{ .Values.global.tag }}"\
{{- end }}/' ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml

	sed -i "/storage.tsdb.retention.*/a\ \ \ \ \ \ \ \ \ \ \ \ - \'--storage.tsdb.path=/prometheus\'" ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml
}

function prometheus_patch_service() {
  sed -i -e '/port: 9090/ a\
    targetPort: 3001' ${HELM_DIR}/istio/charts/prometheus/templates/service.yaml
}

function prometheus_patch_values() {
  # add annotations and enable ingress
  sed -i \
    -e 's|  annotations: {}|  annotations:\n    service.alpha.openshift.io/serving-cert-secret-name: prometheus-tls|' \
    -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
    ${HELM_DIR}/istio/charts/prometheus/values.yaml

  # TODO: currently, upstream does not include the image value, so we're inserting it here (and removing hub and tag to make this values.yaml consistent with the others
  sed -i -e 's/hub:.*$/image: prometheus/g' \
         -e '/tag:.*$/d' ${HELM_DIR}/istio/charts/prometheus/values.yaml

  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -r -e 's/image:(.*)prometheus/image: prometheus-ubi8/' ${HELM_DIR}/istio/charts/prometheus/values.yaml
  else
    sed -i -r -e 's/image:(.*)prometheus/image: prometheus-rhel8/' ${HELM_DIR}/istio/charts/prometheus/values.yaml
  fi
}

function prometheus_patch_service_account() {
  sed -i -e '/name: prometheus/ a\
  annotations:\
    serviceaccounts.openshift.io/oauth-redirectreference.primary: '\''{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"prometheus"}}'\'' '\
    ${HELM_DIR}/istio/charts/prometheus/templates/serviceaccount.yaml
}

function prometheus_patch_misc() {
  sed -i -e '/nodes/d' ${HELM_DIR}/istio/charts/prometheus/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/prometheus/templates/clusterrolebindings.yaml
}

function prometheus_patch_configmap() {
  sed -i -e "/job_name: 'kubernetes-apiservers'/,/^$/ c\
\    # config removed" ${HELM_DIR}/istio/charts/prometheus/templates/configmap.yaml
  sed -i -e "/job_name: 'kubernetes-nodes'/,/^$/ c\
\    # config removed"  ${HELM_DIR}/istio/charts/prometheus/templates/configmap.yaml
  sed -i -e "/job_name: 'kubernetes-cadvisor'/,/^$/ c\
\    # config removed" ${HELM_DIR}/istio/charts/prometheus/templates/configmap.yaml
}

function prometheusPatch() {
  echo "Patching Prometheus"

  prometheus_patch_deployment
  prometheus_patch_service
  prometheus_patch_service_account
  prometheus_patch_values
  prometheus_patch_misc
  prometheus_patch_configmap
}

prometheusPatch
