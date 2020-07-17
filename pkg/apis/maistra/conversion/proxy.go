package conversion

import (
	"strconv"
	"strings"

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
