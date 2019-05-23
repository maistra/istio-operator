#!/usr/bin/env bash

set -e

: ${HELM_DIR:?"Need to set HELM_DIR to output location for charts, e.g. tmp/_output/istio-releases/istio-1.1.0"}
: ${SOURCE_DIR:?"Need to set SOURCE_DIR to location of the istio-operator source directory"}

: ${THREESCALE_VERSION:=0.6.0}
: ${KIALI_VERSION:=0.20.0}

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
  sed -i -e 's/autoInject:.*$/autoInject: disabled/' \
         -e '/grafana:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/tracing:/,/enabled/ { s/enabled: .*$/enabled: true/ }' \
         -e '/kiali:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/values.yaml

  # enable all namespaces by default
  sed -i -e 's/enableNamespacesByDefault:.*$/enableNamespacesByDefault: true/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml

  # enable egressgateway
  sed -i -e '/istio-egressgateway:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/gateways/values.yaml
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

  # enable ingress for grafana
  sed -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/grafana/values.yaml

  # enable ingress for tracing
  sed -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/tracing/values.yaml

  # enable ingress for kaili
  # update hub/tag
  sed -i -e '/ingress:/,/enabled/ { s/enabled: .*$/enabled: true/ }' ${HELM_DIR}/istio/charts/kiali/values.yaml
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's/hub:.*$/hub: kiali/' \
           -e 's/tag:.*$/tag: v'${KIALI_VERSION}'/' ${HELM_DIR}/istio/charts/kiali/values.yaml
  else
    sed -i -e 's+hub:.*$+hub: openshift-istio-tech-preview+' \
           -e 's/tag:.*$/tag: '${KIALI_VERSION}'/' ${HELM_DIR}/istio/charts/kiali/values.yaml
  fi

  # - remove the create customer resources job, we handle this in the installer to deal with potential races
  rm ${HELM_DIR}/istio/charts/grafana/templates/create-custom-resources-job.yaml

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

  # update the images
  # set global.hub=docker.io/maistra
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's+hub:.*$+hub: docker.io/maistra+g' \
          -e 's/tag:.*$/tag: '${MAISTRA_VERSION}'/' \
          -e 's/image: *proxy_init/image: proxy-init-ubi8/' \
          -e 's/image: *proxyv2/image: proxyv2-ubi8/' ${HELM_DIR}/istio/values.yaml ${HELM_DIR}/istio-init/values.yaml
    sed -i -e 's/image: *galley/image: galley-ubi8/' ${HELM_DIR}/istio/charts/galley/values.yaml
    sed -i -e 's/image: *sidecar_injector/image: sidecar-injector-ubi8/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml
    sed -i -e 's/image: *mixer/image: mixer-ubi8/' ${HELM_DIR}/istio/charts/mixer/values.yaml
    sed -i -e 's/image: *pilot/image: pilot-ubi8/' ${HELM_DIR}/istio/charts/pilot/values.yaml
    sed -i -e 's/image: *citadel/image: citadel-ubi8/' ${HELM_DIR}/istio/charts/security/values.yaml
    sed -i -e 's|\(^jaeger:.*$\)|elasticsearch:\
  hub: registry.centos.org/rhsyseng\
  image: elasticsearch\
  tag: 5.6.10\
\
\1|' ${HELM_DIR}/istio/charts/tracing/values.yaml
    sed -i -e 's/hub:.*$/hub: openshift-istio-tech-preview/' \
           -e 's/tag:.*$/tag: v'${THREESCALE_VERSION}'/' ${HELM_DIR}/maistra-threescale/values.yaml
  else
    sed -i -e 's+hub:.*$+hub: openshift-istio-tech-preview+g' \
          -e 's/tag:.*$/tag: '${MAISTRA_VERSION}'/' \
          -e 's/image: *proxy_init/image: proxy-init-rhel8/' \
          -e 's/image: *proxyv2/image: proxyv2-rhel8/' ${HELM_DIR}/istio/values.yaml ${HELM_DIR}/istio-init/values.yaml
    sed -i -e 's/image: *galley/image: galley-rhel8/' ${HELM_DIR}/istio/charts/galley/values.yaml
    sed -i -e 's/image: *sidecar_injector/image: sidecar-injector-rhel8/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/values.yaml
    sed -i -e 's/image: *mixer/image: mixer-rhel8/' ${HELM_DIR}/istio/charts/mixer/values.yaml
    sed -i -e 's/image: *pilot/image: pilot-rhel8/' ${HELM_DIR}/istio/charts/pilot/values.yaml
    sed -i -e 's/image: *citadel/image: citadel-rhel8/' ${HELM_DIR}/istio/charts/security/values.yaml
    sed -i -e 's|\(^jaeger:.*$\)|elasticsearch:\
  hub: registry.centos.org/rhsyseng\
  image: elasticsearch\
  tag: 5.6.10\
\
\1|' ${HELM_DIR}/istio/charts/tracing/values.yaml
    sed -i -e 's/hub:.*$/hub: openshift-istio-tech-preview/' \
           -e 's/tag:.*$/tag: '${THREESCALE_VERSION}'/' ${HELM_DIR}/maistra-threescale/values.yaml
  fi

  # - remove istio-multi service account
  rm ${HELM_DIR}/istio/templates/serviceaccount.yaml
  # - remove istio-multi cluster role binding
  rm ${HELM_DIR}/istio/templates/clusterrolebinding.yaml
  # - remove istio-reader cluster role
  rm ${HELM_DIR}/istio/templates/clusterrole.yaml

  # - switch prometheus init container image from busybox to prometheus
  sed -i -r -e 's/"?busybox:?.*$/"docker.io\/prom\/prometheus:v2.3.1"/' ${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml

  # - enable ingress (route) for prometheus
  sed -i -e '/ingress:/,/service:/ {
    s/enabled:.*$/enabled: true/
}' ${HELM_DIR}/istio/charts/prometheus/values.yaml

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
  # add webhook port
  sed -i -e 's/^\(.*\)\(volumeMounts:.*\)$/\1  - --port=8443\
\1ports:\
\1- name: webhook\
\1  containerPort: 8443\
\1\2/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml

  # change the location of the healthCheckFile from /health to /tmp/health
  if [[ "${COMMUNITY,,}" != "true" ]]; then
    sed -i -e 's/\/health/\/tmp\/health/' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml
  fi

}

