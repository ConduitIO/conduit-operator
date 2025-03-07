//go:generate mockgen --build_flags=--mod=mod -destination=mock/client_mock.go -package=mock sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter
//go:generate mockgen --build_flags=--mod=mod -destination=mock/recorder_mock.go -package=mock k8s.io/client-go/tools/record EventRecorder

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	v1 "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Log levels
const (
	Info = iota
	Debug
)

// ConduitReconciler reconciles a Conduit object
type ConduitReconciler struct {
	Metadata *v1.ConduitInstanceMetadata

	client.Client
	logr.Logger
	record.EventRecorder
}

//+kubebuilder:rbac:groups=operator.conduit.io,resources=conduits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.conduit.io,resources=conduits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.conduit.io,resources=conduits/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch;create

//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile difference between the desired state of the Conduit resource and runtime.
// Updates to the status and any errors encountered are accumulated through the progress of the loop.
// Returns all accumulated errors at the end of the loop.
func (r *ConduitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		conduit v1.Conduit
		errs    error

		result = ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}
	)

	if err := r.Get(ctx, req.NamespacedName, &conduit); err != nil {
		if !apierrors.IsNotFound(err) {
			r.Logger.Error(err, "unable to fetch Conduit")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if deletedAt := conduit.GetDeletionTimestamp(); deletedAt != nil {
		ctrlutil.RemoveFinalizer(&conduit, v1.ConduitFinalizer)
		if err := r.Update(ctx, &conduit); err != nil {
			return result, err
		}

		r.Eventf(&conduit, corev1.EventTypeNormal, v1.DeletedReason, "Conduit %q was deleted on %q", req.NamespacedName, deletedAt)
		return ctrl.Result{}, nil
	}

	if !ctrlutil.ContainsFinalizer(&conduit, v1.ConduitFinalizer) {
		ctrlutil.AddFinalizer(&conduit, v1.ConduitFinalizer)
		if err := r.Update(ctx, &conduit); err != nil {
			return result, err
		}
		return result, nil
	}

	if err := r.CreateOrUpdateConfig(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error reconciling config: %w", err))
	}

	if err := r.CreateOrUpdateSecret(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error reconciling config: %w", err))
	}

	if err := r.CreateOrUpdateVolume(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error reconciling volume: %w", err))
	}

	if err := r.CreateOrUpdateDeployment(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error reconciling deployment: %w", err))
	}

	if err := r.CreateOrUpdateService(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error reconciling service: %w", err))
	}

	if err := r.UpdateStatus(ctx, &conduit); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error updating status: %w", err))
	}

	return result, errs
}

// CreateOrUpdateConfig ensures the ConfigMap resource containing
// the Conduit pipeline is consistent with the Conduit resource.
// Status conditions are set depending on the outcome of the operation.
func (r *ConduitReconciler) CreateOrUpdateConfig(ctx context.Context, c *v1.Conduit) error {
	var (
		nn = c.NamespacedName()
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
		}
	)

	y, err := PipelineConfigYAML(ctx, r.Client, c)
	if err != nil {
		return err
	}

	cmFn := func() error {
		cm.Data = map[string]string{
			"pipeline.yaml": y,
		}

		c.Status.SetCondition(
			v1.ConditionConduitConfigReady,
			corev1.ConditionTrue,
			"ConfigReady",
			"Pipeline configuration created",
		)

		return ctrlutil.SetControllerReference(c, cm, r.Scheme())
	}

	op, err := ctrlutil.CreateOrUpdate(ctx, r, cm, cmFn)
	if err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to reconcile config %q: %s", nn, err)
		c.Status.SetCondition(
			v1.ConditionConduitConfigReady,
			corev1.ConditionFalse,
			"ConfigError",
			err.Error(),
		)
		return err
	}

	switch op {
	case ctrlutil.OperationResultUpdated:
		r.Eventf(c, corev1.EventTypeNormal, v1.UpdatedReason, "Conduit config %q updated", nn)
	case ctrlutil.OperationResultCreated:
		r.Eventf(c, corev1.EventTypeNormal, v1.CreatedReason, "Conduit config %q created", nn)
	}

	return nil
}

