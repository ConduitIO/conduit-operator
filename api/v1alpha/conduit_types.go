package v1alpha

import (
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ConditionConduitReady             ConditionType = "Ready"
	ConditionConduitConfigReady       ConditionType = "ConfigReady"
	ConditionConduitVolumeReady       ConditionType = "VolumeBound"
	ConditionConduitDeploymentRunning ConditionType = "DeploymentRunning"
	ConditionConduitServiceReady      ConditionType = "ServiceReady"

	ConduitFinalizer = "finalizers.operator.conduit.io"
)

const (
	ErroredReason  = "Error"
	CreatedReason  = "Created"
	UpdatedReason  = "Updated"
	RunningReason  = "Running"
	PendingReason  = "Pending"
	VolBoundReason = "VolumeBound"
	DeletedReason  = "Deleted"
)

var conduitConditions = NewConditionSet(
	ConditionConduitReady,
	ConditionConduitConfigReady,
	ConditionConduitVolumeReady,
	ConditionConduitDeploymentRunning,
	ConditionConduitServiceReady,
)

const (
	ConduitVersion             = "v0.11.1"
	ConduitImage               = "ghcr.io/conduitio/conduit"
	ConduitContainerName       = "conduit-server"
	ConduitPipelinePath        = "/conduit.pipelines"
	ConduitVolumePath          = "/conduit.storage"
	ConduitDBPath              = "/conduit.storage/db"
	ConduitConnectorsPath      = "/conduit.storage/connectors"
	ConduitProcessorsPath      = "/conduit.storage/processors"
	ConduitStorageVolumeMount  = "conduit-storage"
	ConduitPipelineVolumeMount = "conduit-pipelines"
	ConduitInitImage           = "golang:1.23-alpine"
	ConduitInitContainerName   = "conduit-init"
)

var (
	ConduitPipelineFile          = path.Join(ConduitPipelinePath, "pipeline.yaml")
	ConduitWithProcessorsVersion = "0.9.0"
)

// ConduitSpec defines the desired state of Conduit
type ConduitSpec struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	Running     bool   `json:"running,omitempty"`
	Version     string `json:"version,omitempty"`

	Connectors []*ConduitConnector `json:"connectors,omitempty"`
	Processors []*ConduitProcessor `json:"processors,omitempty"`
}

type ConduitConnector struct {
	Name          string `json:"name,omitempty"`
	Type          string `json:"type,omitempty"`
	Plugin        string `json:"plugin,omitempty"`
	PluginName    string `json:"pluginName,omitempty"`
	PluginPkg     string `json:"pluginPkg,omitempty"`
	PluginVersion string `json:"pluginVersion,omitempty"`

	Settings   []SettingsVar       `json:"settings,omitempty"`
	Processors []*ConduitProcessor `json:"processors,omitempty"`
}

type ConduitProcessor struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Workers int    `json:"workers,omitempty"`

	Settings []SettingsVar `json:"settings,omitempty"`
}

type GlobalConfigMapRef struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Key       string `json:"key,omitempty"`
}

type SettingsVar struct {
	Name         string                    `json:"name,omitempty"`
	Value        string                    `json:"value,omitempty"`
	SecretRef    *corev1.SecretKeySelector `json:"secretRef,omitempty"`
	ConfigMapRef *GlobalConfigMapRef       `json:"configMapRef,omitempty"`
}

// ConduitStatus defines the observed state of Conduit
type ConduitStatus struct {
	ObservedGeneration int64        `json:"observedGeneration,omitempty"`
	Conditions         Conditions   `json:"conditions,omitempty"`
	UpdatedAt          *metav1.Time `json:"updatedAt,omitempty"`
}

func (s *ConduitStatus) SetCondition(ct ConditionType, status corev1.ConditionStatus, reason string, message string) {
	s.Conditions = conduitConditions.SetCondition(s.Conditions, ct, status, reason, message)
}

func (s *ConduitStatus) GetCondition(ct ConditionType) *Condition {
	return conduitConditions.GetCondition(s.Conditions, ct)
}

// ConditionChanged returns true when the expected condition status does not match current status
func (s *ConduitStatus) ConditionChanged(ct ConditionType, expected corev1.ConditionStatus) bool {
	if cond := conduitConditions.GetCondition(s.Conditions, ct); cond != nil {
		return cond.Status != expected
	}

	return false
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Conduit is the Schema for the conduits API
type Conduit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConduitSpec   `json:"spec,omitempty"`
	Status ConduitStatus `json:"status,omitempty"`
}

func (r *Conduit) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprint("conduit-server-", r.Name),
		Namespace: r.Namespace,
	}
}

//+kubebuilder:object:root=true

// ConduitList contains a list of Conduit
type ConduitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Conduit `json:"items"`
}

// +k8s:deepcopy-gen=false
// ConduitInstanceConfig contains metadata which will be passed to each conduit deployment
type ConduitInstanceMetadata struct {
	PodAnnotations map[string]string `yaml:"podAnnotations"`
	Labels         map[string]string `yaml:"labels"`
}

func init() {
	SchemeBuilder.Register(&Conduit{}, &ConduitList{})
}