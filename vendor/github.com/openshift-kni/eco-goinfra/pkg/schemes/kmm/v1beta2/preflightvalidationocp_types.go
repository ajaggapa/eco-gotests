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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreflightValidationOCPSpec describes the desired state of the resource, such as the OCP release image
// that Module CRs need to be verified against as well as the push image flag
// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
// +kubebuilder:validation:Required
type PreflightValidationOCPSpec struct {
	// KernelVersion describes the kernel image that all Modules need to be checked against.
	// +kubebuilder:validation:Required
	KernelVersion string `json:"kernelVersion"`

	// DTKImage contains the DTK image that will be used during in-cluster Build verification
	// for all the modules. In case no Module in the cluster contains build section, no DTK image
	// should be provided
	// +optional
	DTKImage string `json:"dtkImage"`

	// Boolean flag that determines whether images build during preflight must also
	// be pushed to a defined repository
	// +optional
	PushBuiltImage bool `json:"pushBuiltImage"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// PreflightValidationOCP initiates a preflight validations for all Modules on the current OCP cluster.
// +kubebuilder:resource:path=preflightvalidationsocp,scope=Cluster
// +kubebuilder:resource:path=preflightvalidationsocp,scope=Cluster,shortName=pfvo
// +operator-sdk:csv:customresourcedefinitions:displayName="Preflight Validation OCP"
type PreflightValidationOCP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required

	Spec   PreflightValidationOCPSpec `json:"spec,omitempty"`
	Status PreflightValidationStatus  `json:"status,omitempty"`
}

func (p PreflightValidationOCP) Hub() {}

// +kubebuilder:object:root=true

// PreflightValidationList is a list of PreflightValidation objects.
type PreflightValidationOCPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of PreflightValidation. More info:
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []PreflightValidationOCP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreflightValidationOCP{}, &PreflightValidationOCPList{})
}
