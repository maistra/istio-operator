#!/usr/bin/env bash

set -e

: ${HELM_DIR:?"Need to set HELM_DIR to output location for charts, e.g. tmp/_output/istio-releases/istio-1.1.0"}
: ${SOURCE_DIR:?"Need to set SOURCE_DIR to location of the istio-operator source directory"}

: ${THREESCALE_VERSION:=0.7.1}
: ${KIALI_VERSION:=1.0.0}

if [[ "${COMMUNITY,,}" == "true" ]]; then
  : ${HUB:=docker.io/maistra}
else
  : ${HUB:=registry.redhat.io/openshift-service-mesh}
fi

# copy maistra specific templates into charts
function copyOverlay() {
  echo "copying Maistra chart customizations over stock Istio charts"
  find "${SOURCE_DIR}/helm/" -maxdepth 1 -mindepth 1 -type d | xargs -I '{}' -n 1 -rt cp -r '{}' ${HELM_DIR}
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
  echo "patching Helm charts"

  # Webhooks are not namespaced!  we do this to ensure we're not setting owner references on them
  sed -i -e '/metadata:/,/webhooks:/ { /namespace/d }' \
    ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/mutatingwebhookconfiguration.yaml.tpl \
    ${HELM_DIR}/istio/charts/galley/templates/validatingwebhookconfiguration.yaml.tpl

  # update global defaults
  # disable autoInject
  # enable grafana, tracing and kiali, by default
  # set dnsRefreshRate to 300s
  sed -i -e 's/autoInject:.*$/autoInject: disabled/' \
         -e '/grafana:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/tracing:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/kiali:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/dnsRefreshRate:/ { s/5s/300s/ }' ${HELM_DIR}/istio/values.yaml

  # enable all namespaces by default
  sed -i -e 's/enableNamespacesByDefault:.*$/enableNamespacesByDefault: true/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml

  # enable egressgateway
  sed -i -e '/istio-egressgateway:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/istio-ingressgateway:/,/^[^ ]/ {
                s/type:.*$/type: ClusterIP/
                /ports:/,/meshExpansionPorts:/ {
                  /nodePort/ d
                  /port: 31400/,+1 d
                  /port: 15029/,+2 d
                  /port: 15030/,+2 d
                  /port: 15031/,+2 d
                  /port: 15032/,+2 d
                }
             }' ${HELM_DIR}/istio/charts/gateways/values.yaml
  # add support for IOR
  sed -i -e '/istio-ingressgateway:/,/enabled:/ {
    /enabled:/ a\
