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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	dataplanev1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/dataplane-operator/pkg/deployment"
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/rolebinding"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/serviceaccount"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1beta1"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
)

var dataplaneAnsibleImageDefaults dataplanev1.DataplaneAnsibleImageDefaults

const (
	// FrrDefaultImage -
	FrrDefaultImage = "quay.io/podified-antelope-centos9/openstack-frr:current-podified"
	// IscsiDDefaultImage -
	IscsiDDefaultImage = "quay.io/podified-antelope-centos9/openstack-iscsid:current-podified"
	// LogrotateDefaultImage -
	LogrotateDefaultImage = "quay.io/podified-antelope-centos9/openstack-cron:current-podified"
	// MultipathdDefaultImage -
	MultipathdDefaultImage = "quay.io/podified-antelope-centos9/openstack-multipathd:current-podified"
	// NeutronMetadataAgentDefaultImage -
	NeutronMetadataAgentDefaultImage = "quay.io/podified-antelope-centos9/openstack-neutron-metadata-agent-ovn:current-podified"
	// NeutronSRIOVAgentDefaultImage -
	NeutronSRIOVAgentDefaultImage = "quay.io/podified-antelope-centos9/openstack-neutron-sriov-agent:current-podified"
	// NovaComputeDefaultImage -
	NovaComputeDefaultImage = "quay.io/podified-antelope-centos9/openstack-nova-compute:current-podified"
	// OvnControllerAgentDefaultImage -
	OvnControllerAgentDefaultImage = "quay.io/podified-antelope-centos9/openstack-ovn-controller:current-podified"
	// OvnBgpAgentDefaultImage -
	OvnBgpAgentDefaultImage = "quay.io/podified-antelope-centos9/openstack-ovn-bgp-agent:current-podified"
	// TelemetryCeilometerComputeDefaultImage -
	TelemetryCeilometerComputeDefaultImage = "quay.io/podified-antelope-centos9/openstack-ceilometer-compute:current-podified"
	// TelemetryCeilometerIpmiDefaultImage -
	TelemetryCeilometerIpmiDefaultImage = "quay.io/podified-antelope-centos9/openstack-ceilometer-ipmi:current-podified"
	// TelemetryNodeExporterDefaultImage -
	TelemetryNodeExporterDefaultImage = "quay.io/prometheus/node-exporter:v1.5.0"
)

// SetupAnsibleImageDefaults -
func SetupAnsibleImageDefaults() {
	dataplaneAnsibleImageDefaults = dataplanev1.DataplaneAnsibleImageDefaults{
		Frr:                        util.GetEnvVar("RELATED_IMAGE_EDPM_FRR_IMAGE_URL_DEFAULT", FrrDefaultImage),
		IscsiD:                     util.GetEnvVar("RELATED_IMAGE_EDPM_ISCSID_IMAGE_URL_DEFAULT", IscsiDDefaultImage),
		Logrotate:                  util.GetEnvVar("RELATED_IMAGE_EDPM_LOGROTATE_CROND_IMAGE_URL_DEFAULT", LogrotateDefaultImage),
		Multipathd:                 util.GetEnvVar("RELATED_IMAGE_EDPM_MULTIPATHD_IMAGE_URL_DEFAULT", MultipathdDefaultImage),
		NeutronMetadataAgent:       util.GetEnvVar("RELATED_IMAGE_EDPM_NEUTRON_METADATA_AGENT_IMAGE_URL_DEFAULT", NeutronMetadataAgentDefaultImage),
		NeutronSRIOVAgent:          util.GetEnvVar("RELATED_IMAGE_EDPM_NEUTRON_SRIOV_AGENT_IMAGE_URL_DEFAULT", NeutronSRIOVAgentDefaultImage),
		NovaCompute:                util.GetEnvVar("RELATED_IMAGE_EDPM_NOVA_COMPUTE_IMAGE_URL_DEFAULT", NovaComputeDefaultImage),
		OvnControllerAgent:         util.GetEnvVar("RELATED_IMAGE_EDPM_OVN_CONTROLLER_AGENT_IMAGE_URL_DEFAULT", OvnControllerAgentDefaultImage),
		OvnBgpAgent:                util.GetEnvVar("RELATED_IMAGE_EDPM_OVN_BGP_AGENT_IMAGE_URL_DEFAULT", OvnBgpAgentDefaultImage),
		TelemetryCeilometerCompute: util.GetEnvVar("RELATED_IMAGE_EDPM_CEILOMETER_COMPUTE_IMAGE_URL_DEFAULT", TelemetryCeilometerComputeDefaultImage),
		TelemetryCeilometerIpmi:    util.GetEnvVar("RELATED_IMAGE_EDPM_CEILOMETER_IPMI_IMAGE_URL_DEFAULT", TelemetryCeilometerIpmiDefaultImage),
		TelemetryNodeExporter:      util.GetEnvVar("RELATED_IMAGE_EDPM_NODE_EXPORTER_IMAGE_URL_DEFAULT", TelemetryNodeExporterDefaultImage),
	}
}

