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
  for file in $(find "${HELM_DIR}" -name "*.yaml" ! -name "values.yaml" ! -name "kube-gateway.yaml" -print0 -o -name "*.yaml.tpl" | xargs -0 grep -Hl '^kind: Deployment'); do
    sed_wrap -i -e '/^spec:/,$ { /template:$/,$ { /metadata:$/,$ { s/^\(.*\)labels:/\1labels:\
\1  maistra-control-plane: {{ .Release.Namespace }}/ } } }' "$file"
  done

  # OSSM-5312: add maistra-control-plane and maistra-version to kube-gateway template, which is populated by Gateway deployment controller.
  # Patch gateway deployment
  sed_wrap -i -e 's/{{- toJsonMap .Labels | nindent 4 }}/{{- toJsonMap (strdict "maistra-version" "'"${MAISTRA_VERSION}"'").Labels | nindent 4}}/' "${HELM_DIR}/istio-control/istio-discovery/files/kube-gateway.yaml"
  sed_wrap -i -e '/"service.istio.io\/canonical-revision" "latest"/ a\
            "maistra-control-plane" .Values.global.istioNamespace\
            "maistra-version" "'"${MAISTRA_VERSION}"'"' "${HELM_DIR}/istio-control/istio-discovery/files/kube-gateway.yaml"
  # Patch gateway service
  sed_wrap -i -e 's/{{ toJsonMap .Labels | nindent 4}}/{{ toJsonMap\
      (strdict\
        "maistra-control-plane" .Values.global.istioNamespace\
        "maistra-version" "'"${MAISTRA_VERSION}"'"\
      ).Labels | nindent 4}}/' "${HELM_DIR}/istio-control/istio-discovery/files/kube-gateway.yaml"

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

  # - nuke unnecessary files from base
  rm -rf "${HELM_DIR}/base/templates" "${HELM_DIR}/base/files" "${HELM_DIR}/base/crds/crd-operator.yaml"

  # rename base/crds to istio-init/files
  mv "${HELM_DIR}"/base/crds/* "${HELM_DIR}/istio-init/files"
  rm -rf "${HELM_DIR}/base"
  CRD_DIR="${HELM_DIR}/istio-init/files" "${SOURCE_DIR}/build/split-istio-crds.sh"

  # MAISTRA-1972 - disable protocol sniffing
  sed_wrap -i -e 's/\(enableProtocolSniffing.*:\).*$/\1 false/' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"


  # - add a maistra-version label to all objects which have a release label
  # do this after we've separated crds
  # shellcheck disable=SC2044
  for file in $(find "${HELM_DIR}" -name "*.yaml" ! -name "kube-gateway.yaml" -o -name "*.yaml.tpl"); do
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
    if grep -l '\.Values\.global\.istioNamespace' "$file"; then
      sed_wrap -i -e 's/\.Values\.global\.istioNamespace/.Release.Namespace/' "$file"
    fi
    if grep -l -e '{{- if not (eq .Values.revision "") *}}-{{ .Values.revision }}{{- end }}' "$file"; then
      sed_wrap -i -e 's/{{- if not (eq .Values.revision "") *}}-{{ .Values.revision }}{{- end }}/-{{ .Values.revision | default "default" }}/' "$file"
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
  sed_wrap -i '/- apiGroups:.*admissionregistration\.k8s\.io/,/verbs:/ {
     /- apiGroups:.*admissionregistration\.k8s\.io/ i\
{{ if .Values.global.clusterWide }}
     /verbs:/ a\
{{ end }}
    }'  "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml"
  sed_wrap -i -e '/- apiGroups:.*certificates\.k8s\.io/,/verbs:/ d' "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml"
  sed_wrap -i -e '/- apiGroups:.*authentication\.k8s\.io/,/verbs:/ d' "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrole.yaml"

  # reader-clusterrole and reader-clusterrolebinding added, if clusterWide and multiCluster mode enabled
  sed_wrap -i '1i\{{- if and .Values.global.clusterWide .Values.global.multiCluster.enabled }}' "${HELM_DIR}/istio-control/istio-discovery/templates/reader-clusterrole.yaml"  
  sed_wrap -i '$ a\{{- end }}' "${HELM_DIR}/istio-control/istio-discovery/templates/reader-clusterrole.yaml" 

  sed_wrap -i '1i\{{- if and .Values.global.clusterWide .Values.global.multiCluster.enabled }}' "${HELM_DIR}/istio-control/istio-discovery/templates/reader-clusterrolebinding.yaml"  
  sed_wrap -i '$ a\{{- end }}' "${HELM_DIR}/istio-control/istio-discovery/templates/reader-clusterrolebinding.yaml"  
 

  convertClusterRoleBinding "${HELM_DIR}/istio-control/istio-discovery/templates/clusterrolebinding.yaml" ""
  sed_wrap -i -e '/- "discovery"/ a\
{{- if not .Values.global.clusterWide }}\
          - --memberRollName=default\
          - --enableNodeAccess=false\
          - --enableCRDScan=false\
          - --enableIngressClassName=false\
{{- end}}' "${deployment}"

  ############## disable webhook config updates ############################
  # Name of the mutatingwebhookconfiguration to patch, if istioctl is not used.
  sed_wrap -i -e '/env:/ a\
          - name: INJECTION_WEBHOOK_CONFIG_NAME\
            value: ""' "${deployment}"

  # Name of validatingwebhookconfiguration to patch.
  # Empty will skip using cluster admin to patch.
  sed_wrap -i -e '/env:/ a\
          - name: VALIDATION_WEBHOOK_CONFIG_NAME\
            value: ""' "${deployment}"

  # Disable PRIORITIZED_LEADER_ELECTION so that MutatingWebhookConfigurations aren't watched
  sed_wrap -i -e '/env:/ a\
          - name: PRIORITIZED_LEADER_ELECTION\
            value: "false"' "${deployment}"
  ##############################################################################

  # remove privileged security settings
  sed_wrap -i -r -e '/template:/,/containers:/ { /securityContext:/,/containers:/ { /containers:/! d }}' \
      -e '/containers:/,$ { /securityContext:/,/capabilities:/ { /capabilities:|securityContext:/! d }}' "${deployment}"

  sed_wrap -i -e '/base:/ a\
  validationURL: ""' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  # always istiod
  sed_wrap -i -e '/{{- if ne .Values.revision ""}}/,/{{- end }}/d' \
      -e '/matchLabels:/a\
\      app: istiod\
\      istio.io/rev: {{ .Values.revision | default "default" }}' "${deployment}"

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

  sed_wrap -i -e '/base:/ i\
gateways: {}\n' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

  sed_wrap -i -e '/sidecarInjectorWebhook:/ a\
  objectSelector:\
    enabled: false\n' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"

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
  echo "move helpers file to gateways directory"
  cp "${HELM_DIR}"/istio-control/istio-discovery/templates/_helpers.tpl "${HELM_DIR}/gateways/istio-ingress/templates"
  cp "${HELM_DIR}"/istio-control/istio-discovery/templates/_helpers.tpl "${HELM_DIR}/gateways/istio-egress/templates"

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

  echo "
telemetry:
  enabled: true
  v2:
    enabled: true
    prometheus:
      enabled: true
      configOverride:
        gateway: {}
        inboundSidecar: {}
        outboundSidecar: {}
    stackdriver:
      enabled: false
      configOverride: {}
      disableOutbound: false" | \
  tee >(cat >> "${HELM_DIR}/gateways/istio-ingress/values.yaml")\
      >(cat >> "${HELM_DIR}/gateways/istio-egress/values.yaml")

  sed_wrap -i -e '/  logLevel:/ a\\n    tracer: zipkin' "${HELM_DIR}/gateways/istio-ingress/values.yaml"
  sed_wrap -i -e '/  logLevel:/ a\\n    tracer: zipkin' "${HELM_DIR}/gateways/istio-egress/values.yaml"

  sed_wrap -i -e '/  priorityClassName:/ a\
\
  # Whether to enable the Kubernetes Ingress feature.\
  k8sIngress:\
    enabled: false' "${HELM_DIR}/gateways/istio-ingress/values.yaml"

  # Disable defaultTemplates to avoid injection of arbitrary things
  sed_wrap -i -e 's/defaultTemplates: \[\]/\# defaultTemplates: \[\]/' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"
  sed_wrap -i -e '/{{- if .Values.sidecarInjectorWebhook.defaultTemplates }}/,+7d' "${HELM_DIR}/istio-control/istio-discovery/templates/istiod-injector-configmap.yaml"
  sed_wrap -i -e '/policy:/ i\
    defaultTemplates: [sidecar]
' "${HELM_DIR}/istio-control/istio-discovery/templates/istiod-injector-configmap.yaml"

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

  # MAISTRA-2528 Enable DNS Capture for proxies by default
  # MAISTRA-2656 Fix missing DNS registry entries in istio-agent
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

  # parameterize runAsUser, runAsGroup, ans fsGroup
  # shellcheck disable=SC2016
  sed_wrap -i -E -e 's/(runAsUser|runAsGroup|fsGroup): 1337/\1: {{ $gateway.\1 }}/' \
    "${HELM_DIR}/gateways/istio-ingress/templates/deployment.yaml" \
    "${HELM_DIR}/gateways/istio-egress/templates/deployment.yaml" \
    "${HELM_DIR}/gateways/istio-egress/templates/injected-deployment.yaml" \
    "${HELM_DIR}/gateways/istio-ingress/templates/injected-deployment.yaml"

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

function patchSidecarInjector() {
  echo "patching Sidecar Injector specific Helm charts"
  # Instead of patching mutatingwebhook.yaml, we now overlay it.

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
  }' "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"

  sed_wrap -i -E -e 's/(^ *)(runAsUser: )1337/\1{{- if .ProxyUID }}\n\1\2{{ .ProxyUID }}\n\1{{- end }}/g' \
    -e 's/(^ *)(runAsGroup: )1337/\1{{- if .ProxyGID }}\n\1\2{{ .ProxyGID }}\n\1{{- end }}/g' \
    -e 's/(^ *)(fsGroup: )1337/\1{{- if .ProxyGID }}\n\1\2{{ .ProxyGID }}\n\1{{- end }}/g' "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"
  sed_wrap -i -E -e 's/(^ *)(fsGroup: )1337/\1{{- if .ProxyGID }}\n\1\2{{ .ProxyGID }}\n\1{{- end }}/g' "${HELM_DIR}/istio-control/istio-discovery/files/gateway-injection-template.yaml"

  sed_wrap -i -e '/- name: istio-proxy/,/resources:/ {
    / *- ALL/a\
        - KILL\
        - MKNOD\
        - SETGID\
        - SETUID
  }' "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"

  # replace 'default' in CA_ADDR spec to use valueOrDefault
  sed_wrap -i -e 's/value: istiod-{{ .Values.revision | default "default" }}.*$/value: istiod-{{ valueOrDefault .Values.revision "default" }}.{{ .Values.global.istioNamespace }}.svc:15012/' \
      "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml" \
      "${HELM_DIR}/istio-control/istio-discovery/files/gateway-injection-template.yaml"

  # never apply init container, even for validation
  sed_wrap -i -e '/^  initContainers:/,/^  containers:/ {/^  containers:/!d}' \
      "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"
  # use the correct cni network defintion
  # shellcheck disable=SC2016
  sed_wrap -i -e '/annotations:/,$s/`istio-cni`/.Values.istio_cni.istio_cni_network/' \
      "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"
  # status port is incorrect
  sed_wrap -i -e 's/statusPort: 15020$/statusPort: 15021/' "${HELM_DIR}/istio-control/istio-discovery/values.yaml"
  # exclude 15090 from inbound ports
  # shellcheck disable=SC2016
  sed_wrap -i -e 's$traffic.sidecar.istio.io/excludeInboundPorts: "{{ excludeInboundPort (annotation .ObjectMeta `status.sidecar.istio.io/port` .Values.global.proxy.statusPort) (annotation .ObjectMeta `traffic.sidecar.istio.io/excludeInboundPorts` .Values.global.proxy.excludeInboundPorts) }}"$traffic.sidecar.istio.io/excludeInboundPorts: "15090,{{ excludeInboundPort (annotation .ObjectMeta `status.sidecar.istio.io/port` .Values.global.proxy.statusPort) (annotation .ObjectMeta `traffic.sidecar.istio.io/excludeInboundPorts` .Values.global.proxy.excludeInboundPorts) }}"$' \
      "${HELM_DIR}/istio-control/istio-discovery/files/injection-template.yaml"
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

function patchGrafanaTemplate() {
  # add extra values
  sed_wrap -i -e '/passphraseKey:/ a\
  persistenceResources:\
    requests:\
      storage: "5Gi" # PVC spec.resources.requests.storage default value' "${HELM_DIR}/istio-telemetry/grafana/values.yaml"

  # add configurable capacity
  sed_wrap -i -e 's/5Gi/{{\ .Values.grafana.persistenceResources.requests.storage\ \|\ quote\ }}/' "${HELM_DIR}/istio-telemetry/grafana/templates/pvc.yaml"
}

function convertClusterToNamespaced() {
  # $1 - file to convert
  # $2 - cluster kind
  # $3 - namespaced kind
  # $4 - dereference
  sed_wrap -i -e 's/^\(\( *\)kind.*'"$2"'.*$\)/\2kind: {{ if .Values.global.clusterWide }}'"$2"'{{ else }}'"$3"'{{ end }}/' "${1}"
  sed_wrap -i -e '/metadata:/ a\
  {{ if not .Values.global.clusterWide }}namespace: {{ '"$4"'.Release.Namespace }}{{ end }}' "${1}"
}

function convertClusterRoleBinding() {
  convertClusterToNamespaced "$1" "ClusterRoleBinding" "RoleBinding" "$2"
}

function removeUnsupportedCharts() {
  echo "removing unsupported Helm charts"
  rm -rf "${HELM_DIR}/istio-cni"
  rm -rf "${HELM_DIR}/istiocoredns"
  rm -rf "${HELM_DIR}/istiod-remote"
  rm -rf "${HELM_DIR}/istio-operator"
  rm -rf "${HELM_DIR}/ztunnel"
}

function moveEnvoyFiltersToMeshConfigChart() {
  echo "moving EnvoyFilter manifests to mesh-config"
  mv "${HELM_DIR}"/istio-control/istio-discovery/templates/telemetry*.yaml "${HELM_DIR}/mesh-config/templates"
  cp "${HELM_DIR}"/istio-control/istio-discovery/templates/_helpers.tpl "${HELM_DIR}/mesh-config/templates"

  sed_nowrap -n -e '/^telemetry:/,/^      logWindowDuration/ p' "${HELM_DIR}/istio-control/istio-discovery/values.yaml" > "${HELM_DIR}/mesh-config/values.yaml"
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
  cp "${SOURCE_DIR}/resources/helm/overlays/global.yaml" "${SOURCE_DIR}/resources/helm/v2.5/"
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
patchSidecarInjector
moveEnvoyFiltersToMeshConfigChart
copyGlobalValues
patchPilotServingCert
patchGrafanaTemplate
# TODO: remove this hack once the image is updated to include workingDir
hacks
