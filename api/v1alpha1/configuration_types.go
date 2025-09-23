/*
Copyright 2025.

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

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// Content is the content to be written to the file
	Content string `json:"content"`

	// Create indicates whether to create the file if it does not exist
	Create bool `json:"create,omitempty"`
	// +optional
	Permission *uint32 `json:"permission,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration.
type ConfigurationStatus struct {
	// LastUpdated is the last time the configuration was updated
	LastUpdated metav1.Time `json:"lastUpdated"`

	// Content is the current content of the file at the specified path
	Content string `json:"content,omitempty"`

	// FileExists indicates whether the file exists at the specified path
	FileExists bool `json:"fileExists,omitempty"`
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Path",type="string",JSONPath=".spec.path",description="Path of the file"
// +kubebuilder:printcolumn:name="Exists",type="boolean",JSONPath=".status.fileExists",description="Tells if the file exists"
// +kubebuilder:printcolumn:name="LastUpdate",type="date",JSONPath=".status.lastUpdated",description="Last update of the file"

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Configuration
	// +required
	Spec ConfigurationSpec `json:"spec"`

	// status defines the observed state of Configuration
	// +optional
	Status ConfigurationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ConfigurationList contains a list of Configuration
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}