const (
	// AnsibleSSHPrivateKey ssh private key
	AnsibleSSHPrivateKey = "ssh-privatekey"
	// AnsibleSSHAuthorizedKeys authorized keys
	AnsibleSSHAuthorizedKeys = "authorized_keys"
)

// OpenStackDataPlaneNodeSetReconciler reconciles a OpenStackDataPlaneNodeSet object
type OpenStackDataPlaneNodeSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackDataPlaneNodeSetReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackDataPlaneNodeSet")
}

//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets/finalizers,verbs=update
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplaneservices/finalizers,verbs=update
//+kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets/status,verbs=get
//+kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=network.openstack.org,resources=ipsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.openstack.org,resources=ipsets/status,verbs=get
//+kubebuilder:rbac:groups=network.openstack.org,resources=ipsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=network.openstack.org,resources=netconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs,verbs=get;list;watch
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs/status,verbs=get
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata/status,verbs=get
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;

// RBAC for the ServiceAccount for the internal image registry
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid,resources=securitycontextconstraints,verbs=use
//+kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups="",resources=projects,verbs=get
//+kubebuilder:rbac:groups="project.openshift.io",resources=projects,verbs=get
//+kubebuilder:rbac:groups="",resources=imagestreamimages,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=imagestreammappings,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=imagestreams,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=imagestreams/layers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=imagestreamtags,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=imagetags,verbs=get;list;watch
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreamimages,verbs=get;list;watch
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreammappings,verbs=get;list;watch
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreams,verbs=get;list;watch
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreams/layers,verbs=get
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagetags,verbs=get;list;watch
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreamtags,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackDataPlaneNodeSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *OpenStackDataPlaneNodeSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling NodeSet")

	validate := validator.New()

	// Fetch the OpenStackDataPlaneNodeSet instance
	instance := &dataplanev1.OpenStackDataPlaneNodeSet{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, _ := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)

	// initialize status if Conditions is nil, but do not reset if it already
	// exists
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the conditions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Reset all conditions to Unknown as the state is not yet known for
	// this reconcile loop.
	instance.InitConditions()
	// Set ObservedGeneration since we've reset conditions
	instance.Status.ObservedGeneration = instance.Generation

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() { // update the Ready condition based on the sub conditions
		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, dataplanev1.NodeSetReadyMessage)
		} else if instance.Status.Conditions.IsUnknown(condition.ReadyCondition) {
			// Recalculate ReadyCondition based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			Log.Error(err, "Error updating instance status conditions")
			_err = err
			return
		}
	}()

	if instance.Status.ConfigMapHashes == nil {
		instance.Status.ConfigMapHashes = make(map[string]string)
	}
	if instance.Status.SecretHashes == nil {
		instance.Status.SecretHashes = make(map[string]string)
	}

	instance.Status.Conditions.MarkFalse(dataplanev1.SetupReadyCondition, condition.RequestedReason, condition.SeverityInfo, condition.ReadyInitMessage)

	// Detect config changes and set Status ConfigHash
	configHash, err := r.GetSpecConfigHash(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if configHash != instance.Status.DeployedConfigHash {
		instance.Status.ConfigHash = configHash
	}

	// Ensure Services
	err = deployment.EnsureServices(ctx, helper, instance, validate)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			dataplanev1.SetupReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			dataplanev1.DataPlaneNodeSetErrorMessage,
			err.Error())
		return ctrl.Result{}, err
	}

	// Ensure IPSets Required for Nodes
	allIPSets, isReady, err := deployment.EnsureIPSets(ctx, helper, instance)
	if err != nil || !isReady {
		return ctrl.Result{}, err
	}

	// Ensure DNSData Required for Nodes
	dnsData := deployment.DataplaneDNSData{}
	err = dnsData.EnsureDNSData(
		ctx, helper,
		instance, allIPSets)
	if err != nil || !isReady {
		return ctrl.Result{}, err
	}

	instance.Status.DNSClusterAddresses = dnsData.ClusterAddresses
	instance.Status.CtlplaneSearchDomain = dnsData.CtlplaneSearchDomain
	instance.Status.AllHostnames = dnsData.Hostnames
	instance.Status.AllIPs = dnsData.AllIPs

	ansibleSSHPrivateKeySecret := instance.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret

	secretKeys := []string{}
	secretKeys = append(secretKeys, AnsibleSSHPrivateKey)
	if !instance.Spec.PreProvisioned {
		secretKeys = append(secretKeys, AnsibleSSHAuthorizedKeys)
	}
	_, result, err = secret.VerifySecret(
		ctx,
		types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      ansibleSSHPrivateKeySecret,
		},
		secretKeys,
		helper.GetClient(),
		time.Second*5,
	)
	if err != nil {
		if (result != ctrl.Result{}) {
			instance.Status.Conditions.MarkFalse(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				dataplanev1.InputReadyWaitingMessage,
				"secret/"+ansibleSSHPrivateKeySecret)
		} else {
			instance.Status.Conditions.MarkFalse(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityError,
				err.Error())
		}
		return result, err
	}

	// all our input checks out so report InputReady
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	// Reconcile ServiceAccount
	nodeSetServiceAccount := serviceaccount.NewServiceAccount(
		&corev1.ServiceAccount{
			ObjectMeta: v1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
		},
		time.Duration(10),
	)
	saResult, err := nodeSetServiceAccount.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceAccountReadyErrorMessage,
			err.Error())
		return saResult, err
	} else if (saResult != ctrl.Result{}) {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ServiceAccountCreatingMessage)
		return saResult, nil
	}

	regViewerRoleBinding := rolebinding.NewRoleBinding(
		&rbacv1.RoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "registry-viewer",
			},
		},
		time.Duration(10),
	)
	rbResult, err := regViewerRoleBinding.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceAccountReadyErrorMessage,
			err.Error())
		return rbResult, err
	} else if (rbResult != ctrl.Result{}) {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ServiceAccountCreatingMessage)
		return rbResult, nil
	}

	instance.Status.Conditions.MarkTrue(
		condition.ServiceAccountReadyCondition,
		condition.ServiceAccountReadyMessage)

	// Reconcile BaremetalSet if required
	if !instance.Spec.PreProvisioned {
		// Reset the NodeSetBareMetalProvisionReadyCondition to unknown
		instance.Status.Conditions.MarkUnknown(dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.InitReason, condition.InitReason)
		isReady, err := deployment.DeployBaremetalSet(ctx, helper, instance,
			allIPSets, dnsData.ServerAddresses)
		if err != nil || !isReady {
			return ctrl.Result{}, err
		}
	}

	if instance.Status.Deployed && instance.DeletionTimestamp.IsZero() {
		// The NodeSet is already deployed and not being deleted, so reconciliation
		// is already complete.
		Log.Info("NodeSet already deployed", "instance", instance)
		return ctrl.Result{}, nil
	}

	// Generate NodeSet Inventory
	_, err = deployment.GenerateNodeSetInventory(ctx, helper, instance,
		allIPSets, dnsData.ServerAddresses, dataplaneAnsibleImageDefaults)
	if err != nil {
		errorMsg := fmt.Sprintf("Unable to generate inventory for %s", instance.Name)
		util.LogErrorForObject(helper, err, errorMsg, instance)
		instance.Status.Conditions.MarkFalse(
			dataplanev1.SetupReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			dataplanev1.DataPlaneNodeSetErrorMessage,
			errorMsg)
		return ctrl.Result{}, err
	}

	// all setup tasks complete, mark SetupReadyCondition True
	instance.Status.Conditions.MarkTrue(dataplanev1.SetupReadyCondition, condition.ReadyMessage)

	// Set DeploymentReadyCondition to False if it was unknown.
	// Handles the case where the NodeSet is created, but not yet deployed.
	if instance.Status.Conditions.IsUnknown(condition.DeploymentReadyCondition) {
		Log.Info("Set DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.NotRequestedReason, condition.SeverityInfo,
			condition.DeploymentReadyInitMessage)
	}

	deploymentExists, isDeploymentReady, err := checkDeployment(helper, instance)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			condition.DeploymentReadyErrorMessage,
			err.Error())
		Log.Error(err, "Unable to get deployed OpenStackDataPlaneDeployments.")
		return ctrl.Result{}, err
	}
	if isDeploymentReady {
		Log.Info("Set NodeSet DeploymentReadyCondition true")
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition,
			condition.DeploymentReadyMessage)
	} else if deploymentExists {
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage)
	} else {
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
			condition.DeploymentReadyInitMessage)
	}
	return ctrl.Result{}, nil
}