# The following modifications are made to the generated helm template to extract the CRDs
# - remove all content except for the crd configmaps
# - add maistra-version labels
# all of this is done above in patchTemplates()

# The following modifications are made to the generated helm template for the Grafana yaml file
# - add a service account for grafana
# - remove all non grafana configuration
# - remove the extraneous create custom resources job
# - add the service account to the deployment
# - add a maistra-version label to all objects which have a release label (done in patchTemplates())
function patchGrafanaTemplate() {
  echo "patching Grafana specific Helm charts"

  # - add a service account for grafana
  # added a file to overlays

  # - remove the extraneous create custom resources job
  if [ -f ${HELM_DIR}/istio/charts/grafana/templates/create-custom-resources-job.yaml ]; then
    rm ${HELM_DIR}/istio/charts/grafana/templates/create-custom-resources-job.yaml
  fi

  # - custom resources will be installed directly
  if [ -f ${HELM_DIR}/istio/charts/grafana/templates/configmap-custom-resources.yaml ]; then
    rm ${HELM_DIR}/istio/charts/grafana/templates/configmap-custom-resources.yaml
  fi
  sed -i -e '/grafana-default.yaml.tpl/d' -e '/{{.*end.*}}/d' ${HELM_DIR}/istio/charts/grafana/templates/grafana-ports-mtls.yaml

  # - add the service account to the deployment
  sed -i -e 's/^\(.*\)containers:\(.*\)$/\1serviceAccountName: grafana\
\1containers:\2/' ${HELM_DIR}/istio/charts/grafana/templates/deployment.yaml
}

# patch tracing specific templates
function patchTracingtemplate() {
  echo "patching Jaeger (tracing) specific Helm charts"
  # update jaeger image hub
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's+hub: docker.io/jaegertracing+hub: jaegertracing+g' \
           -e 's+tag: 1.9+tag: 1.11+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  else
    sed -i -e 's+hub: docker.io/jaegertracing+hub: distributed-tracing-tech-preview+g' \
           -e 's+tag: 1.9+tag: 1.11.0+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  fi

  # update jaeger zipkin port name
  sed -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: jaeger-collector-zipkin/
}' ${HELM_DIR}/istio/charts/tracing/values.yaml

}

# The following modifications are made to the generated helm template for the Kiali yaml file
# - remove all non kiali configuration
# - remove the kiali username/password secret
function patchKialiTemplate() {
  echo "patching Kiali specific Helm charts"

  # - remove the kiali username/password secret
  rm ${HELM_DIR}/istio/charts/kiali/templates/demosecret.yaml
}