\  # set to true to enable route creation\
\  ior_enabled: false\
\  ior_image: istio-ior-rhel8\

  }' ${HELM_DIR}/istio/charts/gateways/values.yaml
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's/ior_image:.*$/ior_image: istio-ior-ubi8/' ${HELM_DIR}/istio/charts/gateways/values.yaml
  fi

  # enable ingress for tracing
  sed -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/tracing/values.yaml

  # enable ingress for kaili
  # update hub/tag
  sed -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/kiali/values.yaml
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's/hub:.*$/hub: kiali/' \
           -e 's/tag:.*$/tag: v'${KIALI_VERSION}'/' ${HELM_DIR}/istio/charts/kiali/values.yaml
  else
    sed -i -e 's+hub:.*$+hub: registry\.redhat\.io\/openshift-service-mesh+' \
           -e 's/tag:.*$/tag: '${KIALI_VERSION}'/' ${HELM_DIR}/istio/charts/kiali/values.yaml
  fi

  # In Istio 1.2, this viewOnlyMode is there, but in Istio 1.1 we need to add it - it is supported by the latest Kiali
  sed -i -e 's/grafanaURL:/viewOnlyMode: false\n  grafanaURL:/' ${HELM_DIR}/istio/charts/kiali/values.yaml

  # - remove the cleanup secrets job, we handle this in the installer
  rm ${HELM_DIR}/istio/charts/security/templates/cleanup-secrets.yaml

  # - we create custom resources in the normal way
  if [ "${PATCH_1_0}" == "" ]; then
    rm ${HELM_DIR}/istio/charts/security/templates/create-custom-resources-job.yaml
    rm ${HELM_DIR}/istio/charts/security/templates/configmap.yaml

    # now make sure they're available
    sed -i -e 's/define "security-default\.yaml\.tpl"/if and .Values.createMeshPolicy .Values.global.mtls.enabled/' ${HELM_DIR}/istio/charts/security/templates/enable-mesh-mtls.yaml
    sed -i -e 's/define "security-permissive\.yaml\.tpl"/if and .Values.createMeshPolicy (not .Values.global.mtls.enabled)/' ${HELM_DIR}/istio/charts/security/templates/enable-mesh-permissive.yaml
  fi

  # - remove the kubernetes gateways
  # this no longer exists
  # rm ${HELM_DIR}istio/charts/pilot/templates/gateway.yaml

  # - remove GODEBUG from the pilot environment (and mixer too)
  sed -i -e '/GODEBUG/d' ${HELM_DIR}/istio/charts/pilot/values.yaml ${HELM_DIR}/istio/charts/mixer/values.yaml

  # - change privileged value on istio-proxy injection configmap to false
  # setting the proper values will fix this:
  # global.proxy.privileged=false
  # global.proxy.enableCoreDump=false
  # however, we need to ensure privileged is set for istio_init
  sed -i -e '/- name: istio-init/,/- name: enable-core-dump/ {
    /- NET_ADMIN/,+3 {
      /{{/d
    }
  }' ${HELM_DIR}/istio/templates/sidecar-injector-configmap.yaml

  # add annotation for Multus & Istio CNI
  sed -i -e 's/^\(.*template:.*\)$/\1\
    \{\{- if .Values.istio_cni.enabled \}\}\
      annotations:\
        k8s.v1.cni.cncf.io\/networks: istio-cni\
    \{\{- end \}\}/' ${HELM_DIR}/istio/templates/sidecar-injector-configmap.yaml

  # allow the sidecar injector to set the runAsUser ID dynamically
  # drop unneeded capabilities from sidecar container, so using the restricted SCC doesn't require the SCC admission controller to mutate the pod
  sed -i -e 's/^\(.*runAsUser: 1337.*\)$/\
          capabilities:\
            drop:\
            - KILL\
            - SETUID\
            - SETGID\
            - MKNOD/' ${HELM_DIR}/istio/templates/sidecar-injector-configmap.yaml

  # - update the namespaceSelector to ignore namespaces with the label maistra.io/ignore-namespace
  # set sidecarInjectorWebhook.enableNamespacesByDefault=true
  sed -i -e '/if \.Values\.enableNamespacesByDefault/,/else/ {
    s/istio-injection/maistra.io\/ignore-namespace/
    s/NotIn/DoesNotExist/
    /values/d
    /disabled/d
    /else/ i\
\      - key: istio.openshift.com/ignore-namespace\
\        operator: DoesNotExist
  }' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/mutatingwebhookconfiguration.yaml.tpl

  # - add a maistra-version label to all objects which have a release label
  find ${HELM_DIR} -name "*.yaml" -o -name "*.yaml.tpl" | \
    xargs sed -i -e 's/^\(.*\)release:\(.*\)$/\1maistra-version: '${MAISTRA_VERSION}'\
\1release:\2/'

  # MAISTRA-506 add a maistra-control-plane label for deployment specs
  find ${HELM_DIR} -name "*.yaml" -o -name "*.yaml.tpl" | xargs grep -Hl 'kind: Deployment' |\
    xargs sed -i -e '/^spec:/,$ { /template:$/,$ { /metadata:$/,$ { /labels:$/,$ s/^\(.*\)release:\(.*Name\)\(.*\)$/\1maistra-control-plane:\2space }}\n\1release:\2\3/ } } }'

  # update the images
  # set global.hub=docker.io/maistra
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's+hub:.*$+hub: '${HUB}'+g' \
          -e 's/tag:.*$/tag: '${MAISTRA_VERSION}'/' \
          -e 's/image: *proxy_init/image: proxy-init-ubi8/' \
          -e 's/image: *proxyv2/image: proxyv2-ubi8/' ${HELM_DIR}/istio/values.yaml ${HELM_DIR}/istio-init/values.yaml

    sed -i -e 's/image: *galley/image: galley-ubi8/' ${HELM_DIR}/istio/charts/galley/values.yaml
    sed -i -e 's/image: *sidecar_injector/image: sidecar-injector-ubi8/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml
    sed -i -e 's/image: *mixer/image: mixer-ubi8/' ${HELM_DIR}/istio/charts/mixer/values.yaml
    sed -i -e 's/image: *pilot/image: pilot-ubi8/' ${HELM_DIR}/istio/charts/pilot/values.yaml
    sed -i -e 's/image: *citadel/image: citadel-ubi8/' ${HELM_DIR}/istio/charts/security/values.yaml
    sed -i -e 's/image: *istio-cni/image: istio-cni-ubi8/' ${HELM_DIR}/istio_cni/values.yaml
    sed -i -e 's|\(^jaeger:.*$\)|elasticsearch:\
  hub: registry.centos.org/rhsyseng\
  image: elasticsearch\
  tag: 5.6.10\
\
\1|' ${HELM_DIR}/istio/charts/tracing/values.yaml
    sed -i -e 's/tag:.*$/tag: v'${THREESCALE_VERSION}'/' ${HELM_DIR}/maistra-threescale/values.yaml
  else
    sed -i -e 's+hub:.*$+hub: '${HUB}'+g' \
          -e 's/tag:.*$/tag: '${MAISTRA_VERSION}'/' \
          -e 's/image: *proxy_init/image: proxy-init-rhel8/' \
          -e 's/image: *proxyv2/image: proxyv2-rhel8/' ${HELM_DIR}/istio/values.yaml ${HELM_DIR}/istio-init/values.yaml
    sed -i -e 's/image: *galley/image: galley-rhel8/' ${HELM_DIR}/istio/charts/galley/values.yaml
    sed -i -e 's/image: *sidecar_injector/image: sidecar-injector-rhel8/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml
    sed -i -e 's/image: *mixer/image: mixer-rhel8/' ${HELM_DIR}/istio/charts/mixer/values.yaml
    sed -i -e 's/image: *pilot/image: pilot-rhel8/' ${HELM_DIR}/istio/charts/pilot/values.yaml
    sed -i -e 's/image: *citadel/image: citadel-rhel8/' ${HELM_DIR}/istio/charts/security/values.yaml
    sed -i -e 's/image: *istio-cni/image: istio-cni-rhel8/' ${HELM_DIR}/istio_cni/values.yaml
    sed -i -e 's|\(^jaeger:.*$\)|elasticsearch:\
  hub: registry.centos.org/rhsyseng\
  image: elasticsearch\
  tag: 5.6.10\
\
\1|' ${HELM_DIR}/istio/charts/tracing/values.yaml
    sed -i -e 's/hub:.*$/hub: registry\.redhat\.io\/openshift-service-mesh/' \
           -e 's/tag:.*$/tag: '${THREESCALE_VERSION}'/' ${HELM_DIR}/maistra-threescale/values.yaml
  fi

  # - remove istio-multi service account
  rm ${HELM_DIR}/istio/templates/serviceaccount.yaml
  # - remove istio-multi cluster role binding
  rm ${HELM_DIR}/istio/templates/clusterrolebinding.yaml
  # - remove istio-reader cluster role
  rm ${HELM_DIR}/istio/templates/clusterrole.yaml

  # - switch webhook ports to 8443: add targetPort name to galley service
  # XXX: move upstream (add targetPort name)
  sed -i -e 's/^\(.*\)\(- port: 443.*\)$/\1\2\
\1  targetPort: webhook/' ${HELM_DIR}/istio/charts/galley/templates/service.yaml

  # - switch webhook ports to 8443
  # add name to webhook port (XXX: move upstream)
  # change the location of the healthCheckFile from /health to /tmp/health
  # add --validation-port=8443
  sed -i -e 's/^\(.*\)\- containerPort: 443.*$/\1- name: webhook\
\1  containerPort: 8443/' \
           -e 's/\/health/\/tmp\/health/' \
           -e 's/^\(.*\)\(- --monitoringPort.*\)$/\1\2\
\1- --validation-port=8443/' ${HELM_DIR}/istio/charts/galley/templates/deployment.yaml

  # - switch webhook ports to 8443
  # XXX: move upstream (add targetPort name)
  sed -i -e 's/^\(.*\)\(- port: 443.*\)$/\1\2\
\1  targetPort: webhook/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/service.yaml

  # - switch webhook ports to 8443
  # - disable management of webhook config
  # - add webhook port
  sed -i -e 's/^\(.*\)\(volumeMounts:.*\)$/\1  - --port=8443\
\1  - --manageWebhookConfig=false\
\1ports:\
\1- name: webhook\
\1  containerPort: 8443\
\1\2/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml

  # change the location of the healthCheckFile from /health to /tmp/health
  if [[ "${COMMUNITY,,}" != "true" ]]; then
    sed -i -e 's/\/health/\/tmp\/health/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml
  fi

  # Fix for MAISTRA-334, can be removed when we move to Istio-1.2
  sed -i '/match: (context.protocol == "http" || context.protocol == "grpc")/ s/$/ \&\& (match((request.useragent | "-"), "Prometheus*") == false)/' ${HELM_DIR}/istio/charts/mixer/templates/config.yaml 
}

