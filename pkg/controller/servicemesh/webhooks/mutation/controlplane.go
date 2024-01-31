package mutation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

type ControlPlaneMutator struct {
	client          client.Client
	decoder         *admission.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewControlPlaneMutator(namespaceFilter webhookcommon.NamespaceFilter) *ControlPlaneMutator {
	return &ControlPlaneMutator{
		namespaceFilter: namespaceFilter,
	}
}

var (
	_ admission.Handler         = (*ControlPlaneMutator)(nil)
	_ inject.Client             = (*ControlPlaneMutator)(nil)
	_ admission.DecoderInjector = (*ControlPlaneMutator)(nil)
)

func (v *ControlPlaneMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.Log.WithName("smcp-mutator").
		WithValues("ServiceMeshControlPlane", webhookcommon.ToNamespacedName(&req.AdmissionRequest))

	// do we care about this object?
	if !v.namespaceFilter.Watching(req.Namespace) {
		log.Info(fmt.Sprintf("operator is not watching namespace '%s'", req.Namespace))
		return admission.Allowed("")
	}

	mutator, err := v.decodeRequest(req, log)
	if err != nil {
		log.Error(err, "error decoding admission request")
		return admission.Errored(http.StatusBadRequest, err)
	} else if mutator.Object().GetDeletionTimestamp() != nil {
		log.Info("skipping deleted smcp resource")
		return admission.Allowed("")
	}

	// on create we set the version to the current default version
	// on update, if the version is removed we reset it to what was previously set
	currentVersion := mutator.NewVersion()
	if currentVersion == "" {
		switch req.AdmissionRequest.Operation {
		case admissionv1beta1.Create:
			log.Info("Setting .spec.version to default value", "version", versions.DefaultVersion.String())
			mutator.SetVersion(mutator.DefaultVersion())
		case admissionv1beta1.Update:
			oldVersion := mutator.OldVersion()
			if currentVersion != oldVersion && oldVersion != versions.InvalidVersion.String() {
				log.Info("Setting .spec.version to existing value", "version", oldVersion)
				mutator.SetVersion(oldVersion)
			}
		}
	}

	// As we are deprecating IOR, on creating a v2.5 SMCP we want to disable IOR if not specified explicitly
	if req.AdmissionRequest.Operation == admissionv1beta1.Create {
		newOpenShiftRoute := mutator.IsOpenShiftRouteEnabled()

		if newOpenShiftRoute == nil {
			var ver versions.Version
			var err error
			// If version is not specified
			if currentVersion == "" {
				ver, err = versions.ParseVersion(mutator.DefaultVersion())
			} else {
				ver, err = versions.ParseVersion(currentVersion)
			}
			if err == nil && ver.AtLeast(versions.V2_5.Version()) {
				mutator.SetOpenShiftRoute(false)
			}
		}
	}

	if len(mutator.GetProfiles()) == 0 {
		log.Info("Setting .spec.profiles to default value", "profiles", []string{v1.DefaultTemplate})
		mutator.SetProfiles([]string{v1.DefaultTemplate})
	}

	patches := mutator.GetPatches()
	if patches == nil {
		return admission.Allowed("")
	}
	return admission.Patched("", patches...)
}

// InjectClient injects the client.
func (v *ControlPlaneMutator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *ControlPlaneMutator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

func (v *ControlPlaneMutator) decodeRequest(req admission.Request, logger logr.Logger) (smcpmutator, error) {
	switch req.Kind.Version {
	case v1.SchemeGroupVersion.Version:
		smcp := &v1.ServiceMeshControlPlane{}
		err := v.decoder.Decode(req, smcp)
		if err != nil {
			logger.Error(err, "error decoding admission request")
			return nil, err
		}
		var oldsmcp *v1.ServiceMeshControlPlane
		if req.Operation == admissionv1beta1.Update {
			oldsmcp = &v1.ServiceMeshControlPlane{}
			err = v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldsmcp)
			if err != nil {
				logger.Error(err, "error decoding admission request")
				return nil, err
			}
		}
		return &smcpv1mutator{smcppatch: &smcppatch{}, smcp: smcp, oldsmcp: oldsmcp}, nil
	case v2.SchemeGroupVersion.Version:
		smcp := &v2.ServiceMeshControlPlane{}
		err := v.decoder.Decode(req, smcp)
		if err != nil {
			logger.Error(err, "error decoding admission request")
			return nil, err
		}
		var oldsmcp *v2.ServiceMeshControlPlane
		if req.Operation == admissionv1beta1.Update {
			oldsmcp = &v2.ServiceMeshControlPlane{}
			err = v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldsmcp)
			if err != nil {
				logger.Error(err, "error decoding admission request")
				return nil, err
			}
		}
		return &smcpv2mutator{smcppatch: &smcppatch{}, smcp: smcp, oldsmcp: oldsmcp}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", req.Kind.String())
	}
}

