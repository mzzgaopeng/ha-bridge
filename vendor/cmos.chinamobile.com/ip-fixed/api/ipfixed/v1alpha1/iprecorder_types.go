/*


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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type IPRecorderIPLists struct {
	// INSERT ADDITIONAL IPLists FIELDS - ip record list
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the IPPool.
	Pool string `json:"pool,omitempty"`
	// Recorded IP Address.
	IPAddress string `json:"ipAddress,omitempty"`
	// The spec.gateway of the IPPool.
	Gateway string `json:"gateway,omitempty"`
	// Index of the recorded IPAddress. For example, 3 means 192.168.2.4.
	Index int `json:"index,omitempty"`
	// Record the resource information of the current IPAddress.
	Resources string `json:"resources,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	// The spec.vlan of the IPPool. The valid range is 1-4094.
	Vlan int `json:"vlan,omitempty"`
	// Indicates whether the pod of the current ip has released the ip, but this value cannot be used as the basis for controller recovery.
	Released bool `json:"released,omitempty"`
}

// IPRecorderSpec defines the desired state of IPRecorder
type IPRecorderSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of IPRecorder. Edit IPRecorder_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// IPRecorderStatus defines the observed state of IPRecorder
type IPRecorderStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// IPRecorder is the Schema for the iprecorders API
type IPRecorder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//Spec   IPRecorderSpec   `json:"spec,omitempty"`
	IPLists []IPRecorderIPLists `json:"IPLists,omitempty"`
	Status  IPRecorderStatus    `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPRecorderList contains a list of IPRecorder
type IPRecorderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPRecorder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPRecorder{}, &IPRecorderList{})
}
