/*
Copyright 2024 The KCP Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FrontProxySpec defines the desired state of FrontProxy.
type FrontProxySpec struct {
	// RootShard configures the kcp root shard that this front-proxy instance should connect to.
	RootShard RootShardConfig `json:"rootShard"`
	// Optional: Replicas configures the replica count for the front-proxy Deployment.
	Replicas *int32 `json:"replicas,omitempty"`
	// Optional: Auth configures various aspects of Authentication and Authorization for this front-proxy instance.
	Auth *AuthSpec `json:"auth,omitempty"`
	// Optional: AdditionalPathMappings configures // TODO ?
	AdditionalPathMappings []PathMappingEntry `json:"additionalPathMappings,omitempty"`
	// Optional: Image defines the image to use. Defaults to the latest versioned image during the release of kcp-operator
	Image *ImageSpec `json:"image,omitempty"`
	// ExternalHostname under which the FrontProxy can be reached
	ExternalHostname string `json:"externalHostname"`
}

type AuthSpec struct {
	// Optional: OIDC configures OpenID Connect Authentication
	OIDC *OIDCConfiguration `json:"oidc,omitempty"`
}

// FrontProxyStatus defines the observed state of FrontProxy
type FrontProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FrontProxy is the Schema for the frontproxies API
type FrontProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FrontProxySpec   `json:"spec,omitempty"`
	Status FrontProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FrontProxyList contains a list of FrontProxy
type FrontProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FrontProxy `json:"items"`
}

// TODO for now the PathMappingEntry is defined inline at kcp upstream (https://github.com/kcp-dev/kcp/blob/f81a97d0fba951e6ac6f94e8e0f5339f49a9dd92/cmd/sharded-test-server/frontproxy.go#L69),
// so we have to copy the struct type
type PathMappingEntry struct {
	Path            string `json:"path"`
	Backend         string `json:"backend"`
	BackendServerCA string `json:"backend_server_ca"`
	ProxyClientCert string `json:"proxy_client_cert"`
	ProxyClientKey  string `json:"proxy_client_key"`
}

func init() {
	SchemeBuilder.Register(&FrontProxy{}, &FrontProxyList{})
}
