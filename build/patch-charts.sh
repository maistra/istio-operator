#!/usr/bin/env bash

set -e

: ${HELM_DIR:?"Need to set HELM_DIR to output location for charts, e.g. tmp/_output/istio-releases/istio-1.1.0"}
: ${SOURCE_DIR:?"Need to set SOURCE_DIR to location of the istio-operator source directory"}

: ${OVERLAYS_DIR:=${SOURCE_DIR}/resources/helm/overlays}

# copy maistra specific templates into charts
function copyOverlay() {
  echo "copying Maistra chart customizations over stock Istio charts"
  find "${OVERLAYS_DIR}/" -maxdepth 1 -mindepth 1 -type d | xargs -I '{}' -n 1 -rt cp -rv '{}' ${HELM_DIR}
}

# The following modifications are made to the generated helm template for the Istio yaml files
# - remove the create customer resources job, we handle this in the installer to deal with potential races
# - remove the cleanup secrets job, we handle this in the installer
# - remove the kubernetes gateways
# - change privileged value on istio-proxy injection configmap to false
# - update the namespaceSelector to ignore namespaces with the label maistra.io/ignore-namespace
# - add a maistra-version label to all objects which have a release label
# - remove GODEBUG from the pilot environment
# - remove istio-multi service account
# - remove istio-multi cluster role binding
# - remove istio-reader cluster role
# - switch prometheus init container image from busybox to prometheus
# - switch webhook ports to 8443
# - switch health check files into /tmp
function patchTemplates() {
  echo "patching Istio Helm charts"

  # - add a maistra-version label to all objects which have a release label
  for file in $(find ${HELM_DIR} -name "*.yaml" -o -name "*.yaml.tpl"); do
    if grep -l 'release: ' $file; then
      sed_wrap -i -e '/^metadata:/,/^[^ {]/ { s/^\(.*\)labels:/\1labels:\
\1  maistra-version: '${MAISTRA_VERSION}'/ }' $file
    fi
    if grep -l '\.Values\.global\.istioNamespace' $file; then
      sed_wrap -i -e 's/\.Values\.global\.istioNamespace/.Release.Namespace/' $file
    fi
    if grep -l '{{- if not (eq .Values.revision "") }}-{{ .Values.revision }}{{- end }}' $file; then
      sed_wrap -i -e 's/{{- if not (eq .Values.revision "") }}-{{ .Values.revision }}{{- end }}/-{{ .Values.revision | default "default" }}/' $file
    fi
  done

  # MAISTRA-506 add a maistra-control-plane label for deployment specs
  for file in $(find ${HELM_DIR} -name "*.yaml" -o -name "*.yaml.tpl" | xargs grep -Hl '^kind: Deployment'); do
    sed_wrap -i -e '/^spec:/,$ { /template:$/,$ { /metadata:$/,$ { s/^\(.*\)labels:/\1labels:\
\1  maistra-control-plane: {{ .Release.Namespace }}/ } } }' $file
  done

  # - remove istio-multi service account
  # - for 1.6, this contains the sa for istiod, so need to do some munging
  sed_wrap -i -e 's/\(: istiod-service-account\)/\1-{{ .Values.revision | default "default" }}/' \
      ${HELM_DIR}/base/templates/serviceaccount.yaml \
      ${HELM_DIR}/base/templates/clusterrolebinding.yaml \
      ${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml

  sed_wrap -i -e '1,/^---/ d' \
    ${HELM_DIR}/base/templates/serviceaccount.yaml \
    ${HELM_DIR}/base/templates/clusterrolebinding.yaml

  # update role reference too
  sed_wrap -i -e 's/\(: istiod-pilot\).*$/\1-{{ .Values.revision | default "default" }}/' \
      -e 's/istiod-{{ .Release.Namespace }}/istiod-{{ .Values.revision | default "default" }}-{{ .Release.Namespace }}/' \
      ${HELM_DIR}/base/templates/clusterrolebinding.yaml \
  
  mv ${HELM_DIR}/base/templates/serviceaccount.yaml ${HELM_DIR}/istio-control/istio-discovery/templates/serviceaccount.yaml
  mv ${HELM_DIR}/base/templates/clusterrolebinding.yaml ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrolebinding.yaml
  
  # - remove istio-reader cluster role
  # - and again....
  sed_wrap -i -e '/^---/,$ d' \
      -e 's/name: istiod-.*$/name: istiod-{{ .Values.revision | default "default" }}-{{ .Release.Namespace }}/' \
      ${HELM_DIR}/base/templates/clusterrole.yaml
  mv ${HELM_DIR}/base/templates/clusterrole.yaml ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml

  # - move istiod specific templates to istio-discovery
  mv ${HELM_DIR}/base/templates/endpoints.yaml ${HELM_DIR}/base/templates/services.yaml ${HELM_DIR}/base/templates/validatingwebhookconfiguration.yaml ${HELM_DIR}/istio-control/istio-discovery/templates/

  # - nuke unnecessary files from base
  rm -rf ${HELM_DIR}/base/templates ${HELM_DIR}/base/files ${HELM_DIR}/base/crds/crd-operator.yaml

  # rename base/crds to istio-init/files
  mv ${HELM_DIR}/base/crds/* ${HELM_DIR}/istio-init/files
  rm -rf ${HELM_DIR}/base
}

function patchGalley() {
  echo "patching Galley specific Helm charts"
  # galley
  # Webhooks are not namespaced!  we do this to ensure we're not setting owner references on them
  # add namespace selectors
  # remove define block
  webhookconfig=${HELM_DIR}/istio-control/istio-discovery/templates/validatingwebhookconfiguration.yaml
  sed_wrap -i -e '/metadata:/,/webhooks:/ {
                /namespace:/d
                /name:/s/istiod-{{ .Release.Namespace }}/istiod-{{ .Values.revision | default "default" }}-{{ .Release.Namespace }}/
                /labels:/a\
\    istio.io/rev: \{\{ .Values.revision | default "default" \}\}
              }' $webhookconfig
  sed_wrap -i -e 's/name: istiod$/name: istiod-{{ .Values.revision | default "default" }}/' $webhookconfig
  sed_wrap -i -e 's|\(\(^ *\)rules:\)|\2namespaceSelector:\
\2  matchExpressions:\
\2  - key: maistra.io/member-of\
\2    operator: In\
\2    values:\
\2    - {{ .Release.Namespace }}\
\1|' $webhookconfig
  sed_wrap -i -e '/rules:/ a\
      - operations:\
        - CREATE\
        - UPDATE\
        apiGroups:\
        - authentication.maistra.io\
        apiVersions:\
        - "*"\
        resources:\
        - "*"\
      - operations:\
        - CREATE\
        - UPDATE\
        apiGroups:\
        - rbac.maistra.io\
        apiVersions:\
        - "*"\
        resources:\
        - "*"' $webhookconfig
  sed_wrap -i -e '1 i\
\{\{- if .Values.global.configValidation \}\}
' $webhookconfig

  sed_wrap -i -e 's/^---/{{- end }}/' $webhookconfig

  sed_wrap -i -e 's/failurePolicy: Ignore/failurePolicy: Fail/' $webhookconfig

  # add name to webhook port (XXX: move upstream)
  # change the location of the healthCheckFile from /health to /tmp/health
  # add --validation-port=8443
  deployment=${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml
  sed_wrap -i -e 's/^\(.*\)\- containerPort: 15017.*$/\1- name: webhook\
\1  containerPort: 15017/' $deployment

  # always istiod
  sed_wrap -i -e '/{{- if eq .Values.revision ""}}/,/{{- end }}/d' $deployment

  # multitenant
  echo '
  # Maistra specific
  - apiGroups: ["maistra.io"]
    resources: ["servicemeshmemberrolls"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["route.openshift.io"]
    resources: ["routes", "routes/custom-host"]
    verbs: ["get", "list", "watch", "create", "delete", "update"]' >> ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml
  sed_wrap -i -e 's/, *"nodes"//' ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml
  sed_wrap -i -e '/- apiGroups:.*admissionregistration\.k8s\.io/,/verbs:/ d' ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml
  sed_wrap -i -e '/- apiGroups:.*certificates\.k8s\.io/,/verbs:/ d' ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml
  sed_wrap -i -e '/- apiGroups:.*apiextensions\.k8s\.io/,/verbs:/ d' ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml

  convertClusterRoleBinding ${HELM_DIR}/istio-control/istio-discovery/templates/clusterrolebinding.yaml
  sed_wrap -i -e '/- "discovery"/ a\
          - --memberRollName=default\
          - --cacheCluster=outbound|80||wasm-cacher-{{ .Values.revision | default "default" }}.{{ .Release.Namespace }}.svc.cluster.local\
          - --podLocalitySource=pod' ${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml
  # disable webhook config updates
  sed_wrap -i -r -e '/INJECTION_WEBHOOK_CONFIG_NAME/,/ISTIOD_ADDR/ {
      /INJECTION_WEBHOOK_CONFIG_NAME/a\
\            value: ""\
\          - name: VALIDATION_WEBHOOK_CONFIG_NAME\
\            value: ""
      /INJECTION_WEBHOOK_CONFIG_NAME|ISTIOD_ADDR/! d
    }' $deployment
  # remove privileged security settings
  sed_wrap -i -r -e '/template:/,/containers:/ { /securityContext:/,/containers:/ { /containers:/! d }}' \
      -e '/containers:/,$ { /securityContext:/,/capabilities:/ { /capabilities:|securityContext:/! d }}' \
      $deployment
  echo '
base:
  validationURL: ""' >> ${HELM_DIR}/global.yaml
  # always istiod
  sed_wrap -i -e '/{{- if ne .Values.revision ""}}/,/{{- end }}/d' \
      -e '/matchLabels:/a\
\      app: istiod\
\      istio.io/rev: {{ .Values.revision | default "default" }}' \
    $deployment

  # IOR
  sed_wrap -i -e '/env:/ a\
{{- $iorEnabled := "true" }}\
{{- $gateway := index .Values "gateways" "istio-ingressgateway" }}\
{{- if or (not .Values.gateways.enabled) (not $gateway) (not $gateway.ior_enabled) }}\
{{- $iorEnabled = "false" }}\
{{- end }}\
          - name: ENABLE_IOR\
            value: "{{ $iorEnabled }}"' "${deployment}"

  # Extensions
  sed_wrap -i -e '/env:/ a\
          - name: ENABLE_MAISTRA_EXTENSIONS\
            value: "{{ .Values.wasmExtensions.enabled }}"' "${deployment}"
}

function patchGateways() {
  echo "patching Gateways specific Helm charts"
  sed_wrap -i -e 's/type: LoadBalancer$/type: ClusterIP/' ${HELM_DIR}/gateways/istio-ingress/values.yaml

  sed_wrap -i -e 's/\(^ *\)- containerPort: {{ $val.targetPort | default $val.port }}/\1- name: {{ $val.name }}\
\1  containerPort: {{ $val.targetPort | default $val.port }}/' ${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml
  sed_wrap -i -e 's/\(^ *\)- containerPort: {{ $val.targetPort | default $val.port }}/\1- name: {{ $val.name }}\
\1  containerPort: {{ $val.targetPort | default $val.port }}/' ${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml
}

function patchSidecarInjector() {
  echo "patching Sidecar Injector specific Helm charts"
  # Webhooks are not namespaced!  we do this to ensure we're not setting owner references on them
  webhookconfig=${HELM_DIR}/istio-control/istio-discovery/templates/mutatingwebhook.yaml
  sed_wrap -i -e '/^{{- if eq .Release.Namespace "istio-system"}}/,/^{{- end }}/d' \
      -e '/metadata:/a\
\  name: istiod-{{ .Values.revision | default "default" }}-{{ .Release.Namespace }}' \
    $webhookconfig
  sed_wrap -i -e '/if .Values.sidecarInjectorWebhook.enableNamespacesByDefault/,/{{- end/ {
      /enableNamespacesByDefault/i\
      matchExpressions:\
      - key: maistra.io\/member-of\
        operator: In\
        values:\
        - \{\{ .Release.Namespace \}\}\
      - key: maistra.io/ignore-namespace\
        operator: DoesNotExist\
      - key: istio-injection\
        operator: NotIn\
        values:\
        - disabled
      d
    }' $webhookconfig
  # remove {{- if not .Values.global.operatorManageWebhooks }} ... {{- end }}
  sed_wrap -i -e '/operatorManageWebhooks/ d' $webhookconfig
  sed_wrap -i -e '$ d' $webhookconfig

  # - change privileged value on istio-proxy injection configmap to false
  # setting the proper values will fix this:
  # global.proxy.privileged=false
  # global.proxy.enableCoreDump=false
  # however, we need to ensure privileged is set for istio_init
  sed_wrap -i -e '/- name: istio-init/,/- name: enable-core-dump/ {
    /privileged:/d
    /allowPrivilegeEscalation:/d
    / *- ALL/a\
        - KILL\
        - MKNOD\
        - SETGID\
        - SETUID
  }' ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml

  sed_wrap -i -e 's/runAsUser: 1337/runAsUser: {{ .ProxyUID }}/g' ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml
  sed_wrap -i -e 's/runAsGroup: 1337/runAsGroup: {{ .ProxyUID }}/g' ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml
  sed_wrap -i -e 's/fsGroup: 1337/fsGroup: {{ .ProxyUID }}/g' ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml

  sed_wrap -i -e '/- name: istio-proxy/,/resources:/ {
    / *- ALL/a\
        - KILL\
        - MKNOD\
        - SETGID\
        - SETUID
  }' ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml

  # replace 'default' in CA_ADDR spec to use valueOrDefault
  sed_wrap -i -e 's/value: istiod-{{ .Values.revision | default "default" }}.*$/value: istiod-{{ valueOrDefault .Values.revision "default" }}.{{ .Values.global.istioNamespace }}.svc:15012/' \
      ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml

  # never apply init container, even for validation
  sed_wrap -i -e 's${{ if ne (annotation .ObjectMeta `sidecar.istio.io/interceptionMode` .ProxyConfig.InterceptionMode) `NONE` }}${{ if not .Values.istio_cni.enabled -}}$' \
      ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml
  # use the correct cni network defintion
  sed_wrap -i -e '/podRedirectAnnot:/,$s/istio-cni/{{ .Values.istio_cni.istio_cni_network }}/' \
      ${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml
}

function patchMixer() {
  echo "patching Mixer specific Helm charts"

  # multitenant
  sed_wrap -i -e 's/^---.*$/\
- apiGroups: ["maistra.io"]\
  resources: ["servicemeshmemberrolls"]\
  verbs: ["get", "list", "watch"]/' ${HELM_DIR}/istio-policy/templates/clusterrole.yaml
  sed_wrap -i -e 's/^---.*$/\
- apiGroups: ["maistra.io"]\
  resources: ["servicemeshmemberrolls"]\
  verbs: ["get", "list", "watch"]/' ${HELM_DIR}/istio-telemetry/mixer-telemetry/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio-policy/templates/clusterrolebinding.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio-telemetry/mixer-telemetry/templates/clusterrolebinding.yaml
  sed_wrap -i -e '/name: *mixer/,/args:/ {
    /args/ a\
\          - --memberRollName=default\
\          - --memberRollNamespace=\{\{ .Release.Namespace \}\}
  }' ${HELM_DIR}/istio-policy/templates/deployment.yaml
  sed_wrap -i -e '/name: *mixer/,/args:/ {
    /args/ a\
\          - --memberRollName=default\
\          - --memberRollNamespace=\{\{ .Release.Namespace \}\}
  }' ${HELM_DIR}/istio-telemetry/mixer-telemetry/templates/deployment.yaml
}

# The following modifications are made to the generated helm template for the Kiali yaml file
# - remove all non-kiali operator configuration
# - remove values.yaml from community
function patchKialiTemplate() {
  echo "patching Kiali specific Helm charts"

  # we are using kiali operator, no need for the other templates
  for yaml in demosecret clusterrolebinding clusterrole configmap deployment serviceaccount service
  do
    rm "${HELM_DIR}/istio-telemetry/kiali/templates/${yaml}.yaml"
  done
  rm "${HELM_DIR}/istio-telemetry/kiali/values.yaml"
  rm "${HELM_DIR}/istio-telemetry/kiali/templates/_affinity.tpl"
}

# The following modifications are made to the upstream kiali configuration for deployment on OpenShift
function patchKialiOpenShift() {
  echo "more patching of Kiali specific Helm charts"
  echo "Nothing to do - using kiali operator and the kiali-cr.yaml"
}

function convertClusterToNamespaced() {
  # $1 - file to convert
  # $2 - cluster kind
  # $3 - namespaced kind
  # $4 - dereference
  sed_wrap -i -e 's/^\(\( *\)kind.*'$2'.*$\)/\2kind: '$3'/' "${1}"
  sed_wrap -i -e '0,/name:/ s/^\(\(.*\)name:.*$\)/\1\
\2namespace: {{ '$4'.Release.Namespace }}/' "${1}"
}

function convertClusterRoleBinding() {
  convertClusterToNamespaced "$1" "ClusterRoleBinding" "RoleBinding" "$2"
}

function removeUnsupportedCharts() {
  echo "removing unsupported Helm charts"
  rm -rf ${HELM_DIR}/istio-cni
  rm -rf ${HELM_DIR}/istio-telemetry/prometheusOperator
  rm -rf ${HELM_DIR}/istiocoredns
  rm -rf ${HELM_DIR}/istiod-remote
  rm -rf ${HELM_DIR}/istio-operator
}

function hacks() {
  echo "XXXXXXXX HACKS THAT NEED TO BE RESOLVED BEFORE 2.0 RELEASE XXXXXXXX"
  sed_wrap -i -e '/containers:/,/name: discovery/ {
      /name: discovery/a\
\          workingDir: "/"
    }' ${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml
}

copyOverlay
removeUnsupportedCharts

patchTemplates

patchGalley
patchGateways
patchSidecarInjector
patchMixer
patchKialiTemplate
patchKialiOpenShift

source ${SOURCE_DIR}/build/patch-grafana.sh
source ${SOURCE_DIR}/build/patch-jaeger.sh
source ${SOURCE_DIR}/build/patch-prometheus.sh

# XXX: hacks - remove before 2.0 release
hacks
