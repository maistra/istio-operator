package installation

import (
	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *Handler) getJob(name, namespace string) *v1.Job {
	return &v1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (h *Handler) newJobItems(cr *v1alpha1.Installation, jobName, configMapName, namespace, configMapContent string) []runtime.Object {
	var (
		backoffLimit           int32 = 6
		completions            int32 = 1
		parallelism            int32 = 1
		runAsUser              int64 = 0
		terminationGracePeriod int64 = 30
	)

	return []runtime.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"istio.inventory": configMapContent,
			},
		},
		&v1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
				Labels: map[string]string{
					"job-name": jobName,
				},
			},
			Spec: v1.JobSpec{
				BackoffLimit: &backoffLimit,
				Completions:  &completions,
				Parallelism:  &parallelism,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"job-name": jobName,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Env: []corev1.EnvVar{{
								Name:  "INVENTORY_FILE",
								Value: inventoryFile,
							}, {
								Name:  "PLAYBOOK_FILE",
								Value: playbookFile,
							}, {
								Name:  "OPTS",
								Value: playbookOptions,
							}},
							Image:           h.cleanPrefix(h.getIstioImagePrefix(cr)) + h.getDeploymentType(cr) + "-ansible:" + h.getIstioImageVersion(cr),
							ImagePullPolicy: h.getAlwaysPull(),
							Name:            installerJobName,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "configdir",
								MountPath: configurationDir,
							}, {
								Name:      "configmap",
								MountPath: inventoryDir,
							}},
						}},
						DNSPolicy:                     corev1.DNSClusterFirst,
						HostNetwork:                   true,
						RestartPolicy:                 corev1.RestartPolicyNever,
						SchedulerName:                 "default-scheduler",
						ServiceAccountName:            serviceAccountName,
						TerminationGracePeriodSeconds: &terminationGracePeriod,
						Volumes: []corev1.Volume{{
							Name: "configdir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: configurationDir,
								},
							},
						}, {
							Name: "configmap",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						}},
					},
				},
			},
		},
	}
}
