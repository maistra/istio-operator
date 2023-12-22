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

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maistra.io/istio-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Istio resource", Ordered, func() {
	const (
		istioName         = "test-istio"
		istioNamespace    = "istio-test"
		workloadNamespace = "istio-test-workloads"

		istioVersion = "latest"
		oldVersion   = "v1.20.0"
		newVersion   = "v1.20.1" // TODO: get these versions from versions.yaml

		gracePeriod = 30 * time.Second
		pilotImage  = "maistra.io/test:latest"
	)
	istioKey := client.ObjectKey{Name: istioName}
	istio := &v1alpha1.Istio{}

	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	SetDefaultConsistentlyDuration(10 * time.Second)
	SetDefaultConsistentlyPollingInterval(time.Second)

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	BeforeAll(func() {
		Step("Creating the Namespace to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations.
		// More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		deleteAllIstiosAndRevisions(ctx)
	})

	Describe("basic operation", func() {
		BeforeAll(func() {
			Step("Creating the custom resource")
			istio = &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
				},
				Spec: v1alpha1.IstioSpec{
					Version:   istioVersion,
					Namespace: istioNamespace,
					UpdateStrategy: &v1alpha1.IstioUpdateStrategy{
						Type: v1alpha1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
					Values: []byte(`{
						"pilot":{"image":"` + pilotImage + `"},
						"istio_cni":{"enabled":true}
					}`),
				},
			}
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())
		})

		It("updates the Istio resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
				g.Expect(istio.Status.ObservedGeneration).To(Equal(istio.Generation))
			}).Should(Succeed())
		})

		It("creates the IstioRevision resource", func() {
			revKey := client.ObjectKey{Name: istioName}
			rev := &v1alpha1.IstioRevision{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
				g.Expect(rev.GetOwnerReferences()).To(ContainElement(NewOwnerReference(istio)))
			}).Should(Succeed())

			Expect(rev.Spec).To(Equal(v1alpha1.IstioRevisionSpec{
				Version:   istio.Spec.Version,
				Namespace: istio.Spec.Namespace,
				Values: deprettify([]byte(`{
					"defaultRevision":"",
					"gateways":{
						"istio-egressgateway":{},
						"istio-ingressgateway":{}
					},
					"global":{
						"configValidation":true,
						"istioNamespace":"` + istio.Spec.Namespace + `"
					},
					"istio_cni": {
						"enabled":true
					},
					"pilot":{"image":"` + pilotImage + `"},
					"revision":"` + revKey.Name + `"
				}`)),
			}))
		})

		When("the underlying IstioRevision is deleted", func() {
			rev := &v1alpha1.IstioRevision{}
			revKey := client.ObjectKey{Name: istioName}

			BeforeAll(func() {
				rev = &v1alpha1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: istio.Name,
					},
				}
				Expect(k8sClient.Delete(ctx, rev)).To(Succeed())
			})

			It("recreates the IstioRevision", func() {
				Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
				Expect(rev.GetOwnerReferences()).To(ContainElement(NewOwnerReference(istio)))
				Expect(rev.Spec).To(Equal(v1alpha1.IstioRevisionSpec{
					Version:   istio.Spec.Version,
					Namespace: istio.Spec.Namespace,
					Values: deprettify([]byte(`{
						"defaultRevision":"",
						"gateways":{
							"istio-egressgateway":{},
							"istio-ingressgateway":{}
						},
						"global":{
							"configValidation":true,
							"istioNamespace":"` + istio.Spec.Namespace + `"
						},
						"istio_cni": {
							"enabled":true
						},
						"pilot":{"image":"` + pilotImage + `"},
						"revision": "` + revKey.Name + `"
					}`)),
				}))
			})
		})

		When("the Istio resource is deleted", func() {
			BeforeAll(func() {
				Expect(k8sClient.Delete(ctx, istio)).To(Succeed())
				Eventually(k8sClient.Get).WithContext(ctx).WithArguments(istioKey, istio).Should(ReturnNotFoundError())
			})

			It("deletes the IstioRevision", func() {
				revKey := client.ObjectKey{Name: istioName}
				rev := &v1alpha1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: istio.Name,
					},
				}
				Eventually(k8sClient.Get).WithContext(ctx).WithArguments(revKey, rev).Should(ReturnNotFoundError())
			})
		})
	})

	Describe("update", func() {
		var workloadNs *corev1.Namespace
		rev := &v1alpha1.IstioRevision{}

		for _, withWorkloads := range []bool{true, false} {
			withWorkloads := withWorkloads

			Context(generateContextName(withWorkloads), func() {
				if withWorkloads {
					BeforeAll(func() {
						workloadNs = &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: workloadNamespace,
								Labels: map[string]string{
									"istio.io/rev": istioName,
								},
							},
						}
						Expect(k8sClient.Create(ctx, workloadNs)).To(Succeed())
					})
					AfterAll(func() {
						// since ns deletion doesn't work, we instead remove the label so the namespace no longer references the revision
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workloadNs), workloadNs)).To(Succeed())
						delete(workloadNs.Labels, "istio.io/rev")
						Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
					})
				}

				Context("with InPlace update strategy", func() {
					revKey := client.ObjectKey{Name: istioName}

					BeforeAll(func() {
						istio = &v1alpha1.Istio{
							ObjectMeta: metav1.ObjectMeta{
								Name: istioName,
							},
							Spec: v1alpha1.IstioSpec{
								Version:   oldVersion,
								Namespace: istioNamespace,
								UpdateStrategy: &v1alpha1.IstioUpdateStrategy{
									Type: v1alpha1.UpdateStrategyTypeInPlace,
									InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
								},
							},
						}
						Expect(k8sClient.Create(ctx, istio)).To(Succeed())

						Step("Check if IstioRevision exists")
						Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
					})

					AfterAll(func() {
						deleteAllIstiosAndRevisions(ctx)
					})

					When("version is updated", func() {
						BeforeAll(func() {
							Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
							istio.Spec.Version = newVersion
							Expect(k8sClient.Update(ctx, istio)).To(Succeed())
						})

						It("updates the IstioRevision", func() {
							Eventually(func(g Gomega) {
								g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
								g.Expect(rev.Spec.Version).To(Equal(newVersion))
							}).Should(Succeed())
						})
					})

					When("strategy is changed to RevisionBased", func() {
						BeforeAll(func() {
							By("changing strategy to RevisionBased")
							Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
							istio.Spec.UpdateStrategy.Type = v1alpha1.UpdateStrategyTypeRevisionBased
							Expect(k8sClient.Update(ctx, istio)).To(Succeed())
						})

						It("creates a new IstioRevision", func() {
							revKey := getRevisionKey(istio, istio.Spec.Version)
							Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
							Expect(rev.Spec.Version).To(Equal(istio.Spec.Version))
						})

						if withWorkloads {
							It("doesn't delete the previous IstioRevision while workloads reference it", func() {
								Consistently(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
							})

							When("workloads are moved to the new IstioRevision", func() {
								BeforeAll(func() {
									workloadNs.Labels["istio.io/rev"] = getRevisionName(istio, istio.Spec.Version)
									Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
								})

								It("doesn't immediately delete the previous IstioRevision", func() {
									marginOfError := 2 * time.Second
									Consistently(k8sClient.Get, gracePeriod-marginOfError).WithArguments(ctx, revKey, rev).Should(Succeed())
								})

								When("grace period expires", func() {
									It("deletes the previous IstioRevision", func() {
										Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
									})
								})
							})
						} else {
							When("grace period expires", func() {
								It("deletes the previous IstioRevision", func() {
									marginOfError := 30 * time.Second
									Eventually(k8sClient.Get, gracePeriod+marginOfError).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
								})
							})
						}
					})
				})

				Context("with RevisionBased update strategy", func() {
					BeforeAll(func() {
						istio = &v1alpha1.Istio{
							ObjectMeta: metav1.ObjectMeta{
								Name: istioName,
							},
							Spec: v1alpha1.IstioSpec{
								Version:   oldVersion,
								Namespace: istioNamespace,
								UpdateStrategy: &v1alpha1.IstioUpdateStrategy{
									Type: v1alpha1.UpdateStrategyTypeRevisionBased,
									InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
								},
							},
						}
						Expect(k8sClient.Create(ctx, istio)).To(Succeed())

						Step("Check if IstioRevision exists")
						revKey := getRevisionKey(istio, oldVersion)
						Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())

						if withWorkloads {
							workloadNs.Labels["istio.io/rev"] = getRevisionName(istio, oldVersion)
							Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
						}
					})

					AfterAll(func() {
						deleteAllIstiosAndRevisions(ctx)
					})

					When("version is updated", func() {
						BeforeAll(func() {
							Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
							istio.Spec.Version = newVersion
							Expect(k8sClient.Update(ctx, istio)).To(Succeed())
						})

						It("creates a new IstioRevision", func() {
							revKey := getRevisionKey(istio, newVersion)
							Eventually(func(g Gomega) {
								g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
								g.Expect(rev.Spec.Version).To(Equal(newVersion))
							}).Should(Succeed())
						})

						if withWorkloads {
							It("doesn't delete the previous IstioRevision while workloads reference it", func() {
								revKey := getRevisionKey(istio, oldVersion)
								Consistently(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
							})

							When("workloads are moved to the new IstioRevision", func() {
								BeforeAll(func() {
									workloadNs.Labels["istio.io/rev"] = getRevisionName(istio, newVersion)
									Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
								})

								It("doesn't immediately delete the previous IstioRevision", func() {
									marginOfError := 2 * time.Second
									revKey := getRevisionKey(istio, oldVersion)
									Consistently(k8sClient.Get, gracePeriod-marginOfError).WithArguments(ctx, revKey, rev).Should(Succeed())
								})

								When("grace period expires", func() {
									It("deletes the previous IstioRevision", func() {
										revKey := getRevisionKey(istio, oldVersion)
										Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
									})
								})
							})
						} else {
							When("grace period expires", func() {
								It("deletes the previous IstioRevision", func() {
									revKey := getRevisionKey(istio, oldVersion)
									Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
								})
							})
						}
					})

					When("strategy is changed to InPlace", func() {
						BeforeAll(func() {
							Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
							istio.Spec.UpdateStrategy.Type = v1alpha1.UpdateStrategyTypeInPlace
							Expect(k8sClient.Update(ctx, istio)).To(Succeed())
						})

						It("creates an IstioRevision with no version in the name", func() {
							revKey := client.ObjectKey{Name: istio.Name}
							Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
							Expect(rev.Spec.Version).To(Equal(istio.Spec.Version))
						})

						if withWorkloads {
							It("doesn't delete the previous IstioRevision while workloads reference it", func() {
								revKey := getRevisionKey(istio, newVersion)
								Consistently(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
							})

							When("workloads are moved to the IstioRevision with no version in the name", func() {
								BeforeAll(func() {
									workloadNs.Labels["istio.io/rev"] = istio.Name
									Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
								})

								It("doesn't immediately delete the previous IstioRevision", func() {
									marginOfError := 2 * time.Second
									revKey := getRevisionKey(istio, newVersion)
									Consistently(k8sClient.Get, gracePeriod-marginOfError).WithArguments(ctx, revKey, rev).Should(Succeed())
								})

								When("grace period expires", func() {
									It("deletes the previous IstioRevision", func() {
										revKey := getRevisionKey(istio, newVersion)
										Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
									})
								})
							})
						} else {
							When("grace period expires", func() {
								It("deletes the previous IstioRevision", func() {
									revKey := getRevisionKey(istio, newVersion)
									marginOfError := 30 * time.Second
									Eventually(k8sClient.Get, gracePeriod+marginOfError).WithArguments(ctx, revKey, rev).Should(ReturnNotFoundError())
								})
							})
						}
					})
				})
			})
		}
	})
})

