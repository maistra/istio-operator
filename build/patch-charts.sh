#!/usr/bin/env bash
# shellcheck disable=SC1004

set -e -u

: "${HELM_DIR:?"Need to set HELM_DIR to output location for charts, e.g. tmp/_output/istio-releases/istio-1.1.0"}"
: "${SOURCE_DIR:?"Need to set SOURCE_DIR to location of the istio-operator source directory"}"

: "${OVERLAYS_DIR:=${SOURCE_DIR}/resources/helm/overlays}"

# copy maistra specific templates into charts
function copyOverlay() {
  echo "copying Maistra chart customizations over stock Istio charts"
  find "${OVERLAYS_DIR}/" -maxdepth 1 -mindepth 1 -type d -print0 | xargs -0 -I '{}' -n 1 -rt cp -rv '{}' "${HELM_DIR}"
}

function patchTemplates() {
  echo "patching Istio Helm charts"

  # MAISTRA-506 add a maistra-control-plane label for deployment specs
  for file in $(find "${HELM_DIR}" -name "*.yaml" -not \( -name "values.yaml" -prune \) -print0| xargs -0 grep -Hl '^kind: Deployment'); do
    sed_wrap -i -e '/^spec:/,$ { /template:$/,$ { /metadata:$/,$ { s/^\(.*\)labels:/\1labels:\
\1  maistra-control-plane: {{ .Release.Namespace }}/ } } }' "$file"
  done

  # role and role binding are for istiod only
  sed_wrap -i -e '/labels:/ i\
  annotations:\
    "maistra.io/internal": "true"' \
    -e 's/^  name: istiod{{/  name: istiod-internal{{/' \
      "${HELM_DIR}/istio-control/istio-discovery/templates/role.yaml" \
      "${HELM_DIR}/istio-control/istio-discovery/templates/rolebinding.yaml"

  # read trustDomain from meshConfig instead of hardcoding
  sed_wrap -i -e 's/"cluster.local"/{{ .Values.meshConfig.trustDomain | default .Values.global.trustDomain }}/g' "${HELM_DIR}/istio-control/istio-discovery/templates/configmap.yaml"

  # OSSM-924 If gatewayAPI.controllerMode is enabled, set discoverySelectors
  # so that we only watch namespaces that are listed in the MemberRoll
  sed_wrap -i -e '/define "mesh"/a\
{{- if .Values.gatewayAPI.controllerMode }}\
    discoverySelectors:\
    - matchLabels:\
        maistra.io/member-of: "{{ .Release.Namespace }}"\
{{- end }}\
' "${HELM_DIR}/istio-control/istio-discovery/templates/configmap.yaml"

  # rename base/crds to istio-init/files
  mkdir -p "${HELM_DIR}/istio-init/files"
  mv "${HELM_DIR}"/base/crds/* "${HELM_DIR}/istio-init/files"
  rm -rf "${HELM_DIR}/base"
  CRD_DIR="${HELM_DIR}/istio-init/files" "${SOURCE_DIR}/build/split-istio-crds.sh"

  # MAISTRA-1972 - disable protocol sniffing
  sed_wrap -i -e 's/\(enableProtocolSniffing.*:\).*$/\1 false/' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"


  # - add a maistra-version label to all objects which have a release label
  # do this after we've separated crds
  # shellcheck disable=SC2044
  for file in $(find "${HELM_DIR}" -name "*.yaml" -o -name "*.yaml.tpl"); do
    if grep -l 'release: ' "$file"; then
      sed_wrap -i -e '/^metadata:/,/^[^ {]/ { s/^\(.*\)labels:/\1labels:\
\1  maistra-version: "'"${MAISTRA_VERSION}"'"/ }' "$file"
    elif sed_nowrap -ne '/^metadata:/,/^spec:/p' "$file" | grep -l "labels:"; then
      sed_wrap -i -e 's/^\(.*\)labels:/\1labels:\
\1  maistra-version: "'"${MAISTRA_VERSION}"'"/' "$file"
    elif grep -l '^metadata:' "$file"; then
      sed_wrap -i -e '/^metadata:/ a\
  labels:\
    maistra-version: "'"${MAISTRA_VERSION}"'"' "$file"
    fi
    if grep -l 'operator\.istio\.io' "$file"; then
      sed_wrap -i -e '/operator\.istio\.io/d' "$file"
    fi
  done

    echo "
gatewayAPI:
  enabled: false
  controllerMode: false" | \
    tee >(cat >> "${HELM_DIR}/istio-control/istio-discovery/values.yaml")
}

function patchGalley() {
  echo "patching Galley specific Helm charts"
  # galley
  # Webhooks are not namespaced!  we do this to ensure we're not setting owner references on them
  # add namespace selectors
  # remove define block
  webhookconfig=${HELM_DIR}/istio-control/istio-discovery/templates/validatingwebhookconfiguration.yaml
  sed_wrap -i -e 's|\(\(^ *\)rules:\)|\2namespaceSelector:\
\2  matchExpressions:\
\2  - key: maistra.io/member-of\
\2    operator: In\
\2    values:\
\2    - {{ .Release.Namespace }}\
\1|' "${webhookconfig}"
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
        - "*"' "${webhookconfig}"

  sed_wrap -i '/.*objectSelector:/,/.*{{- end }}/d' "${webhookconfig}"

  sed_wrap -i -e 's/failurePolicy: Ignore/failurePolicy: Fail/' "${webhookconfig}"

  # add name to webhook port (XXX: move upstream)
  # change the location of the healthCheckFile from /health to /tmp/health
  # add --validation-port=8443
  deployment="${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml"
  sed_wrap -i -e 's/^\(.*\)\- containerPort: 15017.*$/\1- name: webhook\
\1  containerPort: 15017/' "${deployment}"

  # multitenant
  sed_wrap -i -e '/^---/i \
  # Allow use of blockOwnerDeletion in ownerReferences pointing to Pods (see OSSM-1321)\
  - apiGroups: [""]\
    resources: ["pods/finalizers"]\
    verbs: ["update"]' "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml"

  sed_wrap -i -e '/- "discovery"/ a\
{{- if not .Values.global.clusterWide }}\
          - --memberRollName=default\
          - --enableNodeAccess=false\
          - --enableCRDScan=false\
          - --enableIngressClassName=false\
{{- end}}' "${deployment}"

  # remove privileged security settings
  sed_wrap -i -r -e '/template:/,/containers:/ { /securityContext:/,/containers:/ { /containers:/! d }}' \
      -e '/containers:/,$ { /securityContext:/,/capabilities:/ { /capabilities:|securityContext:/! d }}' "${deployment}"

  sed_wrap -i -e '/base:/ a\
  validationURL: ""' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  # IOR
  # shellcheck disable=SC2016
  sed_wrap -i -e '/env:/ a\
{{- $iorEnabled := "true" }}\
{{- $gateway := index .Values "gateways" "istio-ingressgateway" }}\
{{- if or (not .Values.gateways.enabled) (not $gateway) (not $gateway.ior_enabled) }}\
{{- $iorEnabled = "false" }}\
{{- end }}\
          - name: ENABLE_IOR\
            value: "{{ $iorEnabled }}"\
          - name: PILOT_CA_CERT_CONFIG_MAP_NAME\
            value: "{{ .Values.global.caCertConfigMapName }}"\
{{- if .Values.gatewayAPI.enabled }}\
          - name: PILOT_ENABLE_GATEWAY_API\
            value: "true"\
          - name: PILOT_ENABLE_GATEWAY_API_STATUS\
            value: "true"\
          - name: PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER\
            value: "true"\
{{- if .Values.gatewayAPI.controllerMode }}\
          - name: PILOT_ENABLE_GATEWAY_CONTROLLER_MODE\
            value: "true"\
          - name: PILOT_GATEWAY_API_DEFAULT_GATEWAYCLASS\
            value: "ocp"\
          - name: PILOT_GATEWAY_API_CONTROLLER_NAME\
            value: "openshift.io/gateway-controller"\
          - name: PILOT_GATEWAY_API_DEPLOYMENT_DEFAULT_LABELS\
            value: "{\\"gateway.openshift.io/inject\\":\\"true\\"}"\
{{- end }}\
{{- else }}\
          - name: PILOT_ENABLE_GATEWAY_API\
            value: "false"\
          - name: PILOT_ENABLE_GATEWAY_API_STATUS\
            value: "false"\
          - name: PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER\
            value: "false"\
{{- end }}' "${deployment}"

  sed_wrap -i -e 's/istio-ca-root-cert/"{{ .Values.global.caCertConfigMapName }}"/g' \
      "${deployment}"

  sed_wrap -i -e '/base:/ i\
gateways: {}\n' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  # analysis
  sed_wrap -i -e '/PILOT_ENABLE_ANALYSIS/ i\
          - name: PILOT_ENABLE_STATUS\
            value: "{{ .Values.global.istiod.enableAnalysis }}"
  ' "${deployment}"

  sed_wrap -i -e '/---/ i\
{{- if .Values.global.istiod.enableAnalysis }}\
  - apiGroups: ["apps"]\
    verbs: ["get", "watch", "list"]\
    resources: [ "deployments" ]\
{{- end }}' "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml"
}

function patchGateways() {
  echo "patching Gateways specific Helm charts"
  sed_wrap -i -r -e 's/type: LoadBalancer *(#.*)?$/type: ClusterIP/' "${HELM_DIR}/gateways/istio-ingress/values.yaml"

  # add tracer config
  # shellcheck disable=SC2016
  tracerConfig='\
  # Configuration for each of the supported tracers\
  tracer:\
    # Configuration for envoy to send trace data to LightStep.\
    # Disabled by default.\
    # address: the <host>:<port> of the satellite pool\
    # accessToken: required for sending data to the pool\
    #\
    datadog:\
      # Host:Port for submitting traces to the Datadog agent.\
      address: "$(HOST_IP):8126"\
    lightstep:\
      address: ""                # example: lightstep-satellite:443\
      accessToken: ""            # example: abcdefg1234567\
    stackdriver:\
      # enables trace output to stdout.\
      debug: false\
      # The global default max number of message events per span.\
      maxNumberOfMessageEvents: 200\
      # The global default max number of annotation events per span.\
      maxNumberOfAnnotations: 200\
      # The global default max number of attributes per span.\
      maxNumberOfAttributes: 200\
    zipkin:\
      # Host:Port for reporting trace data in zipkin format. If not specified, will default to\
      # zipkin service (port 9411) in the same namespace as the other istio components.\
      address: ""\
'
  sed_wrap -i -e "/meshConfig:/ i$tracerConfig" "${HELM_DIR}/gateways/istio-ingress/values.yaml"
  sed_wrap -i -e "/meshConfig:/ i$tracerConfig" "${HELM_DIR}/gateways/istio-egress/values.yaml"

  echo "
gatewayAPI:
  enabled: false
  controllerMode: false" | \
  tee >(cat >> "${HELM_DIR}/gateways/istio-ingress/values.yaml")\
      >(cat >> "${HELM_DIR}/gateways/istio-egress/values.yaml")

  sed_wrap -i -e '/  logLevel:/ a\\n    tracer: zipkin' "${HELM_DIR}/gateways/istio-ingress/values.yaml"
  sed_wrap -i -e '/  logLevel:/ a\\n    tracer: zipkin' "${HELM_DIR}/gateways/istio-egress/values.yaml"

  sed_wrap -i -e '/  priorityClassName:/ a\
\
  # Whether to enable the Kubernetes Ingress feature.\
  k8sIngress:\
    enabled: false' "${HELM_DIR}/gateways/istio-ingress/values.yaml"

  # shellcheck disable=SC2016
  sed_wrap -i -e 's/\(^ *\)- containerPort: {{ $val.targetPort | default $val.port }}/\1- name: {{ $val.name }}\
\1  containerPort: {{ $val.targetPort | default $val.port }}/' "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml" "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"

  # add label for easier selection in Gateway resources
  # shellcheck disable=SC2016
  sed_wrap -i -e 's/^\(.*\)labels:/\1labels:\
\1  maistra.io\/gateway: {{ $gateway.name | default "istio-ingressgateway" }}.{{ $gateway.namespace | default .Release.Namespace }}/' "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml"
  # shellcheck disable=SC2016
  sed_wrap -i -e 's/^\(.*\)labels:/\1labels:\
\1  maistra.io\/gateway: {{ $gateway.name | default "istio-egressgateway" }}.{{ $gateway.namespace | default .Release.Namespace }}/' "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"

  # MAISTRA-1963 Mark gateways as non-privileged
  sed_wrap -i -e '/env:/ a\
          - name: ISTIO_META_UNPRIVILEGED_POD\
            value: "true"
' "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml" "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"

  # MAISTRA-2528 Enable DNS Capture for proxies by default
  # MAISTRA-2656 Fix missing DNS registry entries in istio-agent
  sed_wrap -i -e '/env:/ a\
          - name: ISTIO_META_DNS_CAPTURE\
            value: "true"\
          - name: ISTIO_META_DNS_AUTO_ALLOCATE\
            value: "true"\
          - name: PROXY_XDS_VIA_AGENT\
            value: "true"
' "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml" "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"

  sed_wrap -i -e 's/proxyMetadata: {}/proxyMetadata:\
      ISTIO_META_DNS_CAPTURE: "true"\
      ISTIO_META_DNS_AUTO_ALLOCATE: "true"\
      PROXY_XDS_VIA_AGENT: "true"/
' "${HELM_DIR}/gateways/istio-ingress/values.yaml" "${HELM_DIR}/gateways/istio-egress/values.yaml"

  # gateways in other namespaces need proxy config
  # shellcheck disable=SC2016
  echo "$(sed_nowrap -ne '1,/\$mesh :=/p' "${HELM_DIR}/istio-control/istio-discovery/templates/configmap.yaml"; cat "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml")" > "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"
  # shellcheck disable=SC2016
  echo "$(sed_nowrap -ne '1,/\$mesh :=/p' "${HELM_DIR}/istio-control/istio-discovery/templates/configmap.yaml"; cat "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml")" > "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml"
  # shellcheck disable=SC2016
  sed_wrap -i -e '/env:/ a\
{{- if $gateway.namespace }}\
{{- if ne $gateway.namespace .Release.Namespace }}\
          - name: PROXY_CONFIG\
            value: |-\
{{ toYaml $mesh.defaultConfig | trim | indent 14 }}\
{{- end }}\
{{- end }}
' "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml" "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml"

  # install in specified namespace
  for file in "${HELM_DIR}"/gateways/istio-ingress/templates/*.yaml; do
    # shellcheck disable=SC2016
    sed_wrap -i -e 's/^\( *\)namespace:.*/\1namespace: {{ $gateway.namespace | default .Release.Namespace }}/' "$file"
  done
  for file in "${HELM_DIR}"/gateways/istio-egress/templates/*.yaml; do
    # shellcheck disable=SC2016
    sed_wrap -i -e 's/^\( *\)namespace:.*/\1namespace: {{ $gateway.namespace | default .Release.Namespace }}/' "$file"
  done
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

function patchCNI() {
  sed_wrap -i -e '/- get/ a\
- apiGroups:\
  - security.openshift.io\
  resources:\
  - securitycontextconstraints\
  resourceNames:\
  - privileged\
  verbs:\
  - use' "${HELM_DIR}/istio_cni/templates/clusterrole.yaml"
}


function removeUnsupportedCharts() {
  echo "removing unsupported Helm charts"
  rm -rf "${HELM_DIR}/istiocoredns"
  rm -rf "${HELM_DIR}/istiod-remote"
  rm -rf "${HELM_DIR}/istio-operator"
  # FIXME: we rename the upstream chart for now to get it to work with our operator
  mv "${HELM_DIR}/istio-cni" "${HELM_DIR}/istio_cni"
}

function moveEnvoyFiltersToMeshConfigChart() {
  echo "moving EnvoyFilter manifests to mesh-config"
  mv "${HELM_DIR}"/istio-control/istio-discovery/templates/telemetry*.yaml "${HELM_DIR}/mesh-config/templates"

  sed_nowrap -n -e '/^telemetry:/,/^      logWindowDuration/ p' "${HELM_DIR}/istio-control/istio-discovery/values.yaml" > "${HELM_DIR}/mesh-config/values.yaml"
  sed_wrap -i -n -e '/^telemetry:/,/^      logWindowDuration/ d; p' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"
  sed_wrap -i -e '/multiCluster:/ i\
  # Default mtls policy. If true, mtls between services will be enabled by default.\
  mtls:\
    # Default setting for service-to-service mtls. Can be set explicitly using\
    # destination rules or service annotations.\
    enabled: false\
    # If set to true, and a given service does not have a corresponding DestinationRule configured,\
    # or its DestinationRule does not have TLSSettings specified, Istio configures client side\
    # TLS configuration automatically, based on the server side mTLS authentication policy and the\
    # availibity of sidecars.\
    auto: true\
' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  sed_wrap -i -e '/telemetry:/ i\
# meshConfig defines runtime configuration of components, including Istiod and istio-agent behavior\
# See https://istio.io/docs/reference/config/istio.mesh.v1alpha1/ for all available options\
meshConfig:\
  # The namespace to treat as the administrative root namespace for Istio configuration.\
  # When processing a leaf namespace Istio will search for declarations in that namespace first\
  # and if none are found it will search in the root namespace. Any matching declaration found in the root namespace\
  # is processed as if it were declared in the leaf namespace.\
  rootNamespace:\
\
global:\
  multiCluster:\
    # Should be set to the name of the cluster this installation will run in. This is required for sidecar injection\
    # to properly label proxies\
    clusterName: ""\
\
  # Default mtls policy. If true, mtls between services will be enabled by default.\
  mtls:\
    # Default setting for service-to-service mtls. Can be set explicitly using\
    # destination rules or service annotations.\
    enabled: false\
    # If set to true, and a given service does not have a corresponding DestinationRule configured,\
    # or its DestinationRule does not have TLSSettings specified, Istio configures client side\
    # TLS configuration automatically, based on the server side mTLS authentication policy and the\
    # availibity of sidecars.\
    auto: true\n' "${HELM_DIR}/mesh-config/values.yaml"

}

function copyGlobalValues() {
  echo "copying global.yaml file from overlay charts as global.yaml file is removed in upstream but it's still needed."
  cp "${SOURCE_DIR}/resources/helm/overlays/global.yaml" "${HELM_DIR}/"
}

# This hack is hopefully only needed for a few versions until this PR is merged: https://github.com/istio/istio/pull/39375
# It essentially modifies the chart to have the exact same changes
function patchPilotServingCert() {
  # add extra values
  sed_wrap -i -e '/traceSampling:/ a\
  extraArgs: []\
  extraVolumeMounts: []\
  extraVolumes: []' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  # add extra volume in deployments (prepend before end of file)
  sed_wrap -i -e '/^---$/ i\
{{- if .Values.pilot.extraVolumes }}\
{{ toYaml .Values.pilot.extraVolumes | indent 6 }}\
{{- end }}' "${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml"

  # add extra volume mounts (by prepending to volumesMounts: block)
  sed_wrap -i -e '/volumeMounts:/ a\
{{- if .Values.pilot.extraVolumeMounts }}\
{{ toYaml .Values.pilot.extraVolumeMounts | indent 10 }}\
{{- end }}' "${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml"

  # Add extraArgs (by appending after discovery argument)
  sed_wrap -i -e '/- "discovery"/ a\
{{- if .Values.pilot.extraArgs }}\
  {{-  range .Values.pilot.extraArgs }}\
          - {{ . | quote }}\
  {{- end }}\
{{- end }}'  "${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml"

}

function hacks() {
  sed_wrap -i -e '/containers:/,/name: discovery/ {
      /name: discovery/a\
\          workingDir: "/"
    }' "${HELM_DIR}/istio-control/istio-discovery/templates/deployment.yaml"
}

copyOverlay
removeUnsupportedCharts
patchTemplates
patchGalley
patchGateways
moveEnvoyFiltersToMeshConfigChart
copyGlobalValues
patchPilotServingCert
patchCNI
# TODO: remove this hack once the image is updated to include workingDir
hacks
