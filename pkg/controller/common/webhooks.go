package common

import (
	"bytes"

	v1 "k8s.io/api/admissionregistration/v1"
)

var (
	// IstioRootCertKey name of Secret entry Istio uses for the cert
	IstioRootCertKey = "root-cert.pem"
	// IstiodCertKey name of Secret entry Istiod uses for the cert
	IstiodCertKey = "ca-cert.pem"
	// IstiodTLSSecretCertKey name of Secret entry containing intermediate certificate for Istio provided by cert-manager
	IstiodTLSSecretCertKey = "tls.crt"
	// ServiceCABundleKey name of config map entry service-ca operator uses for the CA bundle
	ServiceCABundleKey = "service-ca.crt"
)

// InjectCABundle updates the CABundle in a WebhookClientConfig. It returns true
// if the value has changed, false otherwise
func InjectCABundle(config *v1.WebhookClientConfig, caBundle []byte) bool {
	if !bytes.Equal(config.CABundle, caBundle) {
		config.CABundle = caBundle
		return true
	}
	return false
}