func checkDeployment(helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
) (bool, bool, error) {
	// Get all completed deployments
	deployments := &dataplanev1.OpenStackDataPlaneDeploymentList{}
	opts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	err := helper.GetClient().List(context.Background(), deployments, opts...)
	if err != nil {
		helper.GetLogger().Error(err, "Unable to retrieve OpenStackDataPlaneDeployment CRs %v")
		return false, false, err
	}

	isDeploymentReady := false
	deploymentExists := false

	// Sort deployments from oldest to newest by the LastTransitionTime of
	// their DeploymentReadyCondition
	slices.SortFunc(deployments.Items, func(a, b dataplanev1.OpenStackDataPlaneDeployment) int {
		aReady := a.Status.Conditions.Get(condition.DeploymentReadyCondition)
		bReady := b.Status.Conditions.Get(condition.DeploymentReadyCondition)
		if aReady != nil && bReady != nil {
			if aReady.LastTransitionTime.Before(&bReady.LastTransitionTime) {
				return -1
			}
		}
		return 1
	})

	for _, deployment := range deployments.Items {
		if !deployment.DeletionTimestamp.IsZero() {
			continue
		}
		if slices.Contains(
			deployment.Spec.NodeSets, instance.Name) {
			deploymentExists = true
			isDeploymentReady = false
			if deployment.Status.Deployed {
				isDeploymentReady = true
				for k, v := range deployment.Status.ConfigMapHashes {
					instance.Status.ConfigMapHashes[k] = v
				}
				for k, v := range deployment.Status.SecretHashes {
					instance.Status.SecretHashes[k] = v
				}
				instance.Status.DeployedConfigHash = deployment.Status.NodeSetHashes[instance.Name]
			}
			deploymentConditions := deployment.Status.NodeSetConditions[instance.Name]
			if instance.Status.DeploymentStatuses == nil {
				instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)
			}
			instance.Status.DeploymentStatuses[deployment.Name] = deploymentConditions
			if condition.IsError(deployment.Status.Conditions.Get(condition.ReadyCondition)) {
				err = fmt.Errorf("check deploymentStatuses for more details")
			}
		}
	}

	return deploymentExists, isDeploymentReady, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackDataPlaneNodeSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// index for ConfigMaps listed on ansibleVarsFrom
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&dataplanev1.OpenStackDataPlaneNodeSet{}, "spec.ansibleVarsFrom.ansible.configMaps",
		func(rawObj client.Object) []string {
			nodeSet := rawObj.(*dataplanev1.OpenStackDataPlaneNodeSet)
			configMaps := make([]string, 0)

			appendConfigMaps := func(varsFrom []dataplanev1.AnsibleVarsFromSource) {
				for _, ref := range varsFrom {
					if ref.ConfigMapRef != nil {
						configMaps = append(configMaps, ref.ConfigMapRef.Name)
					}
				}
			}

			appendConfigMaps(nodeSet.Spec.NodeTemplate.Ansible.AnsibleVarsFrom)
			for _, node := range nodeSet.Spec.Nodes {
				appendConfigMaps(node.Ansible.AnsibleVarsFrom)
			}
			return configMaps
		}); err != nil {
		return err
	}

	// index for Secrets listed on ansibleVarsFrom
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&dataplanev1.OpenStackDataPlaneNodeSet{}, "spec.ansibleVarsFrom.ansible.secrets",
		func(rawObj client.Object) []string {
			nodeSet := rawObj.(*dataplanev1.OpenStackDataPlaneNodeSet)
			secrets := make([]string, 0, len(nodeSet.Spec.Nodes)+1)
			if nodeSet.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret != "" {
				secrets = append(secrets, nodeSet.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret)
			}

			appendSecrets := func(varsFrom []dataplanev1.AnsibleVarsFromSource) {
				for _, ref := range varsFrom {
					if ref.SecretRef != nil {
						secrets = append(secrets, ref.SecretRef.Name)
					}
				}
			}

			appendSecrets(nodeSet.Spec.NodeTemplate.Ansible.AnsibleVarsFrom)
			for _, node := range nodeSet.Spec.Nodes {
				appendSecrets(node.Ansible.AnsibleVarsFrom)
			}
			return secrets
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataplanev1.OpenStackDataPlaneNodeSet{}).
		Owns(&ansibleeev1.OpenStackAnsibleEE{}).
		Owns(&baremetalv1.OpenStackBaremetalSet{}).
		Owns(&infranetworkv1.IPSet{}).
		Owns(&infranetworkv1.DNSData{}).
		Owns(&corev1.Secret{}).
		Watches(&infranetworkv1.DNSMasq{},
			handler.EnqueueRequestsFromMapFunc(r.dnsMasqWatcherFn)).
		Watches(&dataplanev1.OpenStackDataPlaneDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.deploymentWatcherFn)).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.secretWatcherFn),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretWatcherFn),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

