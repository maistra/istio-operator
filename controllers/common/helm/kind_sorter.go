/*
NOTE: this is an exact copy of https://github.com/helm/helm/blob/v2.16.7/pkg/tiller/kind_sorter.go
      Copying was necessary to avoid importing the whole tiller package, along
      with a bunch of dependencies that were incompatible with our requirements


Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"sort"

	"k8s.io/helm/pkg/manifest"
)

// SortOrder is an ordering of Kinds.
type SortOrder []string

// InstallOrder is the order in which manifests should be installed (by Kind).
//
// Those occurring earlier in the list get installed before those occurring later in the list.
var InstallOrder SortOrder = []string{
	"Namespace",
	"NetworkPolicy",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"PodDisruptionBudget",
	"Secret",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"ServiceAccount",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleList",
	"ClusterRoleBinding",
	"ClusterRoleBindingList",
	"Role",
	"RoleList",
	"RoleBinding",
	"RoleBindingList",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"HorizontalPodAutoscaler",
	"StatefulSet",
	"Job",
	"CronJob",
	"Ingress",
	"APIService",
}

// UninstallOrder is the order in which manifests should be uninstalled (by Kind).
//
// Those occurring earlier in the list get uninstalled before those occurring later in the list.
var UninstallOrder SortOrder = []string{
	"APIService",
	"Ingress",
	"Service",
	"CronJob",
	"Job",
	"StatefulSet",
	"HorizontalPodAutoscaler",
	"Deployment",
	"ReplicaSet",
	"ReplicationController",
	"Pod",
	"DaemonSet",
	"RoleBindingList",
	"RoleBinding",
	"RoleList",
	"Role",
	"ClusterRoleBindingList",
	"ClusterRoleBinding",
	"ClusterRoleList",
	"ClusterRole",
	"CustomResourceDefinition",
	"ServiceAccount",
	"PersistentVolumeClaim",
	"PersistentVolume",
	"StorageClass",
	"ConfigMap",
	"Secret",
	"PodDisruptionBudget",
	"PodSecurityPolicy",
	"LimitRange",
	"ResourceQuota",
	"NetworkPolicy",
	"Namespace",
}

type kindSorter struct {
	ordering  map[string]int
	manifests []manifest.Manifest
}

func newKindSorter(m []manifest.Manifest, s SortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		manifests: m,
		ordering:  o,
	}
}

func (k *kindSorter) Len() int { return len(k.manifests) }

func (k *kindSorter) Swap(i, j int) { k.manifests[i], k.manifests[j] = k.manifests[j], k.manifests[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.manifests[i]
	b := k.manifests[j]
	first, aok := k.ordering[a.Head.Kind]
	second, bok := k.ordering[b.Head.Kind]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind and name
		if a.Head.Kind != b.Head.Kind {
			return a.Head.Kind < b.Head.Kind
		}
		return a.Name < b.Name
	}

	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}

	// if same kind sub sort alphanumeric
	if first == second {
		return a.Name < b.Name
	}
	// sort different kinds
	return first < second
}

// SortByKind sorts manifests in InstallOrder
func SortByKind(manifests []manifest.Manifest) []manifest.Manifest {
	ordering := InstallOrder
	ks := newKindSorter(manifests, ordering)
	sort.Sort(ks)
	return ks.manifests
}
