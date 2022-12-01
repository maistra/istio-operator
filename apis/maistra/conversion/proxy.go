package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/apis/maistra/v2"
)

func populateProxyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	proxy := in.Proxy
	if proxy == nil {
		proxy = &v2.ProxyConfig{}
	}
	proxyValues := make(map[string]interface{})
	meshConfigValues := make(map[string]interface{})

	// General
	if proxy.Concurrency != nil {
		if err := setHelmIntValue(proxyValues, "concurrency", int64(*proxy.Concurrency)); err != nil {
			return err
		}
		if err := setHelmIntValue(meshConfigValues, "concurrency", int64(*proxy.Concurrency)); err != nil {
			return err
		}
	}
	if proxy.AdminPort > 0 {
		// admin port is only configurable through .Values.meshConfig.proxyAdminPort
		if err := setHelmIntValue(proxyValues, "adminPort", int64(proxy.AdminPort)); err != nil {
			return err
		}
		if err := setHelmIntValue(meshConfigValues, "defaultConfig.proxyAdminPort", int64(proxy.AdminPort)); err != nil {
			return err
		}
	}

	// Logging
	if err := populateProxyLogging(proxy.Logging, proxyValues); err != nil {
		return err
	}

	// Networking
	if proxy.Networking != nil {
		if proxy.Networking.ClusterDomain != "" {
			if err := setHelmStringValue(proxyValues, "clusterDomain", proxy.Networking.ClusterDomain); err != nil {
				return err
			}
		}
		if proxy.Networking.ConnectionTimeout != "" {
			// XXX: this is actually MeshConfig.defaultConfig.connectTimeout and is not supported in 2.0+, and wasn't configurable in 1.x
			if err := setHelmStringValue(proxyValues, "connectionTimeout", proxy.Networking.ConnectionTimeout); err != nil {
				return err
			}
		}
		if proxy.Networking.MaxConnectionAge != "" {
			if err := setHelmStringValue(values, "pilot.keepaliveMaxServerConnectionAge", proxy.Networking.MaxConnectionAge); err != nil {
				return err
			}
		}
		if proxy.Networking.Initialization != nil {
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
			}
			if proxy.Networking.Initialization.InitContainer != nil && proxy.Networking.Initialization.InitContainer.Runtime != nil {
				container := proxy.Networking.Initialization.InitContainer.Runtime
				initValues := make(map[string]interface{})
				if err := populateContainerConfigValues(container, initValues); err == nil {
					if err := overwriteHelmValues(values, initValues, "global", "proxy_init"); err != nil {
						return err
					}
				}
			}
		}

		// Traffic Control
		if proxy.Networking.TrafficControl != nil {
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
			if len(proxy.Networking.TrafficControl.Inbound.ExcludedPorts) > 0 {
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
			if len(proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges) > 0 {
				if err := setHelmStringValue(proxyValues, "excludeIPRanges", strings.Join(proxy.Networking.TrafficControl.Outbound.ExcludedIPRanges, ",")); err != nil {
					return err
				}
			}
			if len(proxy.Networking.TrafficControl.Outbound.ExcludedPorts) > 0 {
				if err := setHelmStringValue(proxyValues, "excludeOutboundPorts", int32SliceToString(proxy.Networking.TrafficControl.Outbound.ExcludedPorts)); err != nil {
					return err
				}
			}
			if proxy.Networking.TrafficControl.Outbound.Policy != "" {
				if err := setHelmStringValue(values, "global.outboundTrafficPolicy.mode", string(proxy.Networking.TrafficControl.Outbound.Policy)); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "outboundTrafficPolicy.mode", string(proxy.Networking.TrafficControl.Outbound.Policy)); err != nil {
					return err
				}
			}
		}

		// Protocol
		if proxy.Networking.Protocol != nil && proxy.Networking.Protocol.AutoDetect != nil {
			autoDetect := proxy.Networking.Protocol.AutoDetect
			if autoDetect.Timeout != "" {
				if err := setHelmStringValue(proxyValues, "protocolDetectionTimeout", autoDetect.Timeout); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "protocolDetectionTimeout", autoDetect.Timeout); err != nil {
					return err
				}
			}
			if autoDetect.Inbound != nil {
				if err := setHelmBoolValue(values, "pilot.enableProtocolSniffingForInbound", *autoDetect.Inbound); err != nil {
					return err
				}
			}
			if autoDetect.Outbound != nil {
				if err := setHelmBoolValue(values, "pilot.enableProtocolSniffingForOutbound", *autoDetect.Outbound); err != nil {
					return err
				}
			}
		}

		// DNS
		if proxy.Networking.DNS != nil {
			if proxy.Networking.DNS.SearchSuffixes != nil {
				if err := setHelmStringSliceValue(values, "global.podDNSSearchNamespaces", proxy.Networking.DNS.SearchSuffixes); err != nil {
					return err
				}
			}
			if proxy.Networking.DNS.RefreshRate != "" {
				if err := setHelmStringValue(proxyValues, "dnsRefreshRate", proxy.Networking.DNS.RefreshRate); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "dnsRefreshRate", proxy.Networking.DNS.RefreshRate); err != nil {
					return err
				}
			}
		}
	}

	// Injection
	if proxy.Injection != nil {
		injection := proxy.Injection
		if injection.AutoInject != nil {
			if err := setHelmBoolValue(values, "sidecarInjectorWebhook.enableNamespacesByDefault", *injection.AutoInject); err != nil {
				return err
			}
			if *injection.AutoInject {
				if err := setHelmStringValue(proxyValues, "autoInject", "enabled"); err != nil {
					return err
				}
			} else {
				if err := setHelmStringValue(proxyValues, "autoInject", "disabled"); err != nil {
					return err
				}
			}
		}
		if len(injection.AlwaysInjectSelector) > 0 {
			untypedSlice := make([]interface{}, len(injection.AlwaysInjectSelector))
			for index, value := range injection.AlwaysInjectSelector {
				untypedSlice[index] = value
			}
			if alwaysInjectSelector, err := sliceToValues(untypedSlice); err == nil {
				if err := setHelmValue(values, "sidecarInjectorWebhook.alwaysInjectSelector", alwaysInjectSelector); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if len(injection.NeverInjectSelector) > 0 {
			untypedSlice := make([]interface{}, len(injection.NeverInjectSelector))
			for index, value := range injection.NeverInjectSelector {
				untypedSlice[index] = value
			}
			if neverInjectSelector, err := sliceToValues(untypedSlice); err == nil {
				if err := setHelmValue(values, "sidecarInjectorWebhook.neverInjectSelector", neverInjectSelector); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if len(injection.InjectedAnnotations) > 0 {
			if injectedAnnotations, err := toValues(injection.InjectedAnnotations); err == nil {
				if err := setHelmValue(values, "sidecarInjectorWebhook.injectedAnnotations", injectedAnnotations); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// Access logging
	if proxy.AccessLogging != nil {
		if proxy.AccessLogging.File != nil {
			file := proxy.AccessLogging.File
			if file.Name != "" {
				if err := setHelmStringValue(proxyValues, "accessLogFile", file.Name); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "accessLogFile", file.Name); err != nil {
					return err
				}
			}
			if file.Encoding != "" {
				if err := setHelmStringValue(proxyValues, "accessLogEncoding", file.Encoding); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "accessLogEncoding", file.Encoding); err != nil {
					return err
				}
			}
			if file.Format != "" {
				if err := setHelmStringValue(proxyValues, "accessLogFormat", file.Format); err != nil {
					return err
				}
				if err := setHelmStringValue(meshConfigValues, "accessLogFormat", file.Format); err != nil {
					return err
				}
			}
		}
		if proxy.AccessLogging.EnvoyService != nil {
			accessLogServiceValues := map[string]interface{}{}
			if err := populateProxyEnvoyServiceValues(proxy.AccessLogging.EnvoyService, accessLogServiceValues); err == nil {
				if len(accessLogServiceValues) > 0 {
					if err := setHelmValue(proxyValues, "envoyAccessLogService", accessLogServiceValues); err != nil {
						return err
					}
					if proxy.AccessLogging.EnvoyService.Enabled != nil {
						if err := setHelmValue(meshConfigValues, "enableEnvoyAccessLogService", *proxy.AccessLogging.EnvoyService.Enabled); err != nil {
							return err
						}
						// enableEnvoyAccessLogService does not appear to be checked when configuring the proxy, so this
						// should only be set if enaabled
						if *proxy.AccessLogging.EnvoyService.Enabled {
							if err := setHelmValue(meshConfigValues, "defaultConfig.envoyAccessLogService", accessLogServiceValues); err != nil {
								return err
							}
						}
					}
				}
			} else {
				return err
			}
		}
	}

	// Metrics service
	if proxy.EnvoyMetricsService != nil {
		metricsServiceValues := map[string]interface{}{}
		if err := populateProxyEnvoyServiceValues(proxy.EnvoyMetricsService, metricsServiceValues); err == nil {
			if len(metricsServiceValues) > 0 {
				if err := setHelmValue(proxyValues, "envoyMetricsService", metricsServiceValues); err != nil {
					return err
				}
				if proxy.EnvoyMetricsService.Enabled != nil && *proxy.EnvoyMetricsService.Enabled {
					if err := setHelmValue(meshConfigValues, "defaultConfig.envoyMetricsService", metricsServiceValues); err != nil {
						return err
					}
				}
			}
		} else {
			return err
		}
	}

	// Runtime
	if proxy.Runtime != nil {
		containerConfig := proxy.Runtime.Container
		if containerConfig != nil {
			if err := populateCommonContainerConfigValues(&containerConfig.CommonContainerConfig, proxyValues); err != nil {
				return err
			}
			if containerConfig.Image != "" {
				if err := setHelmStringValue(proxyValues, "image", containerConfig.Image); err != nil {
					return err
				}
			}
			for key, value := range containerConfig.Env {
				if err := setHelmValue(meshConfigValues, "defaultConfig.proxyMetadata."+key, value); err != nil {
					return err
				}
			}
		}

		// Readiness
		if proxy.Runtime.Readiness != nil {
			if proxy.Runtime.Readiness.StatusPort > 0 {
				if err := setHelmIntValue(proxyValues, "statusPort", int64(proxy.Runtime.Readiness.StatusPort)); err != nil {
					return err
				}
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
	}

	// set proxy values
	if len(proxyValues) > 0 {
		if err := overwriteHelmValues(values, proxyValues, "global", "proxy"); err != nil {
			return err
		}
	}
	if len(meshConfigValues) > 0 {
		if err := overwriteHelmValues(values, meshConfigValues, "meshConfig"); err != nil {
			return err
		}
	}

	return nil
}

func populateProxyEnvoyServiceValues(service *v2.ProxyEnvoyServiceConfig, envoyServiceValues map[string]interface{}) error {
	if service.Enabled != nil {
		if err := setHelmBoolValue(envoyServiceValues, "enabled", *service.Enabled); err != nil {
			return err
		}
	}
	if service.Address != "" {
		hostPort := strings.SplitN(service.Address, ":", 2)
		host := hostPort[0]
		port := "80"
		if len(hostPort) > 1 {
			port = hostPort[1]
		}
		if err := setHelmStringValue(envoyServiceValues, "host", host); err != nil {
			return err
		}
		if err := setHelmStringValue(envoyServiceValues, "port", port); err != nil {
			return err
		}
	}
	if service.TCPKeepalive != nil {
		if tcpKeepalive, err := toValues(service.TCPKeepalive); err == nil {
			if len(tcpKeepalive) > 0 {
				if err := setHelmValue(envoyServiceValues, "tcpKeepalive", tcpKeepalive); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	if service.TLSSettings != nil {
		if tlsSettings, err := toValues(service.TLSSettings); err == nil {
			if len(tlsSettings) > 0 {
				if err := setHelmValue(envoyServiceValues, "tlsSettings", tlsSettings); err != nil {
					return err
				}
			}
		} else {
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
	rawMeshConfigValues, ok, err := in.GetMap("meshConfig")
	if err != nil {
		return err
	} else if !ok || len(rawMeshConfigValues) == 0 {
		rawMeshConfigValues = make(map[string]interface{})
	}
	meshConfigValues := v1.NewHelmValues(rawMeshConfigValues)

	// remove auto-populated values
	in.RemoveField("istio_cni.enabled")

	// General
	if rawConcurrency, ok, err := proxyValues.GetAndRemoveInt64("concurrency"); ok {
		concurrency := int32(rawConcurrency)
		proxy.Concurrency = &concurrency
		setProxy = true
	} else if err != nil {
		return err
	}
	// meshConfig values take precedence
	if rawConcurrency, ok, err := meshConfigValues.GetAndRemoveInt64("concurrency"); ok {
		concurrency := int32(rawConcurrency)
		proxy.Concurrency = &concurrency
		setProxy = true
	} else if err != nil {
		return err
	}
	if adminPort, ok, err := proxyValues.GetAndRemoveInt64("adminPort"); ok && adminPort > 0 {
		proxy.AdminPort = int32(adminPort)
		setProxy = true
	} else if err != nil {
		return err
	}
	// meshConfig values take precedence
	if adminPort, ok, err := meshConfigValues.GetAndRemoveInt64("defaultConfig.proxyAdminPort"); ok && adminPort > 0 {
		proxy.AdminPort = int32(adminPort)
		setProxy = true
	} else if err != nil {
		return err
	}

	// Logging
	logging := &v2.ProxyLoggingConfig{}
	if applied, err := populateProxyLoggingConfig(proxyValues, logging); err != nil {
		return err
	} else if applied {
		setProxy = true
		proxy.Logging = logging
	}

	// Networking
	networking := &v2.ProxyNetworkingConfig{}
	setNetworking := false
	if clusterDomain, ok, err := proxyValues.GetAndRemoveString("clusterDomain"); ok && clusterDomain != "" {
		networking.ClusterDomain = clusterDomain
		setNetworking = true
	} else if err != nil {
		return err
	}
	if connectionTimeout, ok, err := proxyValues.GetAndRemoveString("connectionTimeout"); ok {
		networking.ConnectionTimeout = connectionTimeout
		setNetworking = true
	} else if err != nil {
		return err
	}
	if maxConnectionAge, ok, err := in.GetAndRemoveString("pilot.keepaliveMaxServerConnectionAge"); ok {
		networking.MaxConnectionAge = maxConnectionAge
		setNetworking = true
	} else if err != nil {
		return err
	}

	initialization := &v2.ProxyNetworkInitConfig{}
	setInitialization := false
	if initType, ok, err := proxyValues.GetAndRemoveString("initType"); ok {
		initialization.Type = v2.ProxyNetworkInitType(initType)
		switch initialization.Type {
		case v2.ProxyNetworkInitTypeCNI:
			setInitialization = true
		case v2.ProxyNetworkInitTypeInitContainer:
			setInitialization = true
		case "":
			// ignore this
		default:
			return fmt.Errorf("unknown proxy init type: %s", proxy.Networking.Initialization.Type)
		}
	} else if err != nil {
		return err
	}
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
			initialization.InitContainer = proxyInitConfig
			setInitialization = true
		}
		if len(rawProxyInit) == 0 {
			in.RemoveField("global.proxy_init")
		} else if err := in.SetField("global.proxy_init", rawProxyInit); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if setInitialization {
		networking.Initialization = initialization
		setNetworking = true
	}

	// Traffic Control
	trafficControl := &v2.ProxyTrafficControlConfig{}
	setTrafficControl := false
	// Inbound
	// XXX: InterceptionMode is not configurable through values.yaml
	if interceptionMode, ok, err := proxyValues.GetAndRemoveString("interceptionMode"); ok && interceptionMode != "" {
		trafficControl.Inbound.InterceptionMode = v2.ProxyNetworkInterceptionMode(interceptionMode)
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if includeInboundPorts, ok, err := proxyValues.GetAndRemoveString("includeInboundPorts"); ok {
		trafficControl.Inbound.IncludedPorts = strings.Split(includeInboundPorts, ",")
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if excludeInboundPorts, ok, err := proxyValues.GetAndRemoveString("excludeInboundPorts"); ok {
		if trafficControl.Inbound.ExcludedPorts, err = stringToInt32Slice(excludeInboundPorts); err != nil {
			return err
		}
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	// Outbound
	if includeIPRanges, ok, err := proxyValues.GetAndRemoveString("includeIPRanges"); ok {
		trafficControl.Outbound.IncludedIPRanges = strings.Split(includeIPRanges, ",")
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if excludeIPRanges, ok, err := proxyValues.GetAndRemoveString("excludeIPRanges"); ok {
		var ipRangeSlice []string
		if excludeIPRanges != "" {
			ipRangeSlice = strings.Split(excludeIPRanges, ",")
		}
		trafficControl.Outbound.ExcludedIPRanges = ipRangeSlice
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if excludeOutboundPorts, ok, err := proxyValues.GetAndRemoveString("excludeOutboundPorts"); ok {
		if trafficControl.Outbound.ExcludedPorts, err = stringToInt32Slice(excludeOutboundPorts); err != nil {
			return err
		}
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if outboundTrafficPolicy, ok, err := in.GetAndRemoveString("global.outboundTrafficPolicy.mode"); ok && outboundTrafficPolicy != "" {
		trafficControl.Outbound.Policy = v2.ProxyOutboundTrafficPolicy(outboundTrafficPolicy)
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if outboundTrafficPolicy, ok, err := meshConfigValues.GetAndRemoveString("outboundTrafficPolicy.mode"); ok && outboundTrafficPolicy != "" {
		trafficControl.Outbound.Policy = v2.ProxyOutboundTrafficPolicy(outboundTrafficPolicy)
		setTrafficControl = true
	} else if err != nil {
		return err
	}

	if setTrafficControl {
		networking.TrafficControl = trafficControl
		setNetworking = true
	}

	// Protocol
	autoDetect := &v2.ProxyNetworkAutoProtocolDetectionConfig{}
	setProtocol := false
	if protocolDetectionTimeout, ok, err := proxyValues.GetAndRemoveString("protocolDetectionTimeout"); ok && protocolDetectionTimeout != "" {
		autoDetect.Timeout = protocolDetectionTimeout
		setProtocol = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if protocolDetectionTimeout, ok, err := meshConfigValues.GetAndRemoveString("protocolDetectionTimeout"); ok && protocolDetectionTimeout != "" {
		autoDetect.Timeout = protocolDetectionTimeout
		setProtocol = true
	} else if err != nil {
		return err
	}
	if enableProtocolSniffingForInbound, ok, err := in.GetAndRemoveBool("pilot.enableProtocolSniffingForInbound"); ok {
		autoDetect.Inbound = &enableProtocolSniffingForInbound
		setProtocol = true
	} else if err != nil {
		return err
	}
	if enableProtocolSniffingForOutbound, ok, err := in.GetAndRemoveBool("pilot.enableProtocolSniffingForOutbound"); ok {
		autoDetect.Outbound = &enableProtocolSniffingForOutbound
		setProtocol = true
	} else if err != nil {
		return err
	}
	if setProtocol {
		networking.Protocol = &v2.ProxyNetworkProtocolConfig{
			AutoDetect: autoDetect,
		}
		setNetworking = true
	}

	// DNS
	dns := &v2.ProxyDNSConfig{}
	setDNS := false
	if podDNSSearchNamespaces, ok, err := in.GetAndRemoveStringSlice("global.podDNSSearchNamespaces"); ok {
		if addedSearchSuffixes, ok, err := in.GetAndRemoveStringSlice("global.multiCluster.addedSearchSuffixes"); ok && len(addedSearchSuffixes) > 0 {
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
			dns.SearchSuffixes = podDNSSearchNamespaces
			setDNS = true
		}
	} else if err != nil {
		return err
	}
	if dnsRefreshRate, ok, err := proxyValues.GetAndRemoveString("dnsRefreshRate"); ok && dnsRefreshRate != "" {
		dns.RefreshRate = dnsRefreshRate
		setDNS = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if dnsRefreshRate, ok, err := meshConfigValues.GetAndRemoveString("dnsRefreshRate"); ok && dnsRefreshRate != "" {
		dns.RefreshRate = dnsRefreshRate
		setDNS = true
	} else if err != nil {
		return err
	}
	if setDNS {
		networking.DNS = dns
		setNetworking = true
	}
	if setNetworking {
		proxy.Networking = networking
		setProxy = true
	}

	// Injection
	injection := &v2.ProxyInjectionConfig{}
	setInjection := false
	if autoInject, ok, err := proxyValues.GetAndRemoveString("autoInject"); ok {
		if autoInject == "enabled" {
			enabled := true
			injection.AutoInject = &enabled
			setInjection = true
		} else {
			disabled := false
			injection.AutoInject = &disabled
			setInjection = true
		}
		// clear other setting
		in.RemoveField("sidecarInjectorWebhook.enableNamespacesByDefault")
	} else if err != nil {
		return err
	} else if enableNamespacesByDefault, ok, err := in.GetAndRemoveBool("sidecarInjectorWebhook.enableNamespacesByDefault"); ok {
		injection.AutoInject = &enableNamespacesByDefault
		setInjection = true
	} else if err != nil {
		return err
	}
	if alwaysInjectSelector, ok, err := in.GetFieldNoCopy("sidecarInjectorWebhook.alwaysInjectSelector"); ok {
		if err := fromValues(alwaysInjectSelector, &injection.AlwaysInjectSelector); err != nil {
			return err
		}
		setInjection = true
		in.RemoveField("sidecarInjectorWebhook.alwaysInjectSelector")
	} else if err != nil {
		return err
	}
	if neverInjectSelector, ok, err := in.GetFieldNoCopy("sidecarInjectorWebhook.neverInjectSelector"); ok {
		if err := fromValues(neverInjectSelector, &injection.NeverInjectSelector); err != nil {
			return err
		}
		setInjection = true
		in.RemoveField("sidecarInjectorWebhook.neverInjectSelector")
	} else if err != nil {
		return err
	}
	if injectedAnnotations, ok, err := in.GetMap("sidecarInjectorWebhook.injectedAnnotations"); ok {
		if err := decodeAndRemoveFromValues(injectedAnnotations, &injection.InjectedAnnotations); err != nil {
			return err
		}
		setInjection = true
		if len(injectedAnnotations) == 0 {
			in.RemoveField("sidecarInjectorWebhook.injectedAnnotations")
		} else if err := in.SetField("sidecarInjectorWebhook.injectedAnnotations", injectedAnnotations); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if setInjection {
		proxy.Injection = injection
		setProxy = true
	}

	// Access logging
	accessLogging := &v2.ProxyAccessLoggingConfig{}
	setAccessLogging := false
	fileAccessLog := &v2.ProxyFileAccessLogConfig{}
	setFileAccessLog := false
	if accessLogFile, ok, err := proxyValues.GetAndRemoveString("accessLogFile"); ok && accessLogFile != "" {
		fileAccessLog.Name = accessLogFile
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if accessLogFile, ok, err := meshConfigValues.GetAndRemoveString("accessLogFile"); ok && accessLogFile != "" {
		fileAccessLog.Name = accessLogFile
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	if accessLogEncoding, ok, err := proxyValues.GetAndRemoveString("accessLogEncoding"); ok && accessLogEncoding != "" {
		fileAccessLog.Encoding = accessLogEncoding
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if accessLogEncoding, ok, err := meshConfigValues.GetAndRemoveString("accessLogEncoding"); ok && accessLogEncoding != "" {
		fileAccessLog.Encoding = accessLogEncoding
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	if accessLogFormat, ok, err := proxyValues.GetAndRemoveString("accessLogFormat"); ok && accessLogFormat != "" {
		fileAccessLog.Format = accessLogFormat
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if accessLogFormat, ok, err := meshConfigValues.GetAndRemoveString("accessLogFormat"); ok && accessLogFormat != "" {
		fileAccessLog.Format = accessLogFormat
		setFileAccessLog = true
	} else if err != nil {
		return err
	}
	if setFileAccessLog {
		accessLogging.File = fileAccessLog
		setAccessLogging = true
	}

	if accessLogServiceValues, ok, err := proxyValues.GetMap("envoyAccessLogService"); ok && len(accessLogServiceValues) > 0 {
		accessLogService := &v2.ProxyEnvoyServiceConfig{}
		if updated, err := populateProxyEnvoyServiceConfig(v1.NewHelmValues(accessLogServiceValues), accessLogService); updated {
			accessLogging.EnvoyService = accessLogService
			setAccessLogging = true
		} else if err != nil {
			return err
		}
		if len(accessLogServiceValues) == 0 {
			proxyValues.RemoveField("envoyAccessLogService")
		} else if err := proxyValues.SetField("envoyAccessLogService", accessLogServiceValues); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if accessLogServiceValues, ok, err := meshConfigValues.GetMap("defaultConfig.envoyAccessLogService"); ok && len(accessLogServiceValues) > 0 {
		accessLogService := &v2.ProxyEnvoyServiceConfig{}
		if updated, err := populateProxyEnvoyServiceConfig(v1.NewHelmValues(accessLogServiceValues), accessLogService); updated {
			accessLogging.EnvoyService = accessLogService
			setAccessLogging = true
		} else if err != nil {
			return err
		}
		if enableEnvoyAccessLogService, ok, err := meshConfigValues.GetAndRemoveBool("enableEnvoyAccessLogService"); ok {
			accessLogService.Enabled = &enableEnvoyAccessLogService
			setAccessLogging = true
		} else if err != nil {
			return err
		}
		if len(accessLogServiceValues) == 0 {
			meshConfigValues.RemoveField("defaultConfig.envoyAccessLogService")
		} else if err := meshConfigValues.SetField("defaultConfig.envoyAccessLogService", accessLogServiceValues); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if setAccessLogging {
		proxy.AccessLogging = accessLogging
		setProxy = true
	}

	// Metrics service
	if metricsServiceValues, ok, err := proxyValues.GetMap("envoyMetricsService"); ok && len(metricsServiceValues) > 0 {
		metricsService := &v2.ProxyEnvoyServiceConfig{}
		if updated, err := populateProxyEnvoyServiceConfig(v1.NewHelmValues(metricsServiceValues), metricsService); updated {
			proxy.EnvoyMetricsService = metricsService
			setProxy = true
		} else if err != nil {
			return err
		}
		if len(metricsServiceValues) == 0 {
			proxyValues.RemoveField("envoyMetricsService")
		} else if err := proxyValues.SetField("envoyMetricsService", metricsServiceValues); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// meshConfig takes precedence
	if metricsServiceValues, ok, err := meshConfigValues.GetMap("defaultConfig.envoyMetricsService"); ok && len(metricsServiceValues) > 0 {
		metricsService := &v2.ProxyEnvoyServiceConfig{}
		if updated, err := populateProxyEnvoyServiceConfig(v1.NewHelmValues(metricsServiceValues), metricsService); updated {
			proxy.EnvoyMetricsService = metricsService
			setProxy = true
		} else if err != nil {
			return err
		}
		if len(metricsServiceValues) == 0 {
			meshConfigValues.RemoveField("defaultConfig.envoyMetricsService")
		} else if err := meshConfigValues.SetField("envoyMetricsService", metricsServiceValues); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Runtime
	runtime := &v2.ProxyRuntimeConfig{}
	setRuntime := false
	container := &v2.ContainerConfig{}
	setContainer := false
	if applied, err := populateContainerConfig(proxyValues, container); err != nil {
		return err
	} else if applied {
		setContainer = true
	}

	if envMap, ok, err := meshConfigValues.GetAndRemoveStringToStringMap("defaultConfig.proxyMetadata"); ok {
		container.Env = envMap
		setContainer = true
	} else if err != nil {
		return err
	}

	if setContainer {
		runtime.Container = container
		setRuntime = true
	}

	// Readiness
	readiness := &v2.ProxyReadinessConfig{}
	setReadiness := false
	if statusPort, ok, err := proxyValues.GetAndRemoveInt64("statusPort"); ok && statusPort > 0 {
		readiness.StatusPort = int32(statusPort)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessInitialDelaySeconds, ok, err := proxyValues.GetAndRemoveInt64("readinessInitialDelaySeconds"); ok && readinessInitialDelaySeconds > 0 {
		readiness.InitialDelaySeconds = int32(readinessInitialDelaySeconds)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessPeriodSeconds, ok, err := proxyValues.GetAndRemoveInt64("readinessPeriodSeconds"); ok && readinessPeriodSeconds > 0 {
		readiness.PeriodSeconds = int32(readinessPeriodSeconds)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessFailureThreshold, ok, err := proxyValues.GetAndRemoveInt64("readinessFailureThreshold"); ok && readinessFailureThreshold > 0 {
		readiness.FailureThreshold = int32(readinessFailureThreshold)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if rewriteAppHTTPProbe, ok, err := in.GetAndRemoveBool("sidecarInjectorWebhook.rewriteAppHTTPProbe"); ok {
		readiness.RewriteApplicationProbes = rewriteAppHTTPProbe
		setReadiness = true
	} else if err != nil {
		return err
	}
	if setReadiness {
		runtime.Readiness = readiness
		setRuntime = true
	}
	if setRuntime {
		proxy.Runtime = runtime
		setProxy = true
	}

	if setProxy {
		out.Proxy = proxy
	}
	if len(proxyValues.GetContent()) == 0 {
		in.RemoveField("global.proxy")
	} else if err := in.SetField("global.proxy", proxyValues.GetContent()); err != nil {
		return err
	}
	if defaultConfig, ok, _ := meshConfigValues.GetMap("defaultConfig"); ok && len(defaultConfig) == 0 {
		meshConfigValues.RemoveField("defaultConfig")
	}
	if len(meshConfigValues.GetContent()) == 0 {
		in.RemoveField("meshConfig")
	} else if err := in.SetField("meshConfig", meshConfigValues.GetContent()); err != nil {
		return err
	}

	return nil
}

func populateProxyEnvoyServiceConfig(in *v1.HelmValues, out *v2.ProxyEnvoyServiceConfig) (bool, error) {
	updated := false
	if enabled, ok, err := in.GetAndRemoveBool("enabled"); ok {
		out.Enabled = &enabled
		updated = true
	} else if err != nil {
		return false, err
	}
	if address, ok, err := in.GetAndRemoveString("host"); ok {
		if port, ok, err := in.GetAndRemoveString("port"); ok {
			if port != "" {
				address = fmt.Sprintf("%s:%s", address, port)
			}
		} else if err != nil {
			return false, err
		}
		out.Address = address
		updated = true
	} else if err != nil {
		return false, err
	}
	if tcpKeepaliveValues, ok, err := in.GetMap("tcpKeepalive"); ok && len(tcpKeepaliveValues) > 0 {
		out.TCPKeepalive = &v2.EnvoyServiceTCPKeepalive{}
		updated = true
		if err := decodeAndRemoveFromValues(tcpKeepaliveValues, out.TCPKeepalive); err != nil {
			return false, err
		}
		if len(tcpKeepaliveValues) == 0 {
			in.RemoveField("tcpKeepalive")
		} else if err := in.SetField("tcpKeepalive", tcpKeepaliveValues); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	if tlsSettingsValues, ok, err := in.GetMap("tlsSettings"); ok && len(tlsSettingsValues) > 0 {
		out.TLSSettings = &v2.EnvoyServiceClientTLSSettings{}
		updated = true
		if err := decodeAndRemoveFromValues(tlsSettingsValues, out.TLSSettings); err != nil {
			return false, err
		}
		if len(tlsSettingsValues) == 0 {
			in.RemoveField("tlsSettings")
		} else if err := in.SetField("tlsSettings", tlsSettingsValues); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	return updated, nil
}