func (r *OpenStackDataPlaneNodeSetReconciler) secretWatcherFn(
	ctx context.Context, obj client.Object) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)

	selector := "spec.ansibleVarsFrom.ansible.configMaps"
	if kind == "secret" {
		selector = "spec.ansibleVarsFrom.ansible.secrets"
	}

	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(selector, obj.GetName()),
		Namespace:     obj.GetNamespace(),
	}

	if err := r.List(ctx, nodeSets, listOpts); err != nil {
		Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeSets.Items))
	for _, nodeSet := range nodeSets.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      nodeSet.Name,
			},
		})
		Log.Info(fmt.Sprintf("reconcile loop for openstackdataplanenodeset %s triggered by %s %s",
			nodeSet.Name, kind, obj.GetName()))
	}
	return requests
}

func (r *OpenStackDataPlaneNodeSetReconciler) dnsMasqWatcherFn(
	ctx context.Context, obj client.Object) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}

	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	if err := r.Client.List(ctx, nodeSets, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeSets.Items))
	for _, nodeSet := range nodeSets.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      nodeSet.Name,
			},
		})
	}
	return requests
}

func (r *OpenStackDataPlaneNodeSetReconciler) deploymentWatcherFn(
	ctx context.Context, obj client.Object) []reconcile.Request {
	Log := r.GetLogger(ctx)
	namespace := obj.GetNamespace()
	deployment := obj.(*dataplanev1.OpenStackDataPlaneDeployment)

	requests := make([]reconcile.Request, 0, len(deployment.Spec.NodeSets))
	for _, nodeSet := range deployment.Spec.NodeSets {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      nodeSet,
			},
		})
	}

	podsInterface := r.Kclient.CoreV1().Pods(namespace)
	podsList, err := podsInterface.List(ctx, v1.ListOptions{
		LabelSelector: fmt.Sprintf("openstackdataplanedeployment=%s", deployment.Name),
		FieldSelector: "status.phase=Failed",
	})

	if err != nil {
		Log.Error(err, "unable to retrieve list of pods for dataplane diagnostic")
	} else {
		for _, pod := range podsList.Items {
			Log.Info(fmt.Sprintf("openstackansibleee job %s failed due to %s with message: %s", pod.Name, pod.Status.Reason, pod.Status.Message))
		}
	}
	return requests
}

// GetSpecConfigHash initialises a new struct with only the field we want to check for variances in.
// We then hash the contents of the new struct using md5 and return the hashed string.
func (r *OpenStackDataPlaneNodeSetReconciler) GetSpecConfigHash(instance *dataplanev1.OpenStackDataPlaneNodeSet) (string, error) {
	configHash, err := util.ObjectHash(&instance.Spec)
	if err != nil {
		return "", err
	}

	return configHash, nil
}