# The following modifications are made to the generated helm template for the Kiali yaml file
# - remove all non-kiali operator configuration
function patchKialiTemplate() {
  echo "patching Kiali specific Helm charts"

  # we are using kiali operator, no need for the other templates
  for yaml in demosecret clusterrolebinding clusterrole configmap deployment ingress serviceaccount service
  do
    rm "${HELM_DIR}/istio/charts/kiali/templates/${yaml}.yaml"
  done
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
  sed -i -e 's/^\(\( *\)kind.*'$2'.*$\)/\2kind: '$3'/' \
         -e '0,/name:/ s/^\(\(.*\)name:.*$\)/\1\
\2namespace: {{ '$4'.Release.Namespace }}/' "${1}"
}

function convertClusterRoleBinding() {
  convertClusterToNamespaced "$1" "ClusterRoleBinding" "RoleBinding" "$2"
}

function convertMeshPolicy() {
  convertClusterToNamespaced "$1" "MeshPolicy" "Policy" "$2"
}

function patchMultiTenant() {
  echo "Patching charts for multitenancy"

  # galley
  sed -i -e '/apiGroups:.*admissionregistration/,/apiGroups/ {
    /admissionregistration/ {
      i\
- apiGroups: ["maistra.io"]\
\  resources: ["servicemeshmemberrolls"]\
\  verbs: ["get", "list", "watch"]
      d
    }
    /apiGroups/!d
  }' ${HELM_DIR}/istio/charts/galley/templates/clusterrole.yaml
  sed -i -e 's/, *"nodes"//' ${HELM_DIR}/istio/charts/galley/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/galley/templates/clusterrolebinding.yaml
  sed -i -e '/--validation-webhook-config-file/ {
    s/^\(\( *\)- --validation-webhook-config-file\)/\2- --deployment-namespace\
