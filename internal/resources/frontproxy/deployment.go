/*
Copyright 2025 The KCP Authors.

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

package frontproxy

import (
	"fmt"

	"k8c.io/reconciler/pkg/reconciling"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/kcp-operator/internal/resources"
	operatorv1alpha1 "github.com/kcp-dev/kcp-operator/sdk/apis/operator/v1alpha1"
)

const (
	ContainerName = "kcp-front-proxy"
)

func DeploymentReconciler(frontproxy *operatorv1alpha1.FrontProxy, rootshard *operatorv1alpha1.RootShard) reconciling.NamedDeploymentReconcilerFactory {
	image, _ := resources.GetImageSettings(frontproxy.Spec.Image)
	args := getArgs(frontproxy)
	name := fmt.Sprintf("%s-fp-kcp", frontproxy.Name)

	return func() (string, reconciling.DeploymentReconciler) {
		return name, func(dep *appsv1.Deployment) (*appsv1.Deployment, error) {
			dep.SetLabels(resources.GetFrontProxyResourceLabels(frontproxy))
			dep.Spec.Selector = &v1.LabelSelector{
				MatchLabels: resources.GetFrontProxyResourceLabels(frontproxy),
			}
			dep.Spec.Template.ObjectMeta.SetLabels(resources.GetFrontProxyResourceLabels(frontproxy))

			container := corev1.Container{
				Name:    ContainerName,
				Image:   image,
				Command: []string{"/kcp-front-proxy"},
				Args:    args,
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			}

			volumes := []corev1.Volume{}
			volumeMounts := []corev1.VolumeMount{}

			// front-proxy dynamic kubeconfig
			volumes = append(volumes, corev1.Volume{
				Name: resources.GetFrontProxyDynamicKubeconfigName(rootshard, frontproxy),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: resources.GetFrontProxyDynamicKubeconfigName(rootshard, frontproxy),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      resources.GetFrontProxyDynamicKubeconfigName(rootshard, frontproxy),
				ReadOnly:  false, // as FrontProxy writes to it to work with different shards
				MountPath: FrontProxyBasepath + "/kubeconfig",
			})

			// front-proxy kubeconfig client cert
			kubeconfigClientCertName := resources.GetFrontproxyCertificateName(rootshard, frontproxy, operatorv1alpha1.KubeconfigCertificate)
			volumes = append(volumes, corev1.Volume{
				Name: kubeconfigClientCertName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: kubeconfigClientCertName,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      kubeconfigClientCertName,
				ReadOnly:  true,
				MountPath: FrontProxyBasepath + "/kubeconfig-client-cert",
			})

			// front-proxy service-account cert
			volumes = append(volumes, corev1.Volume{
				Name: resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServiceAccountCertificate),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServiceAccountCertificate),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServiceAccountCertificate),
				ReadOnly:  true,
				MountPath: fmt.Sprintf("/etc/kcp/tls/%s", string(operatorv1alpha1.ServiceAccountCertificate)),
			})

			// front-proxy server cert
			volumes = append(volumes, corev1.Volume{
				Name: resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServerCertificate),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServerCertificate),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      resources.GetRootShardCertificateName(rootshard, operatorv1alpha1.ServerCertificate),
				ReadOnly:  true,
				MountPath: FrontProxyBasepath + "/tls",
			})

			// front-proxy requestheader client cert
			requestHeaderClientCertName := resources.GetFrontproxyCertificateName(rootshard, frontproxy, operatorv1alpha1.RequestHeaderClientCertificate)
			volumes = append(volumes, corev1.Volume{
				Name: requestHeaderClientCertName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: requestHeaderClientCertName,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      requestHeaderClientCertName,
				ReadOnly:  true,
				MountPath: FrontProxyBasepath + "/requestheader-client",
			})

			// front-proxy config
			cmName := resources.GetFrontProxyConfigName(frontproxy)
			volumes = append(volumes, corev1.Volume{
				Name: cmName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cmName,
						},
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      cmName,
				ReadOnly:  true,
				MountPath: FrontProxyBasepath + "/config",
			})

			// rootshard client ca
			rsClientCAName := resources.GetRootShardCAName(rootshard, operatorv1alpha1.ClientCA)
			volumes = append(volumes, corev1.Volume{
				Name: rsClientCAName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: rsClientCAName,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      rsClientCAName,
				ReadOnly:  true,
				MountPath: FrontProxyBasepath + "/client-ca",
			})

			// kcp rootshard root ca
			rootCAName := resources.GetRootShardCAName(rootshard, operatorv1alpha1.RootCA)
			volumes = append(volumes, corev1.Volume{
				Name: rootCAName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: rootCAName,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      rootCAName,
				ReadOnly:  true,
				MountPath: KcpBasepath + "/tls/ca",
			})

			if frontproxy.Spec.Replicas != nil {
				dep.Spec.Replicas = frontproxy.Spec.Replicas
			}

			dep.Spec.Template.Spec.Volumes = volumes
			container.VolumeMounts = volumeMounts
			dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, container)

			return dep, nil
		}
	}
}

func getArgs(frontproxy *operatorv1alpha1.FrontProxy) []string {

	args := []string{
		"--secure-port=8443",
		"--root-kubeconfig=/etc/kcp-front-proxy/kubeconfig/kubeconfig",
		"--shards-kubeconfig=/etc/kcp-front-proxy/kubeconfig/kubeconfig",
		"--tls-private-key-file=/etc/kcp-front-proxy/tls/tls.key",
		"--tls-cert-file=/etc/kcp-front-proxy/tls/tls.crt",
		"--client-ca-file=/etc/kcp-front-proxy/client-ca/tls.crt",
		"--mapping-file=/etc/kcp-front-proxy/config/path-mapping.yaml",
		"--service-account-key-file=/etc/kcp/tls/service-account/tls.key",
	}

	return args
}