# The following modifications are made to the upstream kiali configuration for deployment on OpenShift
# - Add jaeger and grafana URLs to the configmap as well as the identity certs
# - Add the route.openshift.io api group to the cluster role
# - Add the openshift annotation to the service
# - Remove the prometheus, grafana environment from the deployment
# - Add the kiali-cert volume mount
# - Add the kiali-cert volume
# - Add istio namespace to the configmap
function patchKialiOpenShift() {
  echo "more patching of Kiali specific Helm charts"
  # - Add jaeger and grafana URLs to the configmap as well as the identity certs
  #   these should be defined in the values file kiali.dashboard.grafanaURL
  #   and kiali.dashboard.jaegerURL.  If not specified, the URLs should be
  #   determined from the routes created for Jaeger and Grafana.  This needs to
  #   be done after rendering, unfortunately.
  # - Add istio namespace to the configmap (for 1.0 templates)
  # - Add the kiali-cert volume mount
  # - Add the kiali-cert volume
  if [ -n "${PATCH_1_0}" ]; then
    sed -i -e '/server:/ i\
\    istio_namespace: {{ .Release.Namespace }}' ${HELM_DIR}/istio/charts/kiali/templates/configmap.yaml
  fi
  sed -i -e '/port: 20001/ a\
\      static_content_root_directory: /opt/kiali/console' \
         -e '/grafana:/,/url:/ {
             /url:/ a\
\{\{- if .Values.global.multitenant \}\}\
\    api:\
\      namespaces:\
\        label_selector: maistra.io/member-of=\{\{ .Release.Namespace \}\}\
\{\{- end \}\}\
\{\{- if not (and (.Values.dashboard.user) (.Values.dashboard.passphrase)) \}\}\
\    auth:\
\      strategy: openshift\
\{\{- end \}\}\
\    identity:\
\      cert_file: /kiali-cert/tls.crt\
\      private_key_file: /kiali-cert/tls.key
             }' ${HELM_DIR}/istio/charts/kiali/templates/configmap.yaml

  # - Add the route.openshift.io api group to the cluster role
  sed -i -e '/apiGroups:.*config.istio.io/ i\
\- apiGroups: ["project.openshift.io"]\
\  resources:\
\  - projects\
\  verbs:\
\  - get\
\- apiGroups: ["route.openshift.io"]\
\  resources:\
\  - routes\
\  verbs:\
\  - get\
\- apiGroups: [""]\
\  resources:\
\  - routes\
\  verbs:\
\  - get\
\- apiGroups: ["apps.openshift.io"]\
\  resources:\
\  - deploymentconfigs\
\  verbs:\
\  - get\
\  - list\
\  - watch' ${HELM_DIR}/istio/charts/kiali/templates/clusterrole.yaml

  # - Add the openshift annotation to the service
  sed -i -e '/metadata/ a\
\  annotations:\
\    service.alpha.openshift.io/serving-cert-secret-name: kiali-cert-secret' ${HELM_DIR}/istio/charts/kiali/templates/service.yaml

  # - Remove the prometheus, grafana environment from the deployment
  sed -i -e '/SERVER_CREDENTIALS_USERNAME/,/volumeMounts/ {
    /volumeMounts/b
    d
  }' ${HELM_DIR}/istio/charts/kiali/templates/deployment.yaml

  # - Add the kiali-cert volume mount
  # - Add the kiali-cert volume
  sed -i -e '/kind.*Deployment$/,/^.*affinity:/ {
    /volumeMounts:/ {
      N
      N
      a\
\        - name: kiali-cert\
\          mountPath: "/kiali-cert"
    }
    /configMap:/ {
      N
      a\
\      - name: kiali-cert\
\        secret:\
\          secretName: kiali-cert-secret
    }
  }' ${HELM_DIR}/istio/charts/kiali/templates/deployment.yaml

}

function convertClusterToNamespaced() {
  # $1 - file to convert
  # $2 - cluster kind
  # $3 - namespaced kind
  # $4 - dereference
  sed -i -e 's/^\(\( *\)kind.*'$2'.*$\)/{{- if '$4'.Values.global.multitenant }}\
\2kind: '$3'\
{{- else }}\
\1\
{{- end }}/' \
         -e '0,/name:/ s/^\(\(.*\)name:.*$\)/\1\
{{- if '$4'.Values.global.multitenant }}\
\2namespace: {{ '$4'.Release.Namespace }}\
{{- end }}/' "${1}"
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
    /admissionregistration/d
    /apiGroups/!d
  }' ${HELM_DIR}/istio/charts/galley/templates/clusterrole.yaml

  sed -i -e 's/, *"nodes"//' ${HELM_DIR}/istio/charts/galley/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/galley/templates/clusterrolebinding.yaml

  sed -i -e '/metadata/ {N; s/name: istio-galley/name: istio-galley-\{\{ .Release.Namespace \}\}/}' \
    ${HELM_DIR}/istio/charts/galley/templates/validatingwebhookconfiguration.yaml.tpl

  sed -i -e 's|^\(\(\s*\)rules:.*$\)|{{- if .Values.global.multitenant }}\
\2namespaceSelector:\
\2  matchExpressions:\
\2  - key: maistra.io/member-of\
\2    operator: In\
\2      values:\
\2      - "{{ .Release.Namespace }}"\
{{- end }}\
\1|' ${HELM_DIR}/istio/charts/galley/templates/validatingwebhookconfiguration.yaml.tpl

  sed -i -e '/--validation-webhook-config-file/ {
    s/^\(\( *\)- --validation-webhook-config-file\)/\2- --deployment-namespace\
