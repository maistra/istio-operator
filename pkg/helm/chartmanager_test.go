// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/test"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var ctx = context.TODO()

func TestUpgradeOrInstallChart(t *testing.T) {
	_, cl, cfg := test.SetupEnv(os.Stdout, false)

	relName := "my-release"
	chartDir := filepath.Join("testdata", "chart")

	owner := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Istio",
		Name:       "my-istio",
		UID:        "1234",
		Controller: ptr.Of(true),
	}

	tests := []struct {
		name    string
		setup   func(*WithT, client.Client, *ChartManager, string)
		wantErr bool
	}{
		{
			name: "release does not exist",
		},
		{
			name: "release exists",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
			},
		},
		{
			name: "release in failed state with previous revision",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				upgrade(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusFailed)
			},
		},
		{
			name: "release in failed state with no previous revision",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusFailed)
			},
		},
		{
			name: "release in pending-install state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusPendingInstall)
			},
		},
		{
			name: "release in pending-upgrade state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				upgrade(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusPendingUpgrade)
			},
		},
		{
			name: "release in uninstalled state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusUninstalled)
			},
			wantErr: true,
		},
		{
			name: "release in uninstalling state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusUninstalling)
			},
			wantErr: true,
		},
		{
			name: "release in unknown state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusUnknown)
			},
			wantErr: true,
		},
		{
			name: "release in superseded state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusSuperseded)
			},
			wantErr: true,
		},
		{
			name: "release in pending-rollback state",
			setup: func(g *WithT, cl client.Client, helm *ChartManager, ns string) {
				install(g, helm, chartDir, ns, relName, owner)
				setReleaseStatus(g, helm, ns, relName, release.StatusPendingRollback)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			helm := NewChartManager(cfg, "")
			ns := "test-" + rand.String(8)

			g.Expect(createNamespace(cl, ns)).To(Succeed())

			if tt.setup != nil {
				tt.setup(g, cl, helm, ns)
			}

			rel, err := helm.UpgradeOrInstallChart(ctx, chartDir, Values{"value": "my-value"}, ns, relName, owner)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(rel).ToNot(BeNil())
				g.Expect(rel.Name).To(Equal(relName))

				configMap := &corev1.ConfigMap{}
				g.Expect(cl.Get(ctx, types.NamespacedName{Name: "test", Namespace: ns}, configMap)).To(Succeed())
				g.Expect(configMap.Data).To(HaveKeyWithValue("value", "my-value"))
				g.Expect(configMap.OwnerReferences).To(ContainElement(owner))
			}
		})
	}
}

func createNamespace(cl client.Client, ns string) error {
	return cl.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	})
}

func install(g *WithT, helm *ChartManager, chartDir string, ns string, relName string, owner metav1.OwnerReference) {
	upgradeOrInstall(g, helm, chartDir, ns, relName, owner)
}

func upgrade(g *WithT, helm *ChartManager, chartDir string, ns string, relName string, owner metav1.OwnerReference) {
	upgradeOrInstall(g, helm, chartDir, ns, relName, owner)
}

func upgradeOrInstall(g *WithT, helm *ChartManager, chartDir string, ns string, relName string, owner metav1.OwnerReference) {
	_, err := helm.UpgradeOrInstallChart(ctx, chartDir, Values{"value": "other-value"}, ns, relName, owner)
	g.Expect(err).ToNot(HaveOccurred())
}

func setReleaseStatus(g *WithT, helm *ChartManager, ns, releaseName string, status release.Status) {
	cfg, err := helm.newActionConfig(ctx, ns)
	g.Expect(err).ToNot(HaveOccurred())

	rel, err := getRelease(cfg, releaseName)
	g.Expect(err).ToNot(HaveOccurred())

	rel.SetStatus(status, "simulated status")
	g.Expect(cfg.Releases.Update(rel)).To(Succeed())
}
