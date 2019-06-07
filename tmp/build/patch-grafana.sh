#!/usr/bin/env bash

function grafana_patch_deployment() {
  sed -i -e '/      containers:/ a\
          # OAuth proxy\
        - name: grafana-proxy\
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
            name: secret-grafana-tls\
          args:\
          - -provider=openshift\
          - -https-address=:3001\
          - -http-address=\
          - -email-domain=*\
          - -upstream=http://localhost:3000\
          - '\''-openshift-sar={"namespace": "istio-system", "resource": "pods", "verb": "get"}'\''\
          - '\''-openshift-delegate-urls={"/":{"namespace": "istio-system", "resource": "pods", "verb": "get"}}'\''\
          - -skip-auth-regex=^/metrics\
          - -client-secret-file=/var/run/secrets/kubernetes.io/serviceaccount/token\
          - -openshift-service-account=grafana\
          - -cookie-secret=SECRET\
          - -tls-cert=/etc/tls/private/tls.crt\
          - -tls-key=/etc/tls/private/tls.key\
          - -openshift-ca=/etc/pki/tls/cert.pem\
          - -openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt' \
      -e '/      volumes:/ a\
      # OAuth proxy\
      - name: secret-grafana-tls\
        secret:\
          defaultMode: 420\
          secretName: grafana-tls' \
      -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: grafana\
\1containers:\2/' \
  ${HELM_DIR}/istio/charts/grafana/templates/deployment.yaml
}

function grafana_patch_service() {
  sed -i -e 's/      targetPort: 3000/      targetPort: 3001/' ${HELM_DIR}/istio/charts/grafana/templates/service.yaml
}

function grafana_patch_values() {
  # add annotations and enable ingress
  sed -i \
    -e 's|  annotations: {}|  annotations:\n    service.alpha.openshift.io/serving-cert-secret-name: grafana-tls|' \
    -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
    ${HELM_DIR}/istio/charts/grafana/values.yaml

}

function grafana_patch_misc() {
  # - remove the extraneous create custom resources job
  rm ${HELM_DIR}/istio/charts/grafana/templates/create-custom-resources-job.yaml

  # - custom resources will be installed directly
  rm ${HELM_DIR}/istio/charts/grafana/templates/configmap-custom-resources.yaml

  sed -i -e '/grafana-default.yaml.tpl/d' -e '/{{.*end.*}}/d' ${HELM_DIR}/istio/charts/grafana/templates/grafana-ports-mtls.yaml
}

function GrafanaPatch() {
  echo "Patching Grafana"

  grafana_patch_deployment
  grafana_patch_service
  grafana_patch_values
  grafana_patch_misc
}

GrafanaPatch