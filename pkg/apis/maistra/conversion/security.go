package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populateSecurityValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	security := in.Security
	if security == nil {
		return nil
	}

	// General mutual TLS
	if security.MutualTLS.Enable != nil {
		if err := setHelmBoolValue(values, "global.mtls.enabled", *security.MutualTLS.Enable); err != nil {
			return err
		}
	}
	if security.MutualTLS.Auto != nil {
		if err := setHelmBoolValue(values, "global.mtls.auto", *security.MutualTLS.Auto); err != nil {
			return err
		}
	}

	// SPIFFE trust domain
	if security.MutualTLS.Trust.Domain != "" {
		if err := setHelmStringValue(values, "global.trustDomain", security.MutualTLS.Trust.Domain); err != nil {
			return err
		}
	}
	if security.MutualTLS.Trust.AdditionalDomains != nil {
		if err := setHelmStringSliceValue(values, "global.trustDomainAliases", security.MutualTLS.Trust.AdditionalDomains); err != nil {
			return err
		}
	}

	// CA
	switch security.MutualTLS.CertificateAuthority.Type {
	case v2.CertificateAuthorityTypeIstiod:
		istiod := security.MutualTLS.CertificateAuthority.Istiod
		if err := setHelmStringValue(values, "pilot.ca.implementation", string(security.MutualTLS.CertificateAuthority.Type)); err != nil {
			return err
		}
		if istiod == nil {
			break
		}
		if in.Version == versions.V2_0.String() {
			// configure pilot (istiod) settings
			if istiod.WorkloadCertTTLDefault != "" {
				if err := setHelmStringValue(values, "pilot.env.DEFAULT_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLDefault); err != nil {
					return err
				}
			}
			if istiod.WorkloadCertTTLMax != "" {
				if err := setHelmStringValue(values, "pilot.env.MAX_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLMax); err != nil {
					return err
				}
			}
		} else {
			// configure security (citadel) settings
			if istiod.WorkloadCertTTLDefault != "" {
				if err := setHelmStringValue(values, "security.workloadCertTtl", istiod.WorkloadCertTTLDefault); err != nil {
					return err
				}
			}
			// can't configure WorkloadCertTTLMax on pre 1.6
		}
		switch istiod.Type {
		case v2.IstioCertificateSignerTypePrivateKey:
			if err := setHelmBoolValue(values, "security.selfSigned", false); err != nil {
				return err
			}
			pksigner := istiod.PrivateKey
			if pksigner == nil {
				// XXX: maybe log a warning
				// let it use its defaults
				break
			}
			if in.Version == versions.V2_0.String() {
				// configure pilot (istiod) settings
				if pksigner.RootCADir != "" {
					if err := setHelmStringValue(values, "pilot.env.ROOT_CA_DIR", pksigner.RootCADir); err != nil {
						return err
					}
				}
				// XXX: nothing else is currently configurable
			} else {
				// configure security (citadel) settings
				// XXX: nothing here is currently configurable for pre-1.6
			}
		case v2.IstioCertificateSignerTypeSelfSigned:
			if err := setHelmBoolValue(values, "security.selfSigned", true); err != nil {
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
				if err := setHelmStringValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_CA_CERT_TTL", selfSigned.TTL); err != nil {
					return err
				}
			}
			if selfSigned.GracePeriod != "" {
				if err := setHelmStringValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE", selfSigned.GracePeriod); err != nil {
					return err
				}
			}
			if selfSigned.CheckPeriod != "" {
				if err := setHelmStringValue(values, componentRoot+".env.CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL", selfSigned.CheckPeriod); err != nil {
					return err
				}
			}
			if selfSigned.EnableJitter != nil {
				if err := setHelmBoolValue(values, componentRoot+".env.CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR", *selfSigned.EnableJitter); err != nil {
					return err
				}
			}
			// XXX: selfSigned.Org is not supported
		case "":
		// don't configure any pilot ca settings
		default:
			return fmt.Errorf("unknown Istiod certificate signer type: %s", istiod.Type)
		}
	case v2.CertificateAuthorityTypeCustom:
		custom := security.MutualTLS.CertificateAuthority.Custom
		if err := setHelmStringValue(values, "pilot.ca.implementation", string(security.MutualTLS.CertificateAuthority.Type)); err != nil {
			return err
		}
		if custom == nil {
			break
		}
		if err := setHelmStringValue(values, "global.caAddress", custom.Address); err != nil {
			return err
		}
	case "":
		// don't configure any ca settings
	default:
		return fmt.Errorf("unknown CertificateAuthority type: %s", security.MutualTLS.CertificateAuthority.Type)
	}

	// Identity
	switch security.MutualTLS.Identity.Type {
	case v2.IdentityConfigTypeKubernetes:
		if err := setHelmStringValue(values, "global.jwtPolicy", "first-party-jwt"); err != nil {
			return err
		}
	case v2.IdentityConfigTypeThirdParty:
		if err := setHelmStringValue(values, "global.jwtPolicy", "third-party-jwt"); err != nil {
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
			if err := setHelmStringValue(values, "pilot.env.TOKEN_ISSUER", tpi.Issuer); err != nil {
				return err
			}
		}
		if tpi.Audience != "" {
			if err := setHelmStringValue(values, "global.sds.token.aud", tpi.Audience); err != nil {
				return err
			}
		}
		// XXX: TokenPath is not currently supported
	case "":
		// don't configure any identity settings
	default:
		return fmt.Errorf("unknown Identity type: %s", security.MutualTLS.Identity.Type)
	}

	// Control Plane Security
	if security.MutualTLS.ControlPlane.Enable != nil {
		if err := setHelmBoolValue(values, "global.controlPlaneSecurityEnabled", *security.MutualTLS.ControlPlane.Enable); err != nil {
			return err
		}
	}
	if security.MutualTLS.ControlPlane.CertProvider != "" {
		if err := setHelmStringValue(values, "global.pilotCertProvider", strings.ToLower(string(security.MutualTLS.ControlPlane.CertProvider))); err != nil {
			return err
		}
	}

	return nil
}

func populateSecurityConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	security := &v2.SecurityConfig{}
	setSecurity := false

	// General mutual TLS
	if mtlsEnabled, ok, err := in.GetBool("global.mtls.enabled"); ok {
		security.MutualTLS.Enable = &mtlsEnabled
		setSecurity = true
	} else if err != nil {
		return err
	}
	if autoMtlsEnabled, ok, err := in.GetBool("global.mtls.auto"); ok {
		security.MutualTLS.Auto = &autoMtlsEnabled
		setSecurity = true
	} else if err != nil {
		return err
	}

	// SPIFFE trust domain
	if trustDomain, ok, err := in.GetString("global.trustDomain"); ok {
		security.MutualTLS.Trust.Domain = trustDomain
		setSecurity = true
	} else if err != nil {
		return err
	}
	if trustDomainAliases, ok, err := in.GetStringSlice("global.trustDomainAliases"); ok {
		security.MutualTLS.Trust.AdditionalDomains = trustDomainAliases
		setSecurity = true
	} else if err != nil {
		return err
	}

	// CA
	var caType v2.CertificateAuthorityType
	var istiodCAType v2.IstioCertificateSignerType
	if caImplementation, ok, err := in.GetString("pilot.ca.implementation"); ok && caImplementation != "" {
		caType = v2.CertificateAuthorityType(caImplementation)
	} else if err != nil {
		return err
	}
	if selfSigned, ok, err := in.GetBool("security.selfSigned"); ok {
		if caType == "" {
			caType = v2.CertificateAuthorityTypeIstiod
		}
		if selfSigned {
			istiodCAType = v2.IstioCertificateSignerTypeSelfSigned
		} else {
			istiodCAType = v2.IstioCertificateSignerTypePrivateKey
		}
	} else if err != nil {
		return err
	}
	switch caType {
	case v2.CertificateAuthorityTypeIstiod:
		setSecurity = true
		security.MutualTLS.CertificateAuthority = v2.CertificateAuthorityConfig{
			Type:   v2.CertificateAuthorityTypeIstiod,
			Istiod: &v2.IstiodCertificateAuthorityConfig{},
		}
		istiod := security.MutualTLS.CertificateAuthority.Istiod
		switch istiodCAType {
		case v2.IstioCertificateSignerTypeSelfSigned:
			istiod.Type = v2.IstioCertificateSignerTypeSelfSigned
			selfSignedConfig := &v2.IstioSelfSignedCertificateSignerConfig{}
			setSelfSigned := false
			if ttl, ok, err := in.GetString("pilot.env.CITADEL_SELF_SIGNED_CA_CERT_TTL"); ok {
				selfSignedConfig.TTL = ttl
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if ttl, ok, err := in.GetString("security.env.CITADEL_SELF_SIGNED_CA_CERT_TTL"); ok {
				selfSignedConfig.TTL = ttl
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if gracePeriod, ok, err := in.GetString("pilot.env.CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE"); ok {
				selfSignedConfig.GracePeriod = gracePeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if gracePeriod, ok, err := in.GetString("security.env.CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE"); ok {
				selfSignedConfig.GracePeriod = gracePeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if checkPeriod, ok, err := in.GetString("pilot.env.CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL"); ok {
				selfSignedConfig.CheckPeriod = checkPeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if checkPeriod, ok, err := in.GetString("security.env.CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL"); ok {
				selfSignedConfig.CheckPeriod = checkPeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if enableJitter, ok, err := in.GetBool("pilot.env.CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR"); ok {
				selfSignedConfig.EnableJitter = &enableJitter
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if enableJitter, ok, err := in.GetBool("security.env.CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR"); ok {
				selfSignedConfig.EnableJitter = &enableJitter
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if setSelfSigned {
				istiod.SelfSigned = selfSignedConfig
			}
		case v2.IstioCertificateSignerTypePrivateKey:
			istiod.Type = v2.IstioCertificateSignerTypePrivateKey
			if rootCADir, ok, err := in.GetString("pilot.env.ROOT_CA_DIR"); ok {
				istiod.PrivateKey = &v2.IstioPrivateKeyCertificateSignerConfig{
					RootCADir: rootCADir,
				}
			} else if err != nil {
				return err
			}
		}
		if workloadCertTTLDefault, ok, err := in.GetString("pilot.env.DEFAULT_WORKLOAD_CERT_TTL"); ok {
			istiod.WorkloadCertTTLDefault = workloadCertTTLDefault
		} else if err != nil {
			return err
		} else if workloadCertTTLDefault, ok, err := in.GetString("security.workloadCertTtl"); ok {
			istiod.WorkloadCertTTLDefault = workloadCertTTLDefault
		} else if err != nil {
			return err
		}
		if workloadCertTTLMax, ok, err := in.GetString("pilot.env.MAX_WORKLOAD_CERT_TTL"); ok {
			istiod.WorkloadCertTTLMax = workloadCertTTLMax
		} else if err != nil {
			return err
		}
	case v2.CertificateAuthorityTypeCustom:
		security.MutualTLS.CertificateAuthority = v2.CertificateAuthorityConfig{
			Type:   v2.CertificateAuthorityTypeCustom,
			Custom: &v2.CustomCertificateAuthorityConfig{},
		}
		if caAddress, ok, err := in.GetString("global.caAddress"); ok {
			security.MutualTLS.CertificateAuthority.Custom.Address = caAddress
			setSecurity = true
		} else if err != nil {
			return err
		}
	case "":
		// don't configure CA
	}

	// Identity
	if jwtPolicy, ok, err := in.GetString("global.jwtPolicy"); ok {
		if identityType, err := getIdentityTypeFromJWTPolicy(jwtPolicy); err == nil {
			switch identityType {
			case v2.IdentityConfigTypeKubernetes:
				setSecurity = true
				security.MutualTLS.Identity.Type = v2.IdentityConfigTypeKubernetes
			case v2.IdentityConfigTypeThirdParty:
				setSecurity = true
				security.MutualTLS.Identity.Type = v2.IdentityConfigTypeThirdParty
				thirdPartyConfig := &v2.ThirdPartyIdentityConfig{}
				setThirdParty := false
				if issuer, ok, err := in.GetString("pilot.env.TOKEN_ISSUER"); ok {
					thirdPartyConfig.Issuer = issuer
					setThirdParty = true
					setSecurity = true
				} else if err != nil {
					return err
				}
				if audience, ok, err := in.GetString("global.sds.token.aud"); ok {
					thirdPartyConfig.Audience = audience
					setThirdParty = true
					setSecurity = true
				} else if err != nil {
					return err
				}
				if setThirdParty {
					security.MutualTLS.Identity.ThirdParty = thirdPartyConfig
				}
			}
		} else {
			return err
		}
	} else if err != nil {
		return err
	}

	// Control Plane Security
	if controlPlaneSecurityEnabled, ok, err := in.GetBool("global.controlPlaneSecurityEnabled"); ok {
		security.MutualTLS.ControlPlane.Enable = &controlPlaneSecurityEnabled
		setSecurity = true
	} else if err != nil {
		return err
	}
	if pilotCertProvider, ok, err := in.GetString("global.pilotCertProvider"); ok {
		if pilotCertProviderType, err := getPilotCertProviderType(pilotCertProvider); err == nil {
			if pilotCertProviderType != "" {
				security.MutualTLS.ControlPlane.CertProvider = pilotCertProviderType
				setSecurity = true
			}
		} else {
			return err
		}
	} else if err != nil {
		return err
	}

	if setSecurity {
		out.Security = security
	}

	return nil
}

func getIdentityTypeFromJWTPolicy(jwtPolicy string) (v2.IdentityConfigType, error) {
	switch jwtPolicy {
	case "first-party-jwt":
		return v2.IdentityConfigTypeKubernetes, nil
	case "third-party-jwt":
		return v2.IdentityConfigTypeThirdParty, nil
	case "":
		return "", nil
	}
	return "", fmt.Errorf("unknown jwtPolicy specified: %s", jwtPolicy)
}

func getPilotCertProviderType(pilotCertProvider string) (v2.ControlPlaneCertProviderType, error) {
	switch strings.ToLower(pilotCertProvider) {
	case "istiod":
		return v2.ControlPlaneCertProviderTypeIstiod, nil
	case "kubernetes":
		return v2.ControlPlaneCertProviderTypeKubernetes, nil
	case "custom":
		return v2.ControlPlaneCertProviderTypeCustom, nil
	case "":
		return "", nil
	}
	return "", fmt.Errorf("unknown pilotCertProvider specified: %s", pilotCertProvider)
}