\2- \{\{ .Release.Namespace \}\}\
\2- --webhook-name\
\2- istio-galley-\{\{ .Release.Namespace \}\}\
\2- --memberRollName=default\
\2- --manageWebhookConfig=false\
\1/
  }' ${HELM_DIR}/istio/charts/galley/templates/deployment.yaml

  # gateways
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/gateways/templates/clusterrolebindings.yaml "$"

  # mixer
  sed -i -e '/apiGroups:.*apiextensions.k8s.io/,/apiGroups:/ {
    /apiextensions/ {
      i\
- apiGroups: ["maistra.io"]\
\  resources: ["servicemeshmemberrolls"]\
\  verbs: ["get", "list", "watch"]
      d
    }
    /apiGroups/!d
  }'  ${HELM_DIR}/istio/charts/mixer/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/mixer/templates/clusterrolebinding.yaml
  sed -i -e '/name: *mixer/,/args:/ {
    /args/ a\
\          - --memberRollName=default\
\          - --memberRollNamespace=\{\{ .Release.Namespace \}\}
  }' ${HELM_DIR}/istio/charts/mixer/templates/deployment.yaml

  # pilot
  sed -i -e '/apiGroups:.*apiextensions.k8s.io/,/apiGroups:/ {
    /apiextensions/ {
      i\
- apiGroups: ["maistra.io"]\
\  resources: ["servicemeshmemberrolls"]\
\  verbs: ["get", "list", "watch"]
      d
    }
    /apiGroups/!d
  }' \
         -e 's/, *"nodes"//' ${HELM_DIR}/istio/charts/pilot/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/pilot/templates/clusterrolebinding.yaml
  sed -i -r -e 's/^(( *)- "?discovery"?)/\1\
\2- --memberRollName=default/' ${HELM_DIR}/istio/charts/pilot/templates/deployment.yaml

  # security
  sed -i -e '/apiGroups:.*authentication.k8s.io/,$ {
    /apiGroups/ i\
