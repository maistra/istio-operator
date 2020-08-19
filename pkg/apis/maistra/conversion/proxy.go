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
	if proxy.AutoInject != nil {
		if err := setHelmBoolValue(values, "sidecarInjectorWebhook.enableNamespacesByDefault", *proxy.AutoInject); err != nil {
			return err
		}
		if *proxy.AutoInject {
			if err := setHelmStringValue(proxyValues, "autoInject", "enabled"); err != nil {
				return err
			}
		} else {
			if err := setHelmStringValue(proxyValues, "autoInject", "disabled"); err != nil {
				return err
			}
		}
	}
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
		// XXX: proxy.Networking.ConnectionTimeout is not exposed through values
		if proxy.Networking.ConnectionTimeout != "" {
			if err := setHelmStringValue(proxyValues, "connectionTimeout", proxy.Networking.ConnectionTimeout); err != nil {
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
					if err := setHelmValue(values, "global.proxy_init", initValues); err != nil {
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
		}

		// Protocol
		if proxy.Networking.Protocol != nil {
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
			}
		}
	}

	// Runtime
	if proxy.Runtime != nil {
		if err := populateContainerConfigValues(proxy.Runtime.Container, proxyValues); err != nil {
			return err
		}
		// Readiness
		if proxy.Runtime.Readiness != nil {
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
	if autoInject, ok, err := proxyValues.GetString("autoInject"); ok {
		if autoInject == "enabled" {
			enabled := true
			proxy.AutoInject = &enabled
			setProxy = true
		} else {
			disabled := false
			proxy.AutoInject = &disabled
			setProxy = true
		}
	} else if err != nil {
		return err
	} else if enableNamespacesByDefault, ok, err := in.GetBool("sidecarInjectorWebhook.enableNamespacesByDefault"); ok {
		proxy.AutoInject = &enableNamespacesByDefault
		setProxy = true
	} else if err != nil {
		return err
	}
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
	if clusterDomain, ok, err := proxyValues.GetString("clusterDomain"); ok && clusterDomain != "" {
		networking.ClusterDomain = clusterDomain
		setNetworking = true
	} else if err != nil {
		return err
	}
	if connectionTimeout, ok, err := proxyValues.GetString("connectionTimeout"); ok {
		networking.ConnectionTimeout = connectionTimeout
		setNetworking = true
	} else if err != nil {
		return err
	}

	initialization := &v2.ProxyNetworkInitConfig{}
	setInitialization := false
	if initType, ok, err := proxyValues.GetString("initType"); ok {
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
	if interceptionMode, ok, err := proxyValues.GetString("interceptionMode"); ok && interceptionMode != "" {
		trafficControl.Inbound.InterceptionMode = v2.ProxyNetworkInterceptionMode(interceptionMode)
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if includeInboundPorts, ok, err := proxyValues.GetString("includeInboundPorts"); ok {
		trafficControl.Inbound.IncludedPorts = strings.Split(includeInboundPorts, ",")
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if excludeInboundPorts, ok, err := proxyValues.GetString("excludeInboundPorts"); ok {
		if trafficControl.Inbound.ExcludedPorts, err = stringToInt32Slice(excludeInboundPorts); err != nil {
			return err
		}
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	// Outbound
	if includeIPRanges, ok, err := proxyValues.GetString("includeIPRanges"); ok {
		trafficControl.Outbound.IncludedIPRanges = strings.Split(includeIPRanges, ",")
		setTrafficControl = true
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
		trafficControl.Outbound.ExcludedIPRanges = ipRangeSlice
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if excludeOutboundPorts, ok, err := proxyValues.GetString("excludeOutboundPorts"); ok {
		if trafficControl.Outbound.ExcludedPorts, err = stringToInt32Slice(excludeOutboundPorts); err != nil {
			return err
		}
		setTrafficControl = true
	} else if err != nil {
		return err
	}
	if outboundTrafficPolicy, ok, err := in.GetString("global.outboundTrafficPolicy.mode"); ok && outboundTrafficPolicy != "" {
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
	protocol := &v2.ProxyNetworkProtocolConfig{}
	setProtocol := true
	if protocolDetectionTimeout, ok, err := proxyValues.GetString("protocolDetectionTimeout"); ok && protocolDetectionTimeout != "" {
		protocol.DetectionTimeout = protocolDetectionTimeout
		setProtocol = true
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
		protocol.Debug = protocolDebugConfig
		setProtocol = true
	}
	if setProtocol {
		networking.Protocol = protocol
		setNetworking = true
	}

	// DNS
	dns := &v2.ProxyDNSConfig{}
	setDNS := false
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
			dns.SearchSuffixes = podDNSSearchNamespaces
			setDNS = true
		}
	} else if err != nil {
		return err
	}
	if dnsRefreshRate, ok, err := proxyValues.GetString("dnsRefreshRate"); ok && dnsRefreshRate != "" {
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

	// Runtime
	runtime := &v2.ProxyRuntimeConfig{}
	setRuntime := false
	container := &v2.ContainerConfig{}
	if applied, err := populateContainerConfig(proxyValues, container); err != nil {
		return err
	} else if applied {
		runtime.Container = container
		setRuntime = true
	}
	// Readiness
	readiness := &v2.ProxyReadinessConfig{}
	setReadiness := false
	if statusPort, ok, err := proxyValues.GetInt64("statusPort"); ok && statusPort > 0 {
		readiness.StatusPort = int32(statusPort)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessInitialDelaySeconds, ok, err := proxyValues.GetInt64("readinessInitialDelaySeconds"); ok && readinessInitialDelaySeconds > 0 {
		readiness.InitialDelaySeconds = int32(readinessInitialDelaySeconds)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessPeriodSeconds, ok, err := proxyValues.GetInt64("readinessPeriodSeconds"); ok && readinessPeriodSeconds > 0 {
		readiness.PeriodSeconds = int32(readinessPeriodSeconds)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if readinessFailureThreshold, ok, err := proxyValues.GetInt64("readinessFailureThreshold"); ok && readinessFailureThreshold > 0 {
		readiness.FailureThreshold = int32(readinessFailureThreshold)
		setReadiness = true
	} else if err != nil {
		return err
	}
	if rewriteAppHTTPProbe, ok, err := in.GetBool("sidecarInjectorWebhook.rewriteAppHTTPProbe"); ok {
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

	return nil
}
