package validation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
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

var (
	_ admission.Handler         = (*ControlPlaneValidator)(nil)
	_ inject.Client             = (*ControlPlaneValidator)(nil)
	_ admission.DecoderInjector = (*ControlPlaneValidator)(nil)
)

func (v *ControlPlaneValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := logf.Log.WithName("smcp-validator").
		WithValues("ServiceMeshControlPlane", webhookcommon.ToNamespacedName(&req.AdmissionRequest))

	// do we care about this object?
	if !v.namespaceFilter.Watching(req.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", req.Namespace))
		return admission.Allowed("")
	}

	if req.Namespace == common.GetOperatorNamespace() {
		return badRequest("service mesh may not be installed in the same project/namespace as the operator")
	}

	smcpList := &maistrav2.ServiceMeshControlPlaneList{}
	err := v.client.List(ctx, smcpList, client.InNamespace(req.Namespace))
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// verify single instance per namespace
	if (len(smcpList.Items) == 1 && smcpList.Items[0].Name != req.Name) || len(smcpList.Items) > 1 {
		return badRequest("only one service mesh may be installed per project/namespace")
	}

	smcprequest, err := v.decodeRequest(req, logger)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	smcp := smcprequest.New()
	smcpVersion := smcprequest.NewVersion()
	if smcp.GetDeletionTimestamp() != nil {
		logger.Info("skipping deleted smcp resource")
		return admission.Allowed("")
	} else if !smcpVersion.IsSupported() {
		return badRequest(fmt.Sprintf("Only '%v' versions are supported", versions.GetSupportedVersionNames()))
	}

	if err := v.validateVersion(ctx, smcp, smcpVersion); err != nil {
		return badRequest(err.Error())
	}

	if req.AdmissionRequest.Operation == admissionv1beta1.Update {
		// verify update
		return v.validateUpdate(ctx, smcprequest.OldVersion(), smcpVersion, smcprequest.Old(), smcp)
	}

	return smcpVersion.Strategy().ValidateRequest(ctx, v.client, req, smcp)
}

func (v *ControlPlaneValidator) decodeRequest(req admission.Request, logger logr.Logger) (*smcprequest, error) {
	switch req.Kind.Version {
	case maistrav1.SchemeGroupVersion.Version:
		return nil, fmt.Errorf("must use v2 ServiceMeshControlPlane resource")
	case maistrav2.SchemeGroupVersion.Version:
		smcp := &maistrav2.ServiceMeshControlPlane{}
		err := v.decoder.Decode(req, smcp)
		if err != nil {
			logger.Error(err, "error decoding admission request")
			return nil, err
		}
		newVersion, err := versions.ParseVersion(smcp.Spec.Version)
		if err != nil {
			return nil, fmt.Errorf("invalid Version specified; supported versions are: %v", versions.GetSupportedVersions())
		}
		var oldsmcp *maistrav2.ServiceMeshControlPlane
		oldVersion := versions.InvalidVersion
		if req.Operation == admissionv1beta1.Update {
			oldsmcp = &maistrav2.ServiceMeshControlPlane{}
			err = v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldsmcp)
			if err != nil {
				logger.Error(err, "error decoding admission request")
				return nil, err
			}
			oldVersion, err = versions.ParseVersion(oldsmcp.Spec.Version)
			if err != nil {
				return nil, fmt.Errorf("invalid Version specified; supported versions are: %v", versions.GetSupportedVersions())
			}
		}
		return &smcprequest{new: smcp, old: oldsmcp, newVersion: newVersion, oldVersion: oldVersion}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", req.Kind.String())
	}
}

func (v *ControlPlaneValidator) validateVersion(ctx context.Context, smcp *maistrav2.ServiceMeshControlPlane, version versions.Version) error {
	return version.Strategy().ValidateV2(ctx, v.client, &smcp.ObjectMeta, &smcp.Spec)
}

func (v *ControlPlaneValidator) validateUpdate(ctx context.Context, oldVersion, newVersion versions.Version, oldObj, newObj *maistrav2.ServiceMeshControlPlane) admission.Response {
	// The logic used here is that we only verify upgrade/downgrade between adjacent versions
	// If an upgrade/downgrade spans multiple versions, the validation for upgrade/downgrade
	// between adjacent versions is chained together, e.g. 1.0 -> 1.3, we'd verify
	// upgrade from 1.0 -> 1.1, then 1.1 -> 1.2, then 1.2 -> 1.3.  If all of those
	// were successful, validation succeeds.  This approach may breakdown if a feature
	// was removed and subsequently reintroduced (e.g. validation from 1.0 -> 1.1
	// fails because feature X is no longer supported, but was added back in 1.3).
	if oldVersion.Version() < newVersion.Version() {
		for version := oldVersion.Version() + 1; version <= newVersion.Version(); version++ {
			if err := version.Strategy().ValidateUpgrade(ctx, v.client, newObj); err != nil {
				return badRequest(fmt.Sprintf("cannot upgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	} else {
		for version := oldVersion.Version(); version > newVersion.Version(); version-- {
			if err := version.Strategy().ValidateDowngrade(ctx, v.client, newObj); err != nil {
				return badRequest(fmt.Sprintf("cannot downgrade control plane from version %s to %s: %s", oldVersion.String(), newVersion.String(), err))
			}
		}
	}

	if err := newVersion.Strategy().ValidateUpdate(ctx, v.client, oldObj, newObj); err != nil {
		return badRequest(err.Error())
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

type smcprequest struct {
	new        *maistrav2.ServiceMeshControlPlane
	old        *maistrav2.ServiceMeshControlPlane
	newVersion versions.Version
	oldVersion versions.Version
}

func (smcp *smcprequest) New() *maistrav2.ServiceMeshControlPlane {
	if smcp == nil {
		return nil
	}
	return smcp.new
}

func (smcp *smcprequest) NewVersion() versions.Version {
	if smcp == nil {
		return versions.InvalidVersion
	}
	return smcp.newVersion
}

func (smcp *smcprequest) Old() *maistrav2.ServiceMeshControlPlane {
	if smcp == nil {
		return nil
	}
	return smcp.old
}

func (smcp *smcprequest) OldVersion() versions.Version {
	if smcp == nil {
		return versions.InvalidVersion
	}
	return smcp.oldVersion
}
