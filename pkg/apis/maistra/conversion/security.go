package conversion

import (
	"fmt"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populateSecurityValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	security := in.Security
	if security == nil {
		return nil
	}

	// General mutual TLS
	if err := setHelmValue(values, "global.mtls.enabled", security.MutualTLS.Enable); err != nil {
		return err
	}

	if err := setHelmValue(values, "global.mtls.auto", security.MutualTLS.Auto); err != nil {
		return err
	}

	// SPIFFE trust domain
	if security.MutualTLS.Trust.Domain != "" {
		if err := setHelmValue(values, "global.trustDomain", security.MutualTLS.Trust.Domain); err != nil {
			return err
		}
	}
	if len(security.MutualTLS.Trust.AdditionalDomains) > 0 {
		if err := setHelmValue(values, "global.trustDomainAliases", security.MutualTLS.Trust.AdditionalDomains); err != nil {
			return err
		}
	}

	// CA
	switch security.MutualTLS.CertificateAuthority.Type {
	case v2.CertificateAuthorityTypeIstiod, "":
		istiod := security.MutualTLS.CertificateAuthority.Istiod
		if istiod == nil {
			// XXX: maybe log a warning?
			// use self-signed as default
			// this is for pre-1.6.  1.6+ is configured based on the presence
			// of a mounted root cert/key in $ROOT_CA_DIR/ca-key.pem, /etc/cacerts/ca-key.pem by default
			if err := setHelmValue(values, "security.selfSigned", true); err != nil {
				return err
			}
			break
		}
		if in.Version == versions.V1_2.String() {
			// configure pilot (istiod) settings
			if istiod.WorkloadCertTTLDefault != "" {
				if err := setHelmValue(values, "pilot.env.DEFAULT_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLDefault); err != nil {
					return err
				}
			}
			if istiod.WorkloadCertTTLMax != "" {
				if err := setHelmValue(values, "pilot.env.MAX_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLMax); err != nil {
					return err
				}
			}
		} else {
			// configure security (citadel) settings
			if istiod.WorkloadCertTTLDefault != "" {
				if err := setHelmValue(values, "security.workloadCertTtl", istiod.WorkloadCertTTLDefault); err != nil {
					return err
				}
			}
			// can't configure WorkloadCertTTLMax on pre 1.6
		}
		switch istiod.Type {
		case v2.IstioCertificateSignerTypePrivateKey:
			if err := setHelmValue(values, "security.selfSigned", false); err != nil {
				return err
			}
			pksigner := istiod.PrivateKey
			if pksigner == nil {
				// XXX: maybe log a warning
				// let it use its defaults
				break
			}
			if in.Version == versions.V1_2.String() {
				// configure pilot (istiod) settings
				if pksigner.RootCADir != "" {
					if err := setHelmValue(values, "pilot.env.ROOT_CA_DIR", pksigner.RootCADir); err != nil {
						return err
					}
				}
				// XXX: nothing else is currently configurable
			} else {
				// configure security (citadel) settings
				// XXX: nothing here is currently configurable for pre-1.6
			}
		case v2.IstioCertificateSignerTypeSelfSigned, "":
			if err := setHelmValue(values, "security.selfSigned", true); err != nil {
				return err
			}
			selfSigned := istiod.SelfSigned
			if selfSigned == nil {
				// XXX: maybe log a warning
				// let it use its defaults
				break
			}
			componentRoot := "pilot"
			if in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String() {
				// configure security (citadel) settings
				componentRoot = "security"
			}
			if selfSigned.TTL != "" {
				if err := setHelmValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_CA_CERT_TTL", selfSigned.TTL); err != nil {
					return err
				}
			}
			if selfSigned.GracePeriod != "" {
				if err := setHelmValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE", selfSigned.GracePeriod); err != nil {
					return err
				}
			}
			if selfSigned.CheckPeriod != "" {
				if err := setHelmValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL", selfSigned.CheckPeriod); err != nil {
					return err
				}
			}
			if selfSigned.EnableJitter != nil {
				if err := setHelmValue(values, componentRoot+".env.CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR", *selfSigned.EnableJitter); err != nil {
					return err
				}
			}
			// XXX: selfSigned.Org is not supported
		default:
			return fmt.Errorf("unknown Istiod certificate signer type: %s", istiod.Type)
		}
	case v2.CertificateAuthorityTypeCustom:
		custom := security.MutualTLS.CertificateAuthority.Custom
		if custom == nil {
			return fmt.Errorf("No configuration specified for Custom CertificateAuthority")
		}
		if err := setHelmValue(values, "global.caAddress", custom.Address); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown CertificateAuthority type: %s", security.MutualTLS.CertificateAuthority.Type)
	}

	// Identity
	switch security.MutualTLS.Identity.Type {
	case v2.IdentityConfigTypeKubernetes:
		if err := setHelmValue(values, "global.jwtPolicy", "first-party-jwt"); err != nil {
			return err
		}
	case v2.IdentityConfigTypeThirdParty, "":
		if err := setHelmValue(values, "global.jwtPolicy", "third-party-jwt"); err != nil {
			return err
		}
		tpi := security.MutualTLS.Identity.ThirdParty
		if tpi == nil {
			// XXX: maybe log a warning
			// let it use its defaults
			break
		}
		if tpi.Issuer != "" {
			// XXX: only supported in 1.6+
			if err := setHelmValue(values, "pilot.env.TOKEN_ISSUER", tpi.Issuer); err != nil {
				return err
			}
		}
		if tpi.Audience != "" {
			if err := setHelmValue(values, "global.sds.token.aud", tpi.Audience); err != nil {
				return err
			}
		}
		// XXX: TokenPath is not currently supported
	default:
		return fmt.Errorf("unknown Identity type: %s", security.MutualTLS.Identity.Type)
	}

	// Control Plane Security
	if err := setHelmValue(values, "global.controlPlaneSecurityEnabled", security.MutualTLS.ControlPlane.Enable); err != nil {
		return err
	}
	if security.MutualTLS.ControlPlane.CertProvider != "" {
		if err := setHelmValue(values, "global.pilotCertProvider", string(security.MutualTLS.ControlPlane.CertProvider)); err != nil {
			return err
		}
	}

	return nil
}
