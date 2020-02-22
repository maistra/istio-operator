package validation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

type ControlPlaneValidator struct {
	client          client.Client
	decoder         *admission.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewControlPlaneValidator(namespaceFilter webhookcommon.NamespaceFilter) *ControlPlaneValidator {
	return &ControlPlaneValidator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*ControlPlaneValidator)(nil)
var _ inject.Client = (*ControlPlaneValidator)(nil)
var _ admission.DecoderInjector = (*ControlPlaneValidator)(nil)

func (v *ControlPlaneValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := logf.Log.WithName("smcp-validator").
		WithValues("ServiceMeshControlPlane", webhookcommon.ToNamespacedName(&req.AdmissionRequest))
	smcp := &maistrav1.ServiceMeshControlPlane{}

	err := v.decoder.Decode(req, smcp)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.Errored(http.StatusBadRequest, err)
	} else if smcp.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smcp resource")
		return admission.Allowed("")
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smcp.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smcp.Namespace))
		return admission.Allowed("")
	}

	if version, err := versions.ParseVersion(smcp.Spec.Version); err != nil {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("invalid Version specified; supported versions are: %v", versions.GetSupportedVersions()))
	} else if err := v.validateVersion(ctx, smcp, version); err != nil {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
	}

	if smcp.Namespace == common.GetOperatorNamespace() {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("service mesh may not be installed in the same project/namespace as the operator"))
	}

	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, smcpList, client.InNamespace(smcp.Namespace))
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// verify single instance per namespace
	if (len(smcpList.Items) == 1 && smcpList.Items[0].Name != smcp.Name) || len(smcpList.Items) > 1 {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "only one service mesh may be installed per project/namespace")
	}

	if req.AdmissionRequest.Operation == admissionv1beta1.Update {
		// verify update
		oldsmcp := &maistrav1.ServiceMeshControlPlane{}
		err := v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldsmcp)
		if err != nil {
			logger.Error(err, "error decoding admission request")
			return admission.Errored(http.StatusBadRequest, err)
		}

		return v.validateUpdate(ctx, oldsmcp, smcp, logger)
	}

	return admission.ValidationResponse(true, "")
}

func (v *ControlPlaneValidator) validateVersion(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane, version versions.Version) error {
	// version specific validation
	switch version.Version() {
	// UndefinedVersion defaults to legacy v1.0
	case versions.V1_0:
		// no validation existed in 1.0, so we won't validate
		return nil
	}
	return version.Strategy().Validate(ctx, v.client, smcp)
}

func (v *ControlPlaneValidator) validateUpdate(ctx context.Context, old, new *maistrav1.ServiceMeshControlPlane, logger logr.Logger) admission.Response {
	if old.Spec.Version == new.Spec.Version {
		return admission.ValidationResponse(true, "")
	}

	oldVersion, err := versions.ParseVersion(old.Spec.Version)
	if err != nil {
		logger.Error(err, "error parsing old resource version")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	newVersion, err := versions.ParseVersion(new.Spec.Version)
	if err != nil {
		logger.Error(err, "error parsing new resource version")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// The logic used here is that we only verify upgrade/downgrade between adjacent versions
	// If an upgrade/downgrade spans multiple versions, the validation for upgrade/downgrade
	// between adjacent versions is chained together, e.g. 1.0 -> 1.3, we'd verify
	// upgrade from 1.0 -> 1.1, then 1.1 -> 1.2, then 1.2 -> 1.3.  If all of those
	// were successful, validation succeeds.  This approach may breakdown if a feature
	// was removed and subsequently reintroduced (e.g. validation from 1.0 -> 1.1
	// fails because feature X is no longer supported, but was added back in 1.3).
	if oldVersion.Version() < newVersion.Version() {
		for version := oldVersion.Version() + 1; version <= newVersion.Version(); version++ {
			if err := version.Strategy().ValidateUpgrade(ctx, v.client, new); err != nil {
				return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("cannot upgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	} else {
		for version := oldVersion.Version(); version > newVersion.Version(); version-- {
			if err := version.Strategy().ValidateDowngrade(ctx, v.client, new); err != nil {
				return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("cannot downgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	}

	return admission.Allowed("")
}

// InjectClient injects the client.
func (v *ControlPlaneValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *ControlPlaneValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
