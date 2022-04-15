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

// IPPoolDetailSpec defines the desired state of IPPoolDetail
type IPPoolDetailSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The spec.cidr of the IPPool.
	Cidr string `json:"cidr,omitempty"`
	// The spec.vlan of the IPPool. The valid range is 1-4094.
	Vlan int `json:"vlan,omitempty"`
	// The IP address index of the IPPool. null indicates that the ip corresponding to the index is not occupied. If it is not null, it means that the ip has been used, such as 0 means 192.168.2.1 has been used.
	// PS: After make manifests and before make install, you need to modify config/crd/bases/ipfixed.cmos.chinamobile.com_ippooldetails.yaml, spec.validation.openAPIV3Schema.properties.spec.properties.allocations.items.nullable: true.
	Allocations []*int `json:"allocations,omitempty"`
	// Indicates the unallocated IP index, such as 2 means 192.168.2.3 can be used.
	Unallocated []int `json:"unallocated,omitempty"`
	// The specific situation of ip occupied in IPPool. IPRecorder name array.
	Recorders []string `json:"recorders,omitempty"`
}

// IPPoolDetailStatus defines the observed state of IPPoolDetail
type IPPoolDetailStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// IPPoolDetail is the Schema for the ippooldetails API
type IPPoolDetail struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolDetailSpec   `json:"spec,omitempty"`
	Status IPPoolDetailStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolDetailList contains a list of IPPoolDetail
type IPPoolDetailList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPPoolDetail `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPPoolDetail{}, &IPPoolDetailList{})
}
