package conversion

import (
	"fmt"
	"strconv"
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

	if security.MutualTLS != nil {
		// General mutual TLS
		if security.MutualTLS.Enabled != nil {
			if err := setHelmBoolValue(values, "global.mtls.enabled", *security.MutualTLS.Enabled); err != nil {
				return err
			}
		}
		if security.MutualTLS.Auto != nil {
			if err := setHelmBoolValue(values, "global.mtls.auto", *security.MutualTLS.Auto); err != nil {
				return err
			}
		}
	}

	// SPIFFE trust domain
	if security.Trust != nil {
		if security.Trust.Domain != "" {
			if err := setHelmStringValue(values, "global.trustDomain", security.Trust.Domain); err != nil {
				return err
			}
		}
		if security.Trust.AdditionalDomains != nil {
			if err := setHelmStringSliceValue(values, "global.trustDomainAliases", security.Trust.AdditionalDomains); err != nil {
				return err
			}
		}
	}

	// CA
	if security.CertificateAuthority != nil {
		switch security.CertificateAuthority.Type {
		case v2.CertificateAuthorityTypeIstiod:
			istiod := security.CertificateAuthority.Istiod
			if err := setHelmStringValue(values, "pilot.ca.implementation", string(security.CertificateAuthority.Type)); err != nil {
				return err
			}
			if istiod == nil {
				break
			}
			if in.Version == versions.V2_0.String() {
				// configure pilot (istiod) settings
				if istiod.WorkloadCertTTLDefault != "" {
					addEnvToComponent(in, "pilot", "DEFAULT_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLDefault)
				}
				if istiod.WorkloadCertTTLMax != "" {
					addEnvToComponent(in, "pilot", "MAX_WORKLOAD_CERT_TTL", istiod.WorkloadCertTTLMax)
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
						addEnvToComponent(in, "pilot", "ROOT_CA_DIR", pksigner.RootCADir)
					}
					// XXX: nothing else is currently configurable
				} else {
					// configure security (citadel) settings
					// XXX: nothing here is currently configurable for pre-1.6
					if pksigner.RootCADir != "" {
						// to support roundtripping
						addEnvToComponent(in, "pilot", "ROOT_CA_DIR", pksigner.RootCADir)
					}
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
					addEnvToComponent(in, componentRoot, "CITADEL_SELF_SIGNED_CA_CERT_TTL", selfSigned.TTL)
				}
				if selfSigned.GracePeriod != "" {
					addEnvToComponent(in, componentRoot, "CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE", selfSigned.GracePeriod)
				}
				if selfSigned.CheckPeriod != "" {
					addEnvToComponent(in, componentRoot, "CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL", selfSigned.CheckPeriod)
				}
				if selfSigned.EnableJitter != nil {
					addEnvToComponent(in, componentRoot, "CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR", strconv.FormatBool(*selfSigned.EnableJitter))
				}
				// XXX: selfSigned.Org is not supported
			case "":
			// don't configure any pilot ca settings
			default:
				return fmt.Errorf("unknown Istiod certificate signer type: %s", istiod.Type)
			}
		case v2.CertificateAuthorityTypeCustom:
			custom := security.CertificateAuthority.Custom
			if err := setHelmStringValue(values, "pilot.ca.implementation", string(security.CertificateAuthority.Type)); err != nil {
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
			return fmt.Errorf("unknown CertificateAuthority type: %s", security.CertificateAuthority.Type)
		}
	}

	// Identity
	if security.Identity != nil {
		switch security.Identity.Type {
		case v2.IdentityConfigTypeKubernetes:
			if err := setHelmStringValue(values, "global.jwtPolicy", "first-party-jwt"); err != nil {
				return err
			}
		case v2.IdentityConfigTypeThirdParty:
			if err := setHelmStringValue(values, "global.jwtPolicy", "third-party-jwt"); err != nil {
				return err
			}
			tpi := security.Identity.ThirdParty
			if tpi == nil {
				// XXX: maybe log a warning
				// let it use its defaults
				break
			}
			if tpi.Issuer != "" {
				// XXX: only supported in 1.6+
				addEnvToComponent(in, "pilot", "TOKEN_ISSUER", tpi.Issuer)
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
			return fmt.Errorf("unknown Identity type: %s", security.Identity.Type)
		}
	}

	// Control Plane Security
	if security.ControlPlane != nil {
		if security.ControlPlane.MTLS != nil {
			if err := setHelmBoolValue(values, "global.controlPlaneSecurityEnabled", *security.ControlPlane.MTLS); err != nil {
				return err
			}
		}
		if security.ControlPlane.CertProvider != "" {
			if err := setHelmStringValue(values, "global.pilotCertProvider", strings.ToLower(string(security.ControlPlane.CertProvider))); err != nil {
				return err
			}
		}
		if security.ControlPlane.TLS != nil {
			tls := security.ControlPlane.TLS
			if len(tls.CipherSuites) > 0 {
				if err := setHelmStringValue(values, "global.tls.cipherSuites", strings.Join(tls.CipherSuites, ",")); err != nil {
					return err
				}
			}
			if len(tls.ECDHCurves) > 0 {
				if err := setHelmStringValue(values, "global.tls.ecdhCurves", strings.Join(tls.ECDHCurves, ",")); err != nil {
					return err
				}
			}
			if len(tls.MinProtocolVersion) > 0 {
				if err := setHelmStringValue(values, "global.tls.minProtocolVersion", tls.MinProtocolVersion); err != nil {
					return err
				}
			}
			if len(tls.MaxProtocolVersion) > 0 {
				if err := setHelmStringValue(values, "global.tls.maxProtocolVersion", tls.MaxProtocolVersion); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func populateSecurityConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	security := &v2.SecurityConfig{}
	setSecurity := false

	// General mutual TLS
	mutualTLS := &v2.MutualTLSConfig{}
	setMutualTLS := false
	if mtlsEnabled, ok, err := in.GetBool("global.mtls.enabled"); ok {
		mutualTLS.Enabled = &mtlsEnabled
		setMutualTLS = true
	} else if err != nil {
		return err
	}
	if autoMtlsEnabled, ok, err := in.GetBool("global.mtls.auto"); ok {
		mutualTLS.Auto = &autoMtlsEnabled
		setMutualTLS = true
	} else if err != nil {
		return err
	}
	if setMutualTLS {
		security.MutualTLS = mutualTLS
		setSecurity = true
	}

	// SPIFFE trust domain
	trust := &v2.TrustConfig{}
	setTrust := false
	if trustDomain, ok, err := in.GetString("global.trustDomain"); ok {
		trust.Domain = trustDomain
		setTrust = true
	} else if err != nil {
		return err
	}
	if trustDomainAliases, ok, err := in.GetStringSlice("global.trustDomainAliases"); ok {
		trust.AdditionalDomains = trustDomainAliases
		setTrust = true
	} else if err != nil {
		return err
	}
	if setTrust {
		security.Trust = trust
		setSecurity = true
	}

	// CA
	ca := &v2.CertificateAuthorityConfig{}
	setCA := false
	var caType v2.CertificateAuthorityType
	var istiodCAType v2.IstioCertificateSignerType
	if caImplementation, ok, err := in.GetString("pilot.ca.implementation"); ok && caImplementation != "" {
		caType = v2.CertificateAuthorityType(caImplementation)
		setCA = true
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
		setCA = true
		ca.Type = v2.CertificateAuthorityTypeIstiod
		ca.Istiod = &v2.IstiodCertificateAuthorityConfig{}
		istiod := ca.Istiod
		switch istiodCAType {
		case v2.IstioCertificateSignerTypeSelfSigned:
			istiod.Type = v2.IstioCertificateSignerTypeSelfSigned
			selfSignedConfig := &v2.IstioSelfSignedCertificateSignerConfig{}
			setSelfSigned := false
			if ttl, ok, err := getAndClearComponentEnv(in, "pilot", "CITADEL_SELF_SIGNED_CA_CERT_TTL"); ok {
				selfSignedConfig.TTL = ttl
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if ttl, ok, err := getAndClearComponentEnv(in, "security", "CITADEL_SELF_SIGNED_CA_CERT_TTL"); ok {
				selfSignedConfig.TTL = ttl
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if gracePeriod, ok, err := getAndClearComponentEnv(in, "pilot", "CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE"); ok {
				selfSignedConfig.GracePeriod = gracePeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if gracePeriod, ok, err := getAndClearComponentEnv(in, "security", "CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE"); ok {
				selfSignedConfig.GracePeriod = gracePeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if checkPeriod, ok, err := getAndClearComponentEnv(in, "pilot", "CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL"); ok {
				selfSignedConfig.CheckPeriod = checkPeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			} else if checkPeriod, ok, err := getAndClearComponentEnv(in, "security", "CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL"); ok {
				selfSignedConfig.CheckPeriod = checkPeriod
				setSelfSigned = true
			} else if err != nil {
				return err
			}
			if rawnEableJitter, ok, err := getAndClearComponentEnv(in, "pilot", "CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR"); ok {
				if enableJitter, err := strconv.ParseBool(rawnEableJitter); err == nil {
					selfSignedConfig.EnableJitter = &enableJitter
					setSelfSigned = true
				} else {
					return err
				}
			} else if err != nil {
				return err
			} else if rawnEableJitter, ok, err := getAndClearComponentEnv(in, "security", "CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR"); ok {
				if enableJitter, err := strconv.ParseBool(rawnEableJitter); err == nil {
					selfSignedConfig.EnableJitter = &enableJitter
					setSelfSigned = true
				} else {
					return err
				}
			} else if err != nil {
				return err
			}
			if setSelfSigned {
				istiod.SelfSigned = selfSignedConfig
			}
		case v2.IstioCertificateSignerTypePrivateKey:
			istiod.Type = v2.IstioCertificateSignerTypePrivateKey
			if rootCADir, ok, err := getAndClearComponentEnv(in, "pilot", "ROOT_CA_DIR"); ok {
				istiod.PrivateKey = &v2.IstioPrivateKeyCertificateSignerConfig{
					RootCADir: rootCADir,
				}
			} else if err != nil {
				return err
			}
		}
		if workloadCertTTLDefault, ok, err := getAndClearComponentEnv(in, "pilot", "DEFAULT_WORKLOAD_CERT_TTL"); ok {
			istiod.WorkloadCertTTLDefault = workloadCertTTLDefault
		} else if err != nil {
			return err
		} else if workloadCertTTLDefault, ok, err := in.GetString("security.workloadCertTtl"); ok {
			istiod.WorkloadCertTTLDefault = workloadCertTTLDefault
		} else if err != nil {
			return err
		}
		if workloadCertTTLMax, ok, err := getAndClearComponentEnv(in, "pilot", "MAX_WORKLOAD_CERT_TTL"); ok {
			istiod.WorkloadCertTTLMax = workloadCertTTLMax
		} else if err != nil {
			return err
		}
	case v2.CertificateAuthorityTypeCustom:
		setCA = true
		ca.Type = v2.CertificateAuthorityTypeCustom
		if caAddress, ok, err := in.GetString("global.caAddress"); ok {
			ca.Custom = &v2.CustomCertificateAuthorityConfig{
				Address: caAddress,
			}
		} else if err != nil {
			return err
		}
	case "":
		// don't configure CA
	}
	if setCA {
		security.CertificateAuthority = ca
		setSecurity = true
	}

	// Identity
	identity := &v2.IdentityConfig{}
	setIdentity := false
	if jwtPolicy, ok, err := in.GetString("global.jwtPolicy"); ok {
		if identityType, err := getIdentityTypeFromJWTPolicy(jwtPolicy); err == nil {
			switch identityType {
			case v2.IdentityConfigTypeKubernetes:
				setIdentity = true
				identity.Type = v2.IdentityConfigTypeKubernetes
			case v2.IdentityConfigTypeThirdParty:
				setIdentity = true
				identity.Type = v2.IdentityConfigTypeThirdParty
				thirdPartyConfig := &v2.ThirdPartyIdentityConfig{}
				setThirdParty := false
				if issuer, ok, err := getAndClearComponentEnv(in, "pilot", "TOKEN_ISSUER"); ok {
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
					identity.ThirdParty = thirdPartyConfig
				}
			}
		} else {
			return err
		}
	} else if err != nil {
		return err
	}
	if setIdentity {
		security.Identity = identity
		setSecurity = true
	}

	// Control Plane Security
	controlPlane := &v2.ControlPlaneSecurityConfig{}
	setControlPlane := false
	if controlPlaneSecurityEnabled, ok, err := in.GetBool("global.controlPlaneSecurityEnabled"); ok {
		controlPlane.MTLS = &controlPlaneSecurityEnabled
		setControlPlane = true
	} else if err != nil {
		return err
	}
	if pilotCertProvider, ok, err := in.GetString("global.pilotCertProvider"); ok {
		if pilotCertProviderType, err := getPilotCertProviderType(pilotCertProvider); err == nil {
			if pilotCertProviderType != "" {
				controlPlane.CertProvider = pilotCertProviderType
				setControlPlane = true
			}
		} else {
			return err
		}
	} else if err != nil {
		return err
	}
	tls := &v2.ControlPlaneTLSConfig{}
	setTLS := false
	if cipherSuites, ok, err := in.GetString("global.tls.cipherSuites"); ok && cipherSuites != "" {
		tls.CipherSuites = strings.Split(cipherSuites, ",")
		setTLS = true
	} else if err != nil {
		return err
	}
	if ecdhCurves, ok, err := in.GetString("global.tls.ecdhCurves"); ok && ecdhCurves != "" {
		tls.ECDHCurves = strings.Split(ecdhCurves, ",")
		setTLS = true
	} else if err != nil {
		return err
	}
	if minProtocolVersion, ok, err := in.GetString("global.tls.minProtocolVersion"); ok {
		tls.MinProtocolVersion = minProtocolVersion
		setTLS = true
	} else if err != nil {
		return err
	}
	if maxProtocolVersion, ok, err := in.GetString("global.tls.maxProtocolVersion"); ok {
		tls.MaxProtocolVersion = maxProtocolVersion
		setTLS = true
	} else if err != nil {
		return err
	}
	if setTLS {
		controlPlane.TLS = tls
		setControlPlane = true
	}
	if setControlPlane {
		security.ControlPlane = controlPlane
		setSecurity = true
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