// CreateOrUpdateSecret creates a secret which will be used by the conduit instance
// to store sensitive information (credentials, etc)
func (r *ConduitReconciler) CreateOrUpdateSecret(ctx context.Context, c *v1.Conduit) error {
	var (
		nn     = c.NamespacedName()
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
		}
	)

	secretData, err := SchemaRegistryConfig(ctx, r.Client, c)
	if err != nil {
		return err
	}

	secretFn := func() error {
		secret.Data = secretData

		c.Status.SetCondition(
			v1.ConditionConduitSecretReady,
			corev1.ConditionTrue,
			"SecretReady",
			"Conduit secret created",
		)

		return ctrlutil.SetControllerReference(c, secret, r.Scheme())
	}

	op, err := ctrlutil.CreateOrUpdate(ctx, r, secret, secretFn)
	if err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to reconcile config %q: %s", nn, err)
		c.Status.SetCondition(
			v1.ConditionConduitSecretReady,
			corev1.ConditionFalse,
			"ConduitSecretError",
			err.Error(),
		)
		return err
	}

	switch op {
	case ctrlutil.OperationResultUpdated:
		r.Eventf(c, corev1.EventTypeNormal, v1.UpdatedReason, "Conduit secret %q updated", nn)
	case ctrlutil.OperationResultCreated:
		r.Eventf(c, corev1.EventTypeNormal, v1.CreatedReason, "Conduit secret %q created", nn)
	}

	return nil
}

// CreateOrUpdateVolume creates / updates the Conduit db volume resource.
// Status conditions are set depending on the outcome of the operation.
func (r *ConduitReconciler) CreateOrUpdateVolume(ctx context.Context, c *v1.Conduit) error {
	var (
		nn  = c.NamespacedName()
		pvc = ConduitVolumeClaim(nn, "1Gi")
	)

	pvcFn := func() error {
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if c.Status.ConditionChanged(v1.ConditionConduitVolumeReady, corev1.ConditionTrue) {
				r.Eventf(c, corev1.EventTypeNormal, v1.VolBoundReason, "Conduit db volume %q bound", nn)
			}
			c.Status.SetCondition(
				v1.ConditionConduitVolumeReady,
				corev1.ConditionTrue,
				"VolumeReady",
				string(pvc.Status.Phase),
			)
		default:
			c.Status.SetCondition(
				v1.ConditionConduitVolumeReady,
				corev1.ConditionFalse,
				"VolumeReady",
				string(pvc.Status.Phase),
			)
		}
		return ctrlutil.SetControllerReference(c, pvc, r.Scheme())
	}

	op, err := ctrlutil.CreateOrUpdate(ctx, r, pvc, pvcFn)
	if err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to reconcile volume %q: %s", nn, err)
		c.Status.SetCondition(
			v1.ConditionConduitVolumeReady,
			corev1.ConditionFalse,
			"VolumeClaimError",
			err.Error(),
		)
		return err
	}

	switch op {
	case ctrlutil.OperationResultUpdated:
		r.Eventf(c, corev1.EventTypeNormal, v1.UpdatedReason, "Conduit db volume %q updated", nn)
	case ctrlutil.OperationResultCreated:
		r.Eventf(c, corev1.EventTypeNormal, v1.CreatedReason, "Conduit db volume %q created", nn)
	}

	return nil
}