- apiGroups: ["maistra.io"]\
\  resources: ["servicemeshmemberrolls"]\
\  verbs: ["get", "list", "watch"]
    d
  }' ${HELM_DIR}/istio/charts/security/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/security/templates/clusterrolebinding.yaml
  # revisit in TP12
  #convertMeshPolicy ${HELM_DIR}/istio/charts/security/templates/enable-mesh-mtls.yaml
  #convertMeshPolicy ${HELM_DIR}/istio/charts/security/templates/enable-mesh-permissive.yaml
  sed -i -e 's/^\(\( *\){.*if .Values.global.trustDomain.*$\)/\
\            - --member-roll-name=default\
\1/' ${HELM_DIR}/istio/charts/security/templates/deployment.yaml

  # sidecarInjectorWebhook
  sed -i -e '/apiGroups:.*admissionregistration.k8s.io/,/apiGroups:/ {
    /admissionregistration/d
    /apiGroups/!d
  }' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/clusterrolebinding.yaml
  sed -i -e '/args:/ a\
            - --webhookConfigName=istio-sidecar-injector-{{ .Release.Namespace }}' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml
}

function removeUnsupportedCharts() {
  rm -rf ${HELM_DIR}/istio/charts/nodeagent
  rm -rf ${HELM_DIR}/istio/charts/servicegraph
  rm -rf ${HELM_DIR}/istio/charts/istiocoredns
  rm -rf ${HELM_DIR}/istio/charts/certmanager

  sed -i -e '/name:.*nodeagent/,+2 d' \
         -e '/name:.*servicegraph/,+2 d' \
         -e '/name:.*istiocoredns/,+2 d' \
         -e '/name:.*certmanager/,+2 d' ${HELM_DIR}/istio/requirements.yaml
}


copyOverlay

removeUnsupportedCharts
patchTemplates
patchKialiTemplate
patchKialiOpenShift

patchMultiTenant
source ${SOURCE_DIR}/tmp/build/patch-grafana.sh
source ${SOURCE_DIR}/tmp/build/patch-jaeger.sh
source ${SOURCE_DIR}/tmp/build/patch-prometheus.sh