func deleteAllIstiosAndRevisions(ctx context.Context) {
	Step("Deleting all Istio and IstioRevision resources")
	Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Istio{})).To(Succeed())
	Eventually(func(g Gomega) {
		list := &v1alpha1.IstioList{}
		g.Expect(k8sClient.List(ctx, list)).To(Succeed())
		g.Expect(list.Items).To(BeEmpty())
	}).Should(Succeed())

	Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IstioRevision{})).To(Succeed())
	Eventually(func(g Gomega) {
		list := &v1alpha1.IstioRevisionList{}
		g.Expect(k8sClient.List(ctx, list)).To(Succeed())
		g.Expect(list.Items).To(BeEmpty())
	}).Should(Succeed())
}

func generateContextName(withWorkloads bool) string {
	if withWorkloads {
		return "with workloads"
	}
	return "with no workloads"
}

func getRevisionKey(istio *v1alpha1.Istio, version string) client.ObjectKey {
	return client.ObjectKey{Name: getRevisionName(istio, version)}
}

func getRevisionName(istio *v1alpha1.Istio, version string) string {
	if istio.Name == "" {
		panic("istio.Name is empty")
	}
	return istio.Name + "-" + strings.ReplaceAll(version, ".", "-")
}

func deprettify(bytes []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(bytes, &obj); err != nil {
		panic(err)
	}
	bytes, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return bytes
}