// CreateOrUpdateDeployment creates and the deployment to manage the runtime of the Conduit server.
// Updates on the Conduit resource will result in changes to the deployment and trigger a rolling upgrade.
// Status conditions are set depending on the outcome of the operation.
func (r *ConduitReconciler) CreateOrUpdateDeployment(ctx context.Context, c *v1.Conduit) error {
	var (
		cm       = corev1.ConfigMap{}
		secret   = corev1.Secret{}
		nn       = c.NamespacedName()
		replicas = r.getReplicas(c)

		deployment = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
		}
		labels = map[string]string{
			"app.kubernetes.io/name": nn.Name,
		}
		annotations = make(map[string]string)
	)

	if err := r.Get(ctx, nn, &cm); err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to get %q configmap: %s", nn, err)
		return err
	}

	if err := r.Get(ctx, nn, &secret); err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to get %q secret: %s", nn, err)
		return err
	}

	annotations["operator.conduit.io/config-map-version"] = cm.ResourceVersion

	// merge instance metadata
	maps.Copy(labels, r.Metadata.Labels)
	maps.Copy(annotations, r.Metadata.PodAnnotations)

	merge := func(current, updated *appsv1.DeploymentSpec) error {
		v, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		return json.Unmarshal(v, current)
	}

	deploymentFn := func() error {
		envVars := EnvVars(c)

		if vars := envVarsFromSecret(&secret); len(vars) > 0 {
			envVars = append(envVars, vars...)
			slices.SortFunc(envVars, func(a, b corev1.EnvVar) int {
				return strings.Compare(a.Name, b.Name)
			})
		}

		container, err := ConduitRuntimeContainer(c.Spec.Image, c.Spec.Version, envVars)
		if err != nil {
			return err
		}

		spec := appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyAlways,
					InitContainers: ConduitInitContainers(c.Spec.Connectors),
					Containers: []corev1.Container{
						container,
					},
					Volumes: []corev1.Volume{
						ConduitVolume(nn.Name),
						ConduitPipelineVol(nn.Name),
					},
				},
			},
		}

		// N.B. Work around defaulted fields to prevent a continues overwriting of
		//      the deployment spec.
		if err := merge(&deployment.Spec, &spec); err != nil {
			return err
		}

		// Ensure labels and annotations are always exact.
		deployment.Spec.Selector.MatchLabels = labels
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Template.ObjectMeta.Annotations = annotations

		status := deployment.Status
		readyReplicas := fmt.Sprintf("%d/%d", status.ReadyReplicas, *spec.Replicas)

		runningStatus, reason := r.deploymentRunningStatus(&deployment)

		r.Logger.V(Debug).Info("deployment status", "deployment", deployment.Name, "status", runningStatus, "reason", reason)

		switch runningStatus {
		case corev1.ConditionTrue:
			if c.Status.ConditionChanged(v1.ConditionConduitDeploymentRunning, runningStatus) {
				r.Eventf(c, corev1.EventTypeNormal, reason, "Conduit deployment %q running, config %q", nn, cm.ResourceVersion)
			}

			c.Status.SetCondition(
				v1.ConditionConduitDeploymentRunning,
				runningStatus,
				reason,
				readyReplicas,
			)
		case corev1.ConditionFalse:
			if c.Status.ConditionChanged(v1.ConditionConduitDeploymentRunning, runningStatus) {
				r.Eventf(c, corev1.EventTypeNormal, reason, "Conduit deployment %q stopped, config %q", nn, cm.ResourceVersion)
			}

			c.Status.SetCondition(
				v1.ConditionConduitDeploymentRunning,
				runningStatus,
				reason,
				readyReplicas,
			)
		default:
			r.Eventf(c, corev1.EventTypeNormal, reason, "Conduit deployment %q pending, config %q", nn, cm.ResourceVersion)
		}

		return ctrlutil.SetControllerReference(c, &deployment, r.Scheme())
	}

	op, err := ctrlutil.CreateOrUpdate(ctx, r, &deployment, deploymentFn)
	if err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to reconcile deployment %q: %s", nn, err)
		c.Status.SetCondition(
			v1.ConditionConduitDeploymentRunning,
			corev1.ConditionFalse,
			"DeploymentError",
			err.Error(),
		)
		return err
	}

	switch op {
	case ctrlutil.OperationResultUpdated:
		r.Eventf(c, corev1.EventTypeNormal, v1.UpdatedReason, "Conduit deployment %q updated, config %q", nn, cm.ResourceVersion)
	case ctrlutil.OperationResultCreated:
		r.Eventf(c, corev1.EventTypeNormal, v1.CreatedReason, "Conduit deployment %q created", nn)
	}
	return nil
}

