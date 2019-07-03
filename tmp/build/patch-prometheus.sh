#!/usr/bin/env bash

function prometheus_patch_deployment() {
  sed -i -e '/      containers:/ a\
          # OAuth proxy\
        - name: prometheus-proxy\
          image: openshift/oauth-proxy:latest\
          imagePullPolicy: IfNotPresent\
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
          args:\
          - -provider=openshift\
          - -https-address=:3001\
          - -http-address=\
          - -email-domain=*\
          - -upstream=http://localhost:9090\
          - '\''-openshift-sar={"namespace": "{{ .Release.Namespace }}", "resource": "pods", "verb": "get"}'\''\
          - '\''-openshift-delegate-urls={"/":{"namespace": "{{ .Release.Namespace }}", "resource": "pods", "verb": "get"}}'\''\
          - -skip-auth-regex=^/metrics\
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
          secretName: prometheus-tls' \
      -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: prometheus\
\1containers:\2/' \
  ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml

  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -r -e 's/image:(.*)prometheus:/image:\1prometheus-ubi8:/' ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml
  else
    sed -i -r -e 's/image:(.*)prometheus:/image:\1prometheus-rhel8:/' ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml
  fi

  sed -i -e '/args:/ a\
            - --storage.tsdb.path=/prometheus' ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml
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
 
  sed -i -e 's+hub:.*$+hub: '${HUB}'+g' \
         -e 's/tag:.*$/tag: '${MAISTRA_VERSION}'/' ${HELM_DIR}/istio/charts/prometheus/values.yaml
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
