package conversion

import (
	"strconv"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	corev1 "k8s.io/api/core/v1"
)

func populateProxyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	proxy := in.Proxy
	if proxy == nil {
		proxy = &v2.ProxyConfig{}
	}
	proxyValues := make(map[string]interface{})

	// General
	if proxy.Concurrency != nil {
		if err := setHelmIntValue(proxyValues, "concurrency", int64(*proxy.Concurrency)); err != nil {
			return err
		}
	}
	// XXX: admin port is not configurable

	// Logging
	if err := populateProxyLogging(&proxy.Logging, proxyValues); err != nil {
		return err
	}

	// Networking
	if proxy.Networking.ClusterDomain != "" {
		if err := setHelmStringValue(proxyValues, "clusterDomain", proxy.Networking.ClusterDomain); err != nil {
			return err
		}
	}
	// XXX: proxy.Networking.ConnectionTimeout is not exposed through values
	switch proxy.Networking.Initialization.Type {
	case v2.ProxyNetworkInitTypeCNI, "":
		istioCNI := make(map[string]interface{})
		if err := setHelmBoolValue(istioCNI, "enabled", true); err != nil {
			return err
		}
		cni := proxy.Networking.Initialization.CNI
		if cni != nil && cni.Runtime != nil {
			if cni.Runtime.PriorityClassName != "" {
				if err := setHelmStringValue(istioCNI, "priorityClassName", cni.Runtime.PriorityClassName); err != nil {
					return err
				}
			}
			if len(cni.Runtime.ImagePullSecrets) > 0 {
				pullSecretsValues := make([]string, 0)
				for _, secret := range cni.Runtime.ImagePullSecrets {
					pullSecretsValues = append(pullSecretsValues, secret.Name)
				}
				if err := setHelmStringSliceValue(istioCNI, "imagePullSecrets", pullSecretsValues); err != nil {
					return err
				}
			}
			if cni.Runtime.ImagePullPolicy != "" {
				if err := setHelmStringValue(istioCNI, "imagePullPolicy", string(cni.Runtime.ImagePullPolicy)); err != nil {
					return err
				}
			}
			if cni.Runtime.Resources != nil {
				if resourcesValues, err := toValues(cni.Runtime.Resources); err == nil {
					if len(resourcesValues) > 0 {
						if err := setHelmValue(istioCNI, "resources", resourcesValues); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
		}
		if err := setHelmValue(values, "istio_cni", istioCNI); err != nil {
			return err
		}
	case v2.ProxyNetworkInitTypeInitContainer:
		if err := setHelmBoolValue(values, "istio_cni.enabled", false); err != nil {
			return err
		}
		if proxy.Networking.Initialization.InitContainer != nil && proxy.Networking.Initialization.InitContainer.Runtime != nil {
			container := proxy.Networking.Initialization.InitContainer.Runtime
			if container.Image != "" {
				if err := setHelmStringValue(values, "global.proxy_init.image", container.Image); err != nil {
					return err
				}
			}
			if container.Resources != nil {
				if resourcesValues, err := toValues(container.Resources); err == nil {
					if len(resourcesValues) > 0 {
						if err := setHelmValue(values, "global.proxy_init.resources", resourcesValues); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
		}
	}

	// Traffic Control
	// Inbound
	// XXX: interceptionMode is not configurable through values.yaml
	if len(proxy.Networking.TrafficControl.Inbound.IncludedPorts) > 0 {
		if err := setHelmStringValue(proxyValues, "includeInboundPorts", strings.Join(proxy.Networking.TrafficControl.Inbound.IncludedPorts, ",")); err != nil {
			return err
		}
	}
	if len(proxy.Networking.TrafficControl.Inbound.ExcludedPorts) > 0 {
		if err := setHelmStringValue(proxyValues, "excludeInboundPorts", strings.Join(proxy.Networking.TrafficControl.Inbound.ExcludedPorts, ",")); err != nil {
			return err
		}
	}
	// Outbound
	if len(proxy.Networking.TrafficControl.Outbound.IncludedIPRanges) > 0 {
		if err := setHelmStringValue(proxyValues, "includeIPRanges", strings.Join(proxy.Networking.TrafficControl.Outbound.IncludedIPRanges, ",")); err != nil {
			return err
		}
	}
	if len(proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges) > 0 {
		if err := setHelmStringValue(proxyValues, "excludeIPRanges", strings.Join(proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges, ",")); err != nil {
			return err
		}
	}
	if len(proxy.Networking.TrafficControl.Outbound.ExcludedPorts) > 0 {
		excludedPorts := make([]string, len(proxy.Networking.TrafficControl.Outbound.ExcludedPorts))
		for index, port := range proxy.Networking.TrafficControl.Outbound.ExcludedPorts {
			excludedPorts[index] = strconv.FormatInt(int64(port), 10)
		}
		if err := setHelmStringValue(proxyValues, "excludeOutboundPorts", strings.Join(excludedPorts, ",")); err != nil {
			return err
		}
	}
	if proxy.Networking.TrafficControl.Outbound.Policy != "" {
		if err := setHelmStringValue(values, "global.outboundTrafficPolicy.mode", string(proxy.Networking.TrafficControl.Outbound.Policy)); err != nil {
			return err
		}
	}

	// Protocol
	if proxy.Networking.Protocol.DetectionTimeout != "" {
		if err := setHelmStringValue(proxyValues, "protocolDetectionTimeout", proxy.Networking.Protocol.DetectionTimeout); err != nil {
			return err
		}
	}
	if proxy.Networking.Protocol.Debug != nil {
		if err := setHelmBoolValue(values, "pilot.enableProtocolSniffingForInbound", proxy.Networking.Protocol.Debug.EnableInboundSniffing); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "pilot.enableProtocolSniffingForOutbound", proxy.Networking.Protocol.Debug.EnableOutboundSniffing); err != nil {
			return err
		}
	}

	// DNS
	if len(proxy.Networking.DNS.SearchSuffixes) > 0 {
		if err := setHelmStringSliceValue(values, "global.podDNSSearchNamespaces", proxy.Networking.DNS.SearchSuffixes); err != nil {
			return err
		}
	}
	if proxy.Networking.DNS.RefreshRate != "" {
		if err := setHelmStringValue(proxyValues, "dnsRefreshRate", proxy.Networking.DNS.RefreshRate); err != nil {
			return err
		}
	}

	// Runtime
	if proxy.Runtime.Resources != nil {
		if resourcesValues, err := toValues(proxy.Runtime.Resources); err == nil {
			if len(resourcesValues) > 0 {
				if err := setHelmValue(proxyValues, "resources", resourcesValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	// Readiness
	if proxy.Runtime.Readiness.StatusPort > 0 {
		if err := setHelmIntValue(proxyValues, "statusPort", int64(proxy.Runtime.Readiness.StatusPort)); err != nil {
			return err
		}
		if proxy.Runtime.Readiness.InitialDelaySeconds > 0 {
			if err := setHelmIntValue(proxyValues, "readinessInitialDelaySeconds", int64(proxy.Runtime.Readiness.InitialDelaySeconds)); err != nil {
				return err
			}
		}
		if proxy.Runtime.Readiness.PeriodSeconds > 0 {
			if err := setHelmIntValue(proxyValues, "readinessPeriodSeconds", int64(proxy.Runtime.Readiness.PeriodSeconds)); err != nil {
				return err
			}
		}
		if proxy.Runtime.Readiness.FailureThreshold > 0 {
			if err := setHelmIntValue(proxyValues, "readinessFailureThreshold", int64(proxy.Runtime.Readiness.FailureThreshold)); err != nil {
				return err
			}
		}
		if err := setHelmBoolValue(values, "sidecarInjectorWebhook.rewriteAppHTTPProbe", proxy.Runtime.Readiness.RewriteApplicationProbes); err != nil {
			return err
		}
	}

	// set proxy values
	if len(proxyValues) > 0 {
		if err := setHelmValue(values, "global.proxy", proxyValues); err != nil {
			return err
		}
	}

	return nil
}

func populateProxyConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	proxy := &v2.ProxyConfig{}
	setProxy := false
	rawProxyValues, ok, err := in.GetMap("proxy")
	if err != nil {
		return err
	} else if !ok || len(rawProxyValues) == 0 {
		rawProxyValues = make(map[string]interface{})
	}
	proxyValues := v1.NewHelmValues(rawProxyValues)

	// General
	if rawConcurrency, ok, err := proxyValues.GetInt64("concurrency"); ok {
		concurrency := int32(rawConcurrency)
		proxy.Concurrency = &concurrency
		setProxy = true
	} else if err != nil {
		return err
	}

	// Logging
	if applied, err := populateProxyLoggingConfig(proxyValues, &proxy.Logging); err != nil {
		return err
	} else if applied {
		setProxy = true
	}

	// Networking
	if clusterDomain, ok, err := proxyValues.GetString("clusterDomain"); ok {
		proxy.Networking.ClusterDomain = clusterDomain
		setProxy = true
	} else if err != nil {
		return err
	}

	if rawIstioCNI, _, err := in.GetMap("istio_cni"); err == nil {
		istioCNI := v1.NewHelmValues(rawIstioCNI)
		if cniEnabled, ok, err := istioCNI.GetBool("enabled"); err != nil {
			return err
		} else if !ok || cniEnabled {
			// cni enabled (default is cni)
			cniConfig := &v2.ProxyCNIConfig{
				Runtime: &v2.ProxyCNIRuntimeConfig{},
			}
			setCNI := false
			if priorityClassName, ok, err := istioCNI.GetString("priorityClassName"); ok {
				cniConfig.Runtime.PriorityClassName = priorityClassName
				setCNI = true
			} else if err != nil {
				return err
			}
			if applied, err := populateContainerConfig(istioCNI, &cniConfig.Runtime.ContainerConfig); err != nil {
				return err
			} else if applied {
				setCNI = true
			}
			if setCNI {
				proxy.Networking.Initialization.Type = v2.ProxyNetworkInitTypeCNI
				proxy.Networking.Initialization.CNI = cniConfig
				setProxy = true
			}
		} else if rawProxyInit, ok, err := in.GetMap("global.proxy_init"); ok && len(rawProxyInit) > 0 {
			// user must explicitly disable cni (although, operator is hard coded
			// to use it, so this should really only configure runtime details
			// for the container)
			proxyInitConfig := &v2.ProxyInitContainerConfig{
				Runtime: &v2.ContainerConfig{},
			}
			if applied, err := populateContainerConfig(v1.NewHelmValues(rawProxyInit), proxyInitConfig.Runtime); err != nil {
				return err
			} else if applied {
				proxy.Networking.Initialization.Type = v2.ProxyNetworkInitTypeInitContainer
				proxy.Networking.Initialization.InitContainer = proxyInitConfig
				setProxy = true
			}
		} else if err != nil {
			return err
		}
	} else {
		return err
	}

	// Traffic Control
	// Inbound
	if includeInboundPorts, ok, err := proxyValues.GetString("includeInboundPorts"); ok && includeInboundPorts != "" {
		proxy.Networking.TrafficControl.Inbound.IncludedPorts = strings.Split(includeInboundPorts, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeInboundPorts, ok, err := proxyValues.GetString("excludeInboundPorts"); ok && excludeInboundPorts != "" {
		proxy.Networking.TrafficControl.Inbound.ExcludedPorts = strings.Split(excludeInboundPorts, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	// Outbound
	if includeIPRanges, ok, err := proxyValues.GetString("includeIPRanges"); ok && includeIPRanges != "" {
		proxy.Networking.TrafficControl.Outbound.IncludedIPRanges = strings.Split(includeIPRanges, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeIPRanges, ok, err := proxyValues.GetString("excludeIPRanges"); ok && excludeIPRanges != "" {
		proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges = strings.Split(excludeIPRanges, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeOutboundPorts, ok, err := proxyValues.GetString("excludeOutboundPorts"); ok && excludeOutboundPorts != "" {
		portSlice := strings.Split(excludeOutboundPorts, ",")
		proxy.Networking.TrafficControl.Outbound.ExcludedPorts = make([]int32, len(portSlice))
		for index, port := range portSlice {
			intPort, err := strconv.ParseInt(port, 10, 32)
			if err != nil {
				return err
			}
			proxy.Networking.TrafficControl.Outbound.ExcludedPorts[index] = int32(intPort)
		}
		setProxy = true
	} else if err != nil {
		return err
	}
	if outboundTrafficPolicy, ok, err := in.GetString("global.outboundTrafficPolicy.mode"); ok && outboundTrafficPolicy != "" {
		proxy.Networking.TrafficControl.Outbound.Policy = v2.ProxyOutboundTrafficPolicy(outboundTrafficPolicy)
		setProxy = true
	} else if err != nil {
		return err
	}

	// Protocol
	if protocolDetectionTimeout, ok, err := proxyValues.GetString("protocolDetectionTimeout"); ok && protocolDetectionTimeout != "" {
		proxy.Networking.Protocol.DetectionTimeout = protocolDetectionTimeout
		setProxy = true
	} else if err != nil {
		return err
	}
	// Protocol Debug
	protocolDebugConfig := &v2.ProxyNetworkProtocolDebugConfig{}
	setProtocolDebug := false
	if enableProtocolSniffingForInbound, ok, err := in.GetBool("pilot.enableProtocolSniffingForInbound"); ok {
		protocolDebugConfig.EnableInboundSniffing = enableProtocolSniffingForInbound
		setProtocolDebug = true
	} else if err != nil {
		return err
	}
	if enableProtocolSniffingForOutbound, ok, err := in.GetBool("pilot.enableProtocolSniffingForOutbound"); ok {
		protocolDebugConfig.EnableOutboundSniffing = enableProtocolSniffingForOutbound
		setProtocolDebug = true
	} else if err != nil {
		return err
	}
	if setProtocolDebug {
		proxy.Networking.Protocol.Debug = protocolDebugConfig
		setProxy = true
	}

	// DNS
	if podDNSSearchNamespaces, ok, err := in.GetStringSlice("global.podDNSSearchNamespaces"); ok && len(podDNSSearchNamespaces) > 0 {
		proxy.Networking.DNS.SearchSuffixes = podDNSSearchNamespaces
		setProxy = true
	} else if err != nil {
		return err
	}
	if dnsRefreshRate, ok, err := proxyValues.GetString("dnsRefreshRate"); ok && dnsRefreshRate != "" {
		proxy.Networking.DNS.RefreshRate = dnsRefreshRate
		setProxy = true
	} else if err != nil {
		return err
	}

	// Runtime
	if resourcesValues, ok, err := proxyValues.GetMap("resources"); ok && len(resourcesValues) > 0 {
		proxy.Runtime.Resources = &corev1.ResourceRequirements{}
		if err := fromValues(resourcesValues, proxy.Runtime.Resources); err != nil {
			return err
		}
		setProxy = true
	} else if err != nil {
		return err
	}
	// Readiness
	if statusPort, ok, err := proxyValues.GetInt64("statusPort"); ok && statusPort > 0 {
		proxy.Runtime.Readiness.StatusPort = int32(statusPort)
		setProxy = true
	} else if err != nil {
		return err
	}
	if readinessInitialDelaySeconds, ok, err := proxyValues.GetInt64("readinessInitialDelaySeconds"); ok && readinessInitialDelaySeconds > 0 {
		proxy.Runtime.Readiness.InitialDelaySeconds = int32(readinessInitialDelaySeconds)
		setProxy = true
	} else if err != nil {
		return err
	}
	if readinessPeriodSeconds, ok, err := proxyValues.GetInt64("readinessPeriodSeconds"); ok && readinessPeriodSeconds > 0 {
		proxy.Runtime.Readiness.PeriodSeconds = int32(readinessPeriodSeconds)
		setProxy = true
	} else if err != nil {
		return err
	}
	if readinessFailureThreshold, ok, err := proxyValues.GetInt64("readinessFailureThreshold"); ok && readinessFailureThreshold > 0 {
		proxy.Runtime.Readiness.FailureThreshold = int32(readinessFailureThreshold)
		setProxy = true
	} else if err != nil {
		return err
	}
	if rewriteAppHTTPProbe, ok, err := in.GetBool("sidecarInjectorWebhook.rewriteAppHTTPProbe"); ok {
		proxy.Runtime.Readiness.RewriteApplicationProbes = rewriteAppHTTPProbe
		setProxy = true
	} else if err != nil {
		return err
	}

	if setProxy {
		out.Proxy = proxy
	}

	return nil
}
