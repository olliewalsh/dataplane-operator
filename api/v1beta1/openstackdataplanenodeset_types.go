/*
Copyright 2023.

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

package v1beta1

import (
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackDataPlaneNodeSetSpec defines the desired state of OpenStackDataPlaneNodeSet
type OpenStackDataPlaneNodeSetSpec struct {
	// +kubebuilder:validation:Optional
	// BaremetalSetTemplate Template for BaremetalSet for the NodeSet
	BaremetalSetTemplate baremetalv1.OpenStackBaremetalSetSpec `json:"baremetalSetTemplate,omitempty"`

	// +kubebuilder:validation:Required
	// NodeTemplate - node attributes specific to nodes defined by this resource. These
	// attributes can be overriden at the individual node level, else take their defaults
	// from valus in this section.
	NodeTemplate NodeTemplate `json:"nodeTemplate"`

	// Nodes - Map of Node Names and node specific data. Values here override defaults in the
	// upper level section.
	// +kubebuilder:validation:Required
	Nodes map[string]NodeSection `json:"nodes"`

	// SecretMaxSize - Maximum size in bytes of a Kubernetes secret. This size is currently situated around
	// 1 MiB (nearly 1 MB).
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1048576
	SecretMaxSize int `json:"secretMaxSize" yaml:"secretMaxSize"`

	// +kubebuilder:validation:Optional
	//
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// PreProvisioned - Set to true if the nodes have been Pre Provisioned.
	PreProvisioned bool `json:"preProvisioned,omitempty"`

	// Env is a list containing the environment variables to pass to the pod
	// +kubebuilder:validation:Optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to pass to the ansibleee resource
	// which allows to connect the ansibleee runner to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={download-cache,bootstrap,configure-network,validate-network,install-os,configure-os,ssh-known-hosts,run-os,reboot-os,install-certs,ovn,neutron-metadata,libvirt,nova,telemetry}
	// Services list
	Services []string `json:"services"`

	// TLSEnabled - Whether the node set has TLS enabled.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	TLSEnabled bool `json:"tlsEnabled" yaml:"tlsEnabled"`

	// Tags - Additional tags for NodeSet
	// +kubebuilder:validation:Optional
	Tags []string `json:"tags,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Data Plane NodeSet"
//+kubebuilder:resource:shortName=osdpns;osdpnodeset;osdpnodesets
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackDataPlaneNodeSet is the Schema for the openstackdataplanenodesets API
// OpenStackDataPlaneNodeSet name must be a valid RFC1123 as it is used in labels
type OpenStackDataPlaneNodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDataPlaneNodeSetSpec   `json:"spec,omitempty"`
	Status OpenStackDataPlaneNodeSetStatus `json:"status,omitempty"`
}

// OpenStackDataPlaneNodeSetStatus defines the observed state of OpenStackDataPlaneNodeSet
type OpenStackDataPlaneNodeSetStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Deployed
	Deployed bool `json:"deployed,omitempty" optional:"true"`

	// DeploymentStatuses
	DeploymentStatuses map[string]condition.Conditions `json:"deploymentStatuses,omitempty" optional:"true"`

	// DNSClusterAddresses
	DNSClusterAddresses []string `json:"dnsClusterAddresses,omitempty" optional:"true"`

	// CtlplaneSearchDomain
	CtlplaneSearchDomain string `json:"ctlplaneSearchDomain,omitempty" optional:"true"`

	// AllHostnames
	AllHostnames map[string]map[infranetworkv1.NetNameStr]string `json:"allHostnames,omitempty" optional:"true"`

	// AllIPs
	AllIPs map[string]map[infranetworkv1.NetNameStr]string `json:"allIPs,omitempty" optional:"true"`

	// ConfigMapHashes
	ConfigMapHashes map[string]string `json:"configMapHashes,omitempty" optional:"true"`

	// SecretHashes
	SecretHashes map[string]string `json:"secretHashes,omitempty" optional:"true"`

	// ConfigHash - holds the curret hash of the NodeTemplate and Node sections of the struct.
	// This hash is used to determine when new Ansible executions are required to roll
	// out config changes.
	ConfigHash string `json:"configHash,omitempty"`

	// DeployedConfigHash - holds the hash of the NodeTemplate and Node sections of the struct
	// that was last deployed.
	// This hash is used to determine when new Ansible executions are required to roll
	// out config changes.
	DeployedConfigHash string `json:"deployedConfigHash,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackDataPlaneNodeSetList contains a list of OpenStackDataPlaneNodeSets
type OpenStackDataPlaneNodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDataPlaneNodeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDataPlaneNodeSet{}, &OpenStackDataPlaneNodeSetList{})
}

// IsReady - returns true if the DataPlane is ready
func (instance OpenStackDataPlaneNodeSet) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// InitConditions - Initializes Status Conditons
func (instance *OpenStackDataPlaneNodeSet) InitConditions() {
	instance.Status.Conditions = condition.Conditions{}
	instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)

	cl := condition.CreateList(
		condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(SetupReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(NodeSetIPReservationReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(NodeSetDNSDataReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.InitReason),
	)

	// Only set Baremetal related conditions if we have baremetal hosts included in the
	// baremetalSetTemplate.
	if len(instance.Spec.BaremetalSetTemplate.BaremetalHosts) > 0 {
		cl = append(cl, *condition.UnknownCondition(NodeSetBareMetalProvisionReadyCondition, condition.InitReason, condition.InitReason))
	}

	instance.Status.Conditions.Init(&cl)
	instance.Status.Deployed = false
}

// GetAnsibleEESpec - get the fields that will be passed to AEE
func (instance OpenStackDataPlaneNodeSet) GetAnsibleEESpec() AnsibleEESpec {
	return AnsibleEESpec{
		NetworkAttachments: instance.Spec.NetworkAttachments,
		ExtraMounts:        instance.Spec.NodeTemplate.ExtraMounts,
		Env:                instance.Spec.Env,
		ServiceAccountName: instance.Name,
	}
}

// DataplaneAnsibleImageDefaults default images for dataplane services
type DataplaneAnsibleImageDefaults struct {
	Frr                        string
	IscsiD                     string
	Logrotate                  string
	NeutronMetadataAgent       string
	NeutronSRIOVAgent          string
	NovaCompute                string
	OvnControllerAgent         string
	OvnBgpAgent                string
	TelemetryCeilometerCompute string
	TelemetryCeilometerIpmi    string
	TelemetryNodeExporter      string
}