type smcpmutator interface {
	Object() metav1.Object
	DefaultVersion() string
	OldVersion() string
	NewVersion() string
	SetVersion(version string)
	GetProfiles() []string
	SetProfiles(profiles []string)
	GetPatches() []jsonpatch.JsonPatchOperation
	IsOpenShiftRouteEnabled() *bool
	SetOpenShiftRoute(bool)
}

type smcppatch struct {
	patches []jsonpatch.JsonPatchOperation
}

func (m *smcppatch) GetPatches() []jsonpatch.JsonPatchOperation {
	return m.patches
}

func (m *smcppatch) SetVersion(version string) {
	m.patches = append(m.patches, jsonpatch.NewPatch("add", "/spec/version", version))
}

func (m *smcppatch) SetProfiles(profiles []string) {
	value := make([]interface{}, len(profiles))
	for index, profile := range profiles {
		value[index] = profile
	}
	m.patches = append(m.patches, jsonpatch.NewPatch("add", "/spec/profiles", value))
}

type smcpv1mutator struct {
	*smcppatch
	smcp    *v1.ServiceMeshControlPlane
	oldsmcp *v1.ServiceMeshControlPlane
}

var _ smcpmutator = (*smcpv1mutator)(nil)

func (m *smcpv1mutator) Object() metav1.Object {
	return m.smcp
}

func (m *smcpv1mutator) DefaultVersion() string {
	return versions.V1_1.String()
}

func (m *smcpv1mutator) NewVersion() string {
	return m.smcp.Spec.Version
}

func (m *smcpv1mutator) OldVersion() string {
	if m.oldsmcp == nil {
		return versions.InvalidVersion.String()
	}
	return m.oldsmcp.Spec.Version
}

func (m *smcpv1mutator) GetProfiles() []string {
	if len(m.smcp.Spec.Profiles) == 0 {
		if m.smcp.Spec.Template == "" {
			return nil
		}
		return []string{m.smcp.Spec.Template}
	}
	return m.smcp.Spec.Profiles
}

func (m *smcpv1mutator) IsOpenShiftRouteEnabled() *bool {
	return nil
}

func (m *smcpv1mutator) SetOpenShiftRoute(value bool) {}

type smcpv2mutator struct {
	*smcppatch
	smcp    *v2.ServiceMeshControlPlane
	oldsmcp *v2.ServiceMeshControlPlane
}

var _ smcpmutator = (*smcpv2mutator)(nil)

func (m *smcpv2mutator) Object() metav1.Object {
	return m.smcp
}

func (m *smcpv2mutator) DefaultVersion() string {
	return versions.DefaultVersion.String()
}

func (m *smcpv2mutator) NewVersion() string {
	return m.smcp.Spec.Version
}

func (m *smcpv2mutator) OldVersion() string {
	if m.oldsmcp == nil {
		return versions.InvalidVersion.String()
	}
	return m.oldsmcp.Spec.Version
}

func (m *smcpv2mutator) GetProfiles() []string {
	return m.smcp.Spec.Profiles
}

func (m *smcpv2mutator) IsOpenShiftRouteEnabled() *bool {
	gateways := m.smcp.Spec.Gateways

	if gateways == nil {
		return nil
	}

	route := gateways.OpenShiftRoute

	if route == nil {
		return nil
	}

	return route.Enabled
}

func (m *smcpv2mutator) SetOpenShiftRoute(value bool) {
	m.patches = append(m.patches, jsonpatch.NewPatch("add", "/spec/gateways/openshiftRoute/enabled", value))
}
