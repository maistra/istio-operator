package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
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
	if proxy.AdminPort > 0 {
		if err := setHelmIntValue(proxyValues, "adminPort", int64(proxy.AdminPort)); err != nil {
			return err
		}
	}

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
	if proxy.Networking.ConnectionTimeout != "" {
		if err := setHelmStringValue(proxyValues, "connectionTimeout", proxy.Networking.ConnectionTimeout); err != nil {
			return err
		}
	}
	switch proxy.Networking.Initialization.Type {
	case v2.ProxyNetworkInitTypeCNI:
		if err := setHelmStringValue(proxyValues, "initType", string(proxy.Networking.Initialization.Type)); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "istio_cni.enabled", true); err != nil {
			return err
		}
	case v2.ProxyNetworkInitTypeInitContainer:
		if err := setHelmStringValue(proxyValues, "initType", string(proxy.Networking.Initialization.Type)); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "istio_cni.enabled", false); err != nil {
			return err
		}
		if proxy.Networking.Initialization.InitContainer != nil && proxy.Networking.Initialization.InitContainer.Runtime != nil {
			container := proxy.Networking.Initialization.InitContainer.Runtime
			initValues := make(map[string]interface{})
			if err := populateContainerConfigValues(container, initValues); err == nil {
				if err := setHelmValue(values, "global.proxy_init", initValues); err != nil {
					return err
				}
			}
		}
	}

	// Traffic Control
	// Inbound
	// XXX: InterceptionMode is not configurable through values.yaml
	if proxy.Networking.TrafficControl.Inbound.InterceptionMode != "" {
		if err := setHelmStringValue(proxyValues, "interceptionMode", string(proxy.Networking.TrafficControl.Inbound.InterceptionMode)); err != nil {
			return err
		}
	}
	// includeInboundPorts="" is a valid value == IncludedPorts([]string{""})
	if len(proxy.Networking.TrafficControl.Inbound.IncludedPorts) > 0 {
		if err := setHelmStringValue(proxyValues, "includeInboundPorts", strings.Join(proxy.Networking.TrafficControl.Inbound.IncludedPorts, ",")); err != nil {
			return err
		}
	}
	if proxy.Networking.TrafficControl.Inbound.ExcludedPorts != nil {
		if err := setHelmStringValue(proxyValues, "excludeInboundPorts", int32SliceToString(proxy.Networking.TrafficControl.Inbound.ExcludedPorts)); err != nil {
			return err
		}
	}
	// Outbound
	// includeIPRanges="" is a valid value == IncludedIPRanges([]string{""})
	// XXX: verify this
	if len(proxy.Networking.TrafficControl.Outbound.IncludedIPRanges) > 0 {
		if err := setHelmStringValue(proxyValues, "includeIPRanges", strings.Join(proxy.Networking.TrafficControl.Outbound.IncludedIPRanges, ",")); err != nil {
			return err
		}
	}
	if proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges != nil {
		if err := setHelmStringValue(proxyValues, "excludeIPRanges", strings.Join(proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges, ",")); err != nil {
			return err
		}
	}
	if proxy.Networking.TrafficControl.Outbound.ExcludedPorts != nil {
		if err := setHelmStringValue(proxyValues, "excludeOutboundPorts", int32SliceToString(proxy.Networking.TrafficControl.Outbound.ExcludedPorts)); err != nil {
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
	if proxy.Networking.DNS.SearchSuffixes != nil {
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
	if proxy.Runtime.Container != nil {
		if err := populateContainerConfigValues(proxy.Runtime.Container, proxyValues); err != nil {
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
	rawProxyValues, ok, err := in.GetMap("global.proxy")
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
	if adminPort, ok, err := proxyValues.GetInt64("adminPort"); ok && adminPort > 0 {
		proxy.AdminPort = int32(adminPort)
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
	if connectionTimeout, ok, err := proxyValues.GetString("connectionTimeout"); ok {
		proxy.Networking.ConnectionTimeout = connectionTimeout
		setProxy = true
	} else if err != nil {
		return err
	}

	if initType, ok, err := proxyValues.GetString("initType"); ok {
		proxy.Networking.Initialization.Type = v2.ProxyNetworkInitType(initType)
		switch proxy.Networking.Initialization.Type {
		case v2.ProxyNetworkInitTypeCNI:
			setProxy = true
		case v2.ProxyNetworkInitTypeInitContainer:
			proxy.Networking.Initialization.Type = v2.ProxyNetworkInitTypeInitContainer
			setProxy = true
			if rawProxyInit, ok, err := in.GetMap("global.proxy_init"); ok && len(rawProxyInit) > 0 {
				// user must explicitly disable cni (although, operator is hard coded
				// to use it, so this should really only configure runtime details
				// for the container)
				proxyInitConfig := &v2.ProxyInitContainerConfig{
					Runtime: &v2.ContainerConfig{},
				}
				if applied, err := populateContainerConfig(v1.NewHelmValues(rawProxyInit), proxyInitConfig.Runtime); err != nil {
					return err
				} else if applied {
					proxy.Networking.Initialization.InitContainer = proxyInitConfig
				}
			} else if err != nil {
				return err
			}
		case "":
			// ignore this
		default:
			return fmt.Errorf("unknown proxy init type: %s", proxy.Networking.Initialization.Type)
		}
	} else if err != nil {
		return err
	}

	// Traffic Control
	// Inbound
	// XXX: InterceptionMode is not configurable through values.yaml
	if interceptionMode, ok, err := proxyValues.GetString("interceptionMode"); ok && interceptionMode != "" {
		proxy.Networking.TrafficControl.Inbound.InterceptionMode = v2.ProxyNetworkInterceptionMode(interceptionMode)
		setProxy = true
	} else if err != nil {
		return err
	}
	if includeInboundPorts, ok, err := proxyValues.GetString("includeInboundPorts"); ok {
		proxy.Networking.TrafficControl.Inbound.IncludedPorts = strings.Split(includeInboundPorts, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeInboundPorts, ok, err := proxyValues.GetString("excludeInboundPorts"); ok {
		if proxy.Networking.TrafficControl.Inbound.ExcludedPorts, err = stringToInt32Slice(excludeInboundPorts); err != nil {
			return err
		}
		setProxy = true
	} else if err != nil {
		return err
	}
	// Outbound
	if includeIPRanges, ok, err := proxyValues.GetString("includeIPRanges"); ok {
		proxy.Networking.TrafficControl.Outbound.IncludedIPRanges = strings.Split(includeIPRanges, ",")
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeIPRanges, ok, err := proxyValues.GetString("excludeIPRanges"); ok {
		var ipRangeSlice []string
		if excludeIPRanges == "" {
			ipRangeSlice = make([]string, 0)
		} else {
			ipRangeSlice = strings.Split(excludeIPRanges, ",")
		}
		proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges = ipRangeSlice
		setProxy = true
	} else if err != nil {
		return err
	}
	if excludeOutboundPorts, ok, err := proxyValues.GetString("excludeOutboundPorts"); ok {
		if proxy.Networking.TrafficControl.Outbound.ExcludedPorts, err = stringToInt32Slice(excludeOutboundPorts); err != nil {
			return err
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
	if podDNSSearchNamespaces, ok, err := in.GetStringSlice("global.podDNSSearchNamespaces"); ok {
		if addedSearchSuffixes, ok, err := in.GetStringSlice("global.multiCluster.addedSearchSuffixes"); ok && len(addedSearchSuffixes) > 0 {
			for _, addedSuffix := range addedSearchSuffixes {
				for index, suffix := range podDNSSearchNamespaces {
					if suffix == addedSuffix {
						podDNSSearchNamespaces = append(podDNSSearchNamespaces[:index], podDNSSearchNamespaces[index+1:]...)
						break
					}
				}
			}
		} else if err != nil {
			return err
		}
		if len(podDNSSearchNamespaces) > 0 {
			proxy.Networking.DNS.SearchSuffixes = podDNSSearchNamespaces
			setProxy = true
		}
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
	container := &v2.ContainerConfig{}
	if applied, err := populateContainerConfig(proxyValues, container); err != nil {
		return err
	} else if applied {
		proxy.Runtime.Container = container
		setProxy = true
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
