package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (v *ControlPlaneValidator) validateV1_1(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane) error {
	logger := logf.Log.WithName("smcp-validator")
	var allErrors []error

	if zipkinAddress, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.tracer.zipkin.address", ".")...); ok && len(zipkinAddress) > 0 {
		tracer, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.proxy.tracer", ".")...)
		if ok && tracer != "zipkin" {
			// tracer must be "zipkin"
			allErrors = append(allErrors, fmt.Errorf("global.proxy.tracer must equal 'zipkin' if global.tracer.zipkin.address is set"))

		}
		// if an address is set, it must point to the same namespace the SMCP resides in
		addressParts := strings.Split(zipkinAddress, ".")
		if len(addressParts) == 1 {
			allErrors = append(allErrors, fmt.Errorf("global.tracer.zipkin.address must include a namespace"))
		} else if len(addressParts) > 1 {
			namespace := addressParts[1]
			if len(addressParts) == 2 {
				// there might be a port :9411 or similar at the end. make sure to ignore for namespace comparison
				namespacePortParts := strings.Split(namespace, ":")
				namespace = namespacePortParts[0]
			}
			if namespace != smcp.GetObjectMeta().GetNamespace() {
				allErrors = append(allErrors, fmt.Errorf("global.tracer.zipkin.address must point to a service in same namespace as SMCP"))
			}
		}
		if err := errForEnabledValue(smcp.Spec.Istio, "tracing.enabled", true); err != nil {
			// tracing.enabled must be false
			allErrors = append(allErrors, fmt.Errorf("tracing.enabled must be false if global.tracer.zipkin.address is set"))
		}

		if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled", true); err != nil {
			if jaegerInClusterURL, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("kiali.jaegerInClusterURL", ".")...); !ok || len(jaegerInClusterURL) == 0 {
				allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL must be defined if global.tracer.zipkin.address is set"))
			}
		}
	}

	if cipherSuite, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.tls.cipherSuite", ".")...); ok && len(cipherSuite) > 0 {
		found := false
		bad := false
		for _, cipher := range strings.Split(cipherSuite, ",") {
			trimmed := strings.Trim(cipher, " ")
			switch trimmed {
			case "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":
				found = true
			}
			// Check for bad ciphers before good ones
			// see https://tools.ietf.org/html/rfc7540#appendix-A
			switch trimmed {
			case
				"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
				"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
				"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
				"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
				"TLS_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_RSA_WITH_AES_128_CBC_SHA256",
				"TLS_RSA_WITH_AES_128_CBC_SHA",
				"TLS_RSA_WITH_AES_256_CBC_SHA",
				"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA",
				"TLS_RSA_WITH_3DES_EDE_CBC_SHA",
				"TLS_DHE_RSA_WITH_AES_128_CBC_SHA",
				"TLS_DHE_RSA_WITH_AES_256_CBC_SHA":
					bad = true
			case
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":
					if bad {
						allErrors = append(allErrors, fmt.Errorf("global.tls.cipherSuite includes an HTTP/2-approved cipher suite (%v), but it comes after unapproved cipher suites", trimmed))
					}
			default:
				allErrors = append(allErrors, fmt.Errorf("global.tls.cipherSuite includes an unrecognised cipher suite %v", trimmed))
			}
		}
		if !found {
			allErrors = append(allErrors, fmt.Errorf("global.tls.cipherSuite must include an HTTP/2-required AES_128_GCM_SHA256 cipher suite, either one of TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 or TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"))
		}
	}

	if ecdhCurves, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.tls.ecdhCurves", ".")...); ok && len(ecdhCurves) > 0 {
		for _, ecdhCurve := range strings.Split(ecdhCurves, ",") {
			trimmed := strings.Trim(ecdhCurve, " ")
			switch trimmed {
			case
				"CurveP256",
				"CurveP384",
				"CurveP521",
				"X25519":
			default:
				allErrors = append(allErrors, fmt.Errorf("global.tls.ecdhCurves includes an unrecognised ECDH curve %v", trimmed))
			}
		}
	}

	smmr, err := v.getSMMR(smcp)
	if err != nil {
		if !errors.IsNotFound(err) {
			// log error, but don't fail validation: we'll just assume that the control plane namespace is the only namespace for now
			logger.Error(err, "failed to retrieve SMMR for SMCP")
			smmr = nil
		}
	}

	meshNamespaces := common.GetMeshNamespaces(smcp, smmr)
	for _, gateway := range getMapKeys(smcp.Spec.Istio, "gateways") {
		if err := errForStringValue(smcp.Spec.Istio, "gateways."+gateway+".namespace", meshNamespaces); err != nil {
			allErrors = append(allErrors, fmt.Errorf("%v: namespace must be part of the mesh", err))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}