\2- \{\{ .Release.Namespace \}\}\
\2- --webhook-name\
\2- istio-galley-\{\{ .Release.Namespace \}\}\
\2\{\{- if .Values.global.multitenant \}\}\
\2- --memberRollName=default\
\2\{\{- end \}\}\
\1/
  }' ${HELM_DIR}/istio/charts/galley/templates/deployment.yaml

  # gateways
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/gateways/templates/clusterrolebindings.yaml "$"

  # istiocoredns
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/istiocoredns/templates/clusterrolebinding.yaml

  # kiali
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/kiali/templates/clusterrolebinding.yaml
  sed -i -e 's/\(name:.*\)$/\1-{{ .Release.Namespace }}/' ${HELM_DIR}/istio/charts/kiali/templates/clusterrole.yaml
  sed -i -e 's/\(name: *kiali\)$/\1-{{ .Release.Namespace }}/' ${HELM_DIR}/istio/charts/kiali/templates/clusterrolebinding.yaml

  # mixer
  sed -i -e '/apiGroups:.*apiextensions.k8s.io/,/apiGroups:/ {
    /apiextensions/d
    /apiGroups/!d
  }'  ${HELM_DIR}/istio/charts/mixer/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/mixer/templates/clusterrolebinding.yaml

  # nodeagent
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/nodeagent/templates/clusterrolebinding.yaml

  # pilot
  sed -i -e '/apiGroups:.*apiextensions.k8s.io/,/apiGroups:/ {
    /apiextensions/d
    /apiGroups/!d
  }' \
         -e 's/, *"nodes"//' ${HELM_DIR}/istio/charts/pilot/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/pilot/templates/clusterrolebinding.yaml

  sed -i -e 's/^\(\( *\)- "?discovery"?\)/\1\
\2\{\{- if .Values.global.multitenant \}\}\
\2- --memberRollName=default\
\2\{\{- end \}\}/' ${HELM_DIR}/istio/charts/pilot/templates/deployment.yaml

  # prometheus
  sed -i -e '/nodes/d' ${HELM_DIR}/istio/charts/prometheus/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/prometheus/templates/clusterrolebindings.yaml

  # security
  sed -i -e '/apiGroups:.*authentication.k8s.io/,$ { d }' ${HELM_DIR}/istio/charts/security/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/security/templates/clusterrolebinding.yaml
  # revisit in TP12
  #convertMeshPolicy ${HELM_DIR}/istio/charts/security/templates/enable-mesh-mtls.yaml
  #convertMeshPolicy ${HELM_DIR}/istio/charts/security/templates/enable-mesh-permissive.yaml
  sed -i -e 's/^\(\( *\){.*if .Values.global.trustDomain.*$\)/\2{{- if .Values.global.multitenant }}\
\            - --member-roll-name=default\
\2{{- end }}\
\1/' ${HELM_DIR}/istio/charts/security/templates/deployment.yaml

  # sidecarInjectorWebhook
  sed -i -e '/apiGroups:.*admissionregistration.k8s.io/,+2 { d }' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/clusterrole.yaml
  convertClusterRoleBinding ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/clusterrolebinding.yaml

  sed -i -e '/metadata/ {N; s/name: istio-sidecar-injector/name: istio-sidecar-injector-\{\{ .Release.Namespace \}\}/}' \
         -e '/if \.Values\.enableNamespacesByDefault/,/end/ {
    /enableNamespacesByDefault/ i\
\{\{- if .Values.global.multitenant \}\}\
\    namespaceSelector:\
\      matchExpressions:\
\      - key: maistra.io/member-of\
\        operator: In\
\        values:\
\        - "{{ .Release.Namespace }}"\
\      - key: maistra.io/ignore-namespace\
\        operator: DoesNotExist\
\      - key: istio.openshift.com/ignore-namespace\
\        operator: DoesNotExist\
\{\{- else \}\}
    /end/ i\
\{\{- end \}\}
  }' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/mutatingwebhookconfiguration.yaml.tpl

  sed -i -e '/args:/ a\
            - --webhookConfigName=istio-sidecar-injector-{{ .Release.Namespace }}' ${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml
}

copyOverlay

patchTemplates
patchGrafanaTemplate
patchTracingtemplate
patchKialiTemplate
patchKialiOpenShift

patchMultiTenant
