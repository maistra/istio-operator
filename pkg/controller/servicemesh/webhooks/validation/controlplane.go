package validation

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
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

	if len(smcp.Spec.Version) > 0 {
		if _, ok := common.GetCNINetworkName(smcp.Spec.Version); !ok {
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("invalid Version specified; supported versions are: %v", common.GetSupportedVersions()))
		}
	}
	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, nil, smcpList)
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	namespace := smcp.Namespace
	for _, othercp := range smcpList.Items {
		if othercp.Name == smcp.Name && othercp.Namespace == smcp.Namespace {
			continue
		}
		if othercp.Namespace == namespace {
			// verify single instance per namespace
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "only one service mesh may be installed per project/namespace")
		}
	}

	// TODO: we should have generic accessors for the helm values
	if globalValues, ok := smcp.Spec.Istio["global"].(map[string]interface{}); ok {
		tracer := "zipkin"
		if proxyValues, ok := globalValues["proxy"].(map[string]interface{}); ok {
			if tracerValue, ok := proxyValues["tracer"].(string); ok {
				tracer = tracerValue
			}
		}
		if tracerValues, ok := globalValues["tracer"].(map[string]interface{}); ok {
			if zipkinValues, ok := tracerValues["zipkin"].(map[string]interface{}); ok {
				if address, ok := zipkinValues["address"].(string); ok {
					// tracer must be "zipkin"
					if tracer != "zipkin" {
						return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "global.proxy.tracer must equal 'zipkin' if global.tracer.zipkin.address is set")
					}
					// if an address is set, it must point to the same namespace the SMCP resides in
					addressParts := strings.Split(address, ".")
					if len(addressParts) == 1 {
						return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "global.tracer.zipkin.address must include a namespace")
					} else if len(addressParts) > 1 {
						namespace := addressParts[1]
						if len(addressParts) == 2 {
							// there might be a port :9411 or similar at the end. make sure to ignore for namespace comparison
							namespacePortParts := strings.Split(namespace, ":")
							namespace = namespacePortParts[0]
						}
						if namespace != smcp.GetObjectMeta().GetNamespace() {
							return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "global.tracer.zipkin.address must point to a service in same namespace as SMCP")
						}
					}
					// tracing.enabled must be false
					if tracingValues, ok := smcp.Spec.Istio["tracing"].(map[string]interface{}); ok {
						if enabled, ok := tracingValues["enabled"].(bool); ok {
							if enabled {
								return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "tracing.enabled must not be true if global.tracer.zipkin.address is set")
							}
						}
					}
					// kiali.jaegerInClusterURL must be set (if kiali is enabled)
					if kialiValues, ok := smcp.Spec.Istio["kiali"].(map[string]interface{}); ok {
						if enabled, ok := kialiValues["enabled"].(bool); ok && enabled {
							if jaegerInClusterURL, ok := kialiValues["jaegerInClusterURL"].(string); !ok || jaegerInClusterURL == "" {
								return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "kiali.jaegerInClusterURL must be defined if global.tracer.zipkin.address is set")
							}
						}
					}
				}
			}
		}
	}

	return admission.ValidationResponse(true, "")
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
