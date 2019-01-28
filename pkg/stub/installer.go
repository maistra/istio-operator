package stub

import (
	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"
	"strings"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"bytes"
	"k8s.io/api/batch/v1"
)

const (
	installerConfigMapName = "install.istio.inventory"
	installerJobName = "openshift-ansible-istio-installer-job"
)

func (h *Handler) getInstallerJob(cr *v1alpha1.Installation) *v1.Job {
	return h.getJob(installerJobName, namespace)
}

func (h *Handler) newInstallerJobItems(cr *v1alpha1.Installation) []sdk.Object {
	return h.newJobItems(cr, installerJobName, installerConfigMapName, namespace, h.getInstallerInventory(cr))
}

func (h *Handler) getInstallerInventory(cr *v1alpha1.Installation) string {
	var b bytes.Buffer

	b.WriteString(`
[OSEv3:children]
masters

[OSEv3:vars]
openshift_istio_install=True`)
	b.WriteString(newline)
	addStringValue(&b,"openshift_deployment_type=", h.getDeploymentType(cr))
	addStringValue(&b,"openshift_istio_namespace=", namespace)
	addStringValue(&b,"openshift_istio_image_prefix=", h.cleanPrefix(h.getIstioImagePrefix(cr)))
	addStringValue(&b,"openshift_istio_image_version=", h.getIstioImageVersion(cr))
	addStringValue(&b, "openshift_release=", h.getOpenShiftRelease())

	if cr.Spec != nil {
		h.addIstioInstallerConfiguration(&b, cr.Spec.Istio)
		h.addJaegerInstallerConfiguration(&b, cr.Spec.Jaeger)
		h.addKialiInstallerConfiguration(&b, cr.Spec.Kiali)
		h.addLauncherInstallerConfiguration(&b, cr.Spec.Launcher)
		h.addThreeScaleInstallerConfiguration(&b, cr.Spec.ThreeScale)
	}

	b.WriteString(`
[masters]
127.0.0.1 ansible_connection=local
`)
	return b.String()
}

func (h *Handler) addIstioInstallerConfiguration(b *bytes.Buffer, istio *v1alpha1.IstioSpec) {
	if istio != nil {
		addBooleanPtrValue(b,"openshift_istio_install_community=", istio.Community)
		addBooleanPtrValue(b,"openshift_istio_install_auth=", istio.Authentication)
	}
}

func (h *Handler) addJaegerInstallerConfiguration(b *bytes.Buffer, jaeger *v1alpha1.JaegerSpec) {
	if jaeger != nil {
		addStringPtrValue(b,"openshift_istio_jaeger_image_prefix=", h.cleanPrefixPtr(jaeger.Prefix))
		addStringPtrValue(b,"openshift_istio_jaeger_image_version=", jaeger.Version)
		addStringPtrValue(b,"openshift_istio_elasticsearch_memory=", jaeger.ElasticsearchMemory)
	}
}

func (h *Handler) addKialiInstallerConfiguration(b *bytes.Buffer, kiali *v1alpha1.KialiSpec) {
	if kiali != nil {
		addStringPtrValue(b,"openshift_istio_kiali_image_prefix=", h.cleanPrefixPtr(kiali.Prefix))
		addStringPtrValue(b,"openshift_istio_kiali_image_version=", kiali.Version)
		addStringPtrValue(b,"openshift_istio_kiali_username=", kiali.Username)
		addStringPtrValue(b,"openshift_istio_kiali_password=", kiali.Password)
	}
}

func (h *Handler) addLauncherInstallerConfiguration(b *bytes.Buffer, launcher *v1alpha1.LauncherSpec) {
	if launcher != nil {
		addBooleanValue(b,"openshift_istio_install_launcher=", true)
		addStringPtrValue(b,"openshift_istio_master_public_url=", h.getMasterPublicURL())
		if launcher.OpenShift != nil {
			addStringPtrValue(b,"launcher_openshift_user=", launcher.OpenShift.User)
			addStringPtrValue(b,"launcher_openshift_pwd=", launcher.OpenShift.Password)
		}
		if launcher.GitHub != nil {
			addStringPtrValue(b,"launcher_github_username=", launcher.GitHub.Username)
			addStringPtrValue(b,"launcher_github_token=", launcher.GitHub.Token)
		}
		if launcher.Catalog != nil {
			addStringPtrValue(b,"launcher_catalog_git_repo=", launcher.Catalog.Repo)
			addStringPtrValue(b,"launcher_catalog_git_branch=", launcher.Catalog.Branch)
			addStringPtrValue(b,"launcher_booster_catalog_filter=", launcher.Catalog.Filter)
		}
	}
}

func (h *Handler) addThreeScaleInstallerConfiguration(b *bytes.Buffer, threeScale *v1alpha1.ThreeScaleSpec) {
	if h.getThreeScaleEnabled(threeScale) {
		addBooleanValue(b,"openshift_istio_install_three_scale=", true)
		addStringPtrValue(b, "openshift_istio_three_scale_image_prefix=", h.cleanPrefixPtr(threeScale.Prefix))
		addStringPtrValue(b, "openshift_istio_three_scale_image_version=", threeScale.Version)
		if threeScale.Adapter != nil {
			addInt32PtrValue(b,"three_scale_adapter_listenAddr=", threeScale.Adapter.ListenAddr)
			addStringPtrValue(b,"three_scale_adapter_logLevel=", threeScale.Adapter.LogLevel)
			addBooleanPtrValue(b,"three_scale_adapter_logJSON=", threeScale.Adapter.LogJSON)
			addBooleanPtrValue(b,"three_scale_adapter_reportMetrics=", threeScale.Adapter.ReportMetrics)
			addInt32PtrValue(b,"three_scale_adapter_metricsPort=", threeScale.Adapter.MetricsPort)
			addIntPtrValue(b,"three_scale_adapter_cacheTTLSeconds=", threeScale.Adapter.CacheTTLSeconds)
			addIntPtrValue(b,"three_scale_adapter_cacheRefreshSeconds=", threeScale.Adapter.CacheRefreshSeconds)
			addIntPtrValue(b,"three_scale_adapter_cacheEntriesMax=", threeScale.Adapter.CacheEntriesMax)
			addIntPtrValue(b,"three_scale_adapter_cacheRefreshRetries=", threeScale.Adapter.CacheRefreshRetries)
			addBooleanPtrValue(b,"three_scale_adapter_allowInsecureConn=", threeScale.Adapter.AllowInsecureConn)
			addIntPtrValue(b,"three_scale_adapter_clientTimeoutSeconds=", threeScale.Adapter.ClientTimeoutSeconds)
		}
	}
}

// We make the following modifications
//   namespace -> namespace/
//   registry/namespace -> registry/namespace
// We do not modify the following formats
//   namespace/
//   registry/namespace/
//   registry/namespace/prefix
func (h *Handler) cleanPrefix(prefix string) string {
	if !strings.HasSuffix(prefix,"/") {
		if strings.Count(prefix, "/") < 2 {
			return prefix + "/"
		}
	}
	return prefix
}

func (h *Handler) cleanPrefixPtr(prefixPtr *string) *string {
	if prefixPtr != nil {
		result := h.cleanPrefix(*prefixPtr)
		return &result
	} else {
		return nil
	}
}
