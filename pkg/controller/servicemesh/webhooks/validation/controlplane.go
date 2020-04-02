package validation

import (
	"context"
	"fmt"
	"net/http"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	"github.com/go-logr/logr"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
)

type ControlPlaneValidator struct {
	client          client.Client
	decoder         atypes.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewControlPlaneValidator(namespaceFilter webhookcommon.NamespaceFilter) *ControlPlaneValidator {
	return &ControlPlaneValidator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*ControlPlaneValidator)(nil)
var _ inject.Client = (*ControlPlaneValidator)(nil)
var _ inject.Decoder = (*ControlPlaneValidator)(nil)

func (v *ControlPlaneValidator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	logger := logf.Log.WithName("smcp-validator").
		WithValues("ServiceMeshControlPlane", webhookcommon.ToNamespacedName(req.AdmissionRequest))
	smcp := &maistrav1.ServiceMeshControlPlane{}

	err := v.decoder.Decode(req, smcp)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	} else if smcp.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smcp resource")
		return admission.ValidationResponse(true, "")
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smcp.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smcp.Namespace))
		return admission.ValidationResponse(true, "")
	}

	if version, err := maistra.ParseVersion(smcp.Spec.Version); err != nil {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("invalid Version specified; supported versions are: %v", maistra.GetSupportedVersions()))
	} else if err := v.validateVersion(ctx, smcp, version); err != nil {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
	}

	if smcp.Namespace == common.GetOperatorNamespace() {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("service mesh may not be installed in the same project/namespace as the operator"))
	}

	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, nil, smcpList)
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	for _, othercp := range smcpList.Items {
		if othercp.Name == smcp.Name && othercp.Namespace == smcp.Namespace {
			continue
		}
		if othercp.Namespace == smcp.Namespace {
			// verify single instance per namespace
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "only one service mesh may be installed per project/namespace")
		}
	}

	if req.AdmissionRequest.Operation == admissionv1beta1.Update {
		// verify update
		oldsmcp := &maistrav1.ServiceMeshControlPlane{}
		err := v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldsmcp)
		if err != nil {
			logger.Error(err, "error decoding admission request")
			return admission.ErrorResponse(http.StatusBadRequest, err)
		}

		return v.validateUpdate(ctx, oldsmcp, smcp, logger)
	}

	return admission.ValidationResponse(true, "")
}

func (v *ControlPlaneValidator) validateVersion(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane, version maistra.Version) error {
	var allErrors []error
	// version specific validation
	switch version.Version() {
	// UndefinedVersion defaults to legacy v1.0
	case maistra.V1_0, maistra.UndefinedVersion:
		// no validation existed in 1.0, so we won't validate
	case maistra.V1_1:
		if err := v.validateV1_1(ctx, smcp); err != nil {
			allErrors = append(allErrors, err)
		}
	default:
		allErrors = append(allErrors, fmt.Errorf("version %s is not supported", version.String()))
	}
	return utilerrors.NewAggregate(allErrors)
}

func (v *ControlPlaneValidator) validateUpdate(ctx context.Context, old, new *maistrav1.ServiceMeshControlPlane, logger logr.Logger) atypes.Response {
	if old.Spec.Version == new.Spec.Version {
		return admission.ValidationResponse(true, "")
	}

	oldVersion, err := maistra.ParseVersion(old.Spec.Version)
	if err != nil {
		logger.Error(err, "error parsing old resource version")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if oldVersion == maistra.UndefinedVersion {
		// UndefinedVersion defaults to legacy v1.0
		oldVersion = maistra.LegacyVersion
	}
	newVersion, err := maistra.ParseVersion(new.Spec.Version)
	if err != nil {
		logger.Error(err, "error parsing new resource version")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if newVersion == maistra.UndefinedVersion {
		// UndefinedVersion defaults to legacy v1.0
		newVersion = maistra.LegacyVersion
	}

	// The logic used here is that we only verify upgrade/downgrade between adjacent versions
	// If an upgrade/downgrade spans multiple versions, the validation for upgrade/downgrade
	// between adjacent versions is chained together, e.g. 1.0 -> 1.3, we'd verify
	// upgrade from 1.0 -> 1.1, then 1.1 -> 1.2, then 1.2 -> 1.3.  If all of those
	// were successful, validation succeeds.  This approach may breakdown if a feature
	// was removed and subsequently reintroduced (e.g. validation from 1.0 -> 1.1
	// fails because feature X is no longer supported, but was added back in 1.3).
	if oldVersion.Version() < newVersion.Version() {
		for version := oldVersion.Version(); version < newVersion.Version(); version++ {
			if err := v.validateUpgrade(ctx, version, old); err != nil {
				return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("cannot upgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	} else {
		for version := oldVersion.Version(); version > newVersion.Version(); version-- {
			if err := v.validateDowngrade(ctx, version, old); err != nil {
				return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("cannot downgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	}

	return admission.ValidationResponse(true, "")
}

func (v *ControlPlaneValidator) validateUpgrade(ctx context.Context, currentVersion maistra.Version, smcp *maistrav1.ServiceMeshControlPlane) error {
	switch currentVersion.Version() {
	case maistra.V1_0:
		return v.validateUpgradeFromV1_0(ctx, smcp)
	default:
		return fmt.Errorf("upgrade from version %s is not supported", currentVersion.String())
	}
}

func (v *ControlPlaneValidator) validateDowngrade(ctx context.Context, currentVersion maistra.Version, smcp *maistrav1.ServiceMeshControlPlane) error {
	switch currentVersion.Version() {
	case maistra.V1_1:
		return v.validateDowngradeFromV1_1(ctx, smcp)
	default:
		return fmt.Errorf("upgrade from version %s is not supported", currentVersion.String())
	}
}

func (v *ControlPlaneValidator) getSMMR(smcp *maistrav1.ServiceMeshControlPlane) (*maistrav1.ServiceMeshMemberRoll, error) {
	smmr := &maistrav1.ServiceMeshMemberRoll{}
	err := v.client.Get(context.TODO(), client.ObjectKey{Namespace: smcp.GetNamespace(), Name: common.MemberRollName}, smmr)
	return smmr, err
}

// InjectClient injects the client.
func (v *ControlPlaneValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *ControlPlaneValidator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