// CreateOrUpdateService create a service pointing to the Conduit deployment
// Status conditions are set depending on the outcome of the operation.
func (r *ConduitReconciler) CreateOrUpdateService(ctx context.Context, c *v1.Conduit) error {
	nn := c.NamespacedName()
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}
	serviceFn := func() error {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:     "http",
				Protocol: corev1.ProtocolTCP,
				Port:     80,
				TargetPort: intstr.IntOrString{
					IntVal: 8080,
				},
			},
		}
		service.Spec.Selector = map[string]string{
			"app.kubernetes.io/name": nn.Name,
		}

		c.Status.SetCondition(
			v1.ConditionConduitServiceReady,
			corev1.ConditionTrue,
			"ServiceReady",
			fmt.Sprintf("Address http://%s:80", service.Spec.ClusterIP),
		)
		return ctrlutil.SetControllerReference(c, &service, r.Scheme())
	}

	op, err := ctrlutil.CreateOrUpdate(ctx, r, &service, serviceFn)
	if err != nil {
		r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to reconcile service %q: %s", nn, err)
		c.Status.SetCondition(
			v1.ConditionConduitServiceReady,
			corev1.ConditionFalse,
			"ServiceError",
			err.Error(),
		)
		return err
	}

	switch op {
	case ctrlutil.OperationResultUpdated:
		r.Eventf(c, corev1.EventTypeNormal, v1.UpdatedReason, "Conduit service %q updated", nn)
	case ctrlutil.OperationResultCreated:
		r.Eventf(c, corev1.EventTypeNormal, v1.CreatedReason, "Conduit service %q created", nn)
	}

	return nil
}

// UpdateStatus keeps the Conduit resource status current using the commulative statuses
// of config, volume, deployment and service.
func (r *ConduitReconciler) UpdateStatus(ctx context.Context, c *v1.Conduit) error {
	var latestConduit v1.Conduit
	if err := r.Get(ctx, client.ObjectKeyFromObject(c), &latestConduit); err != nil {
		return err
	}

	// TODO: Expand status to include connector build and conduit server pod state.
	//       Status only tracks the high level state shared by the deployment.
	//       Investigating any failed pods will require further digging.
	//       It will be useful to expose these conditions:
	//       * Init container responsible for building connectors on bootup
	//       * Conduit server pod running or failing, etc.
	if !equality.Semantic.DeepEqual(latestConduit.Status, c.Status) {
		now := metav1.Now()
		c.Status.UpdatedAt = &now
		c.Status.ObservedGeneration = c.Generation

		if err := r.Status().Update(ctx, c); err != nil {
			r.Eventf(c, corev1.EventTypeWarning, v1.ErroredReason, "Failed to update status: %s", err)
			return err
		}

		r.Event(c, corev1.EventTypeNormal, v1.UpdatedReason, "Status updated")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConduitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: runtime.NumCPU(),
		}).
		For(&v1.Conduit{}).
		Owns(&corev1.Secret{}).
		// todo needs to be watched, because of the Kafka bootstrap servers
		// Owns(&corev1.ConfigMap{}).
		// NB: These don't need to be watched. I think
		// Owns(&appsv1.Deployment{}).
		// Owns(&corev1.PersistentVolumeClaim{}).
		// Owns(&corev1.PersistentVolume{}).
		Complete(r)
}

func (r *ConduitReconciler) getReplicas(c *v1.Conduit) int32 {
	if c.Spec.Running != nil && *c.Spec.Running {
		return 1
	}
	return 0
}

func (r *ConduitReconciler) deploymentRunningStatus(d *appsv1.Deployment) (corev1.ConditionStatus, string) {
	r.Logger.V(Debug).Info("deployment status",
		"deployment", d.Name,
		"replicas", d.Status.Replicas,
		"ready_replicas", d.Status.ReadyReplicas,
		"updated_replicas", d.Status.UpdatedReplicas,
		"available_replicas", d.Status.AvailableReplicas,
		"unavailable_replicas", d.Status.UnavailableReplicas,
	)

	// When the deployment is scaled down, return not running (false)
	if *d.Spec.Replicas == 0 {
		return corev1.ConditionFalse, v1.StoppedReason
	}

	// Status has not been updated yet.
	if d.Status.Replicas == 0 {
		return corev1.ConditionUnknown, v1.PendingReason
	}

	if d.Status.ReadyReplicas >= d.Status.Replicas {
		return corev1.ConditionTrue, v1.RunningReason
	}

	if d.Status.UnavailableReplicas >= d.Status.Replicas {
		return corev1.ConditionFalse, v1.DegradedReason
	}

	return corev1.ConditionUnknown, v1.PendingReason
}
