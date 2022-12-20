package v1

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Conduit) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-operator-conduit-io-v1-conduit,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1,name=mconduit.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Conduit{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Conduit) Default() {
	for _, t := range conduitConditions.GetConditionTypes() {
		cond := r.Status.GetCondition(t)
		if cond == nil {
			r.Status.SetCondition(t, corev1.ConditionUnknown, "", "")
		}
	}

	if r.Spec.Version == "" {
		r.Spec.Version = ConduitVersion
	}

	for _, c := range r.Spec.Connectors {
		c.Plugin = strings.ToLower(c.Plugin)

		if strings.HasPrefix(c.Plugin, "builtin:") {
			c.PluginName = c.Plugin
			continue
		}

		if c.PluginVersion == "" {
			c.PluginVersion = "latest"
		}

		pluginName := strings.TrimPrefix(filepath.Base(c.Plugin), "conduit-connector-")

		c.PluginPkg = fmt.Sprintf("github.com/%s/cmd/connector@%s", c.Plugin, c.PluginVersion)
		c.PluginName = fmt.Sprintf("standalone:%s", pluginName)
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-operator-conduit-io-v1-conduit,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1,name=vconduit.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Conduit{}

// ValidateCreate implements webhook.Validator for the Conduit resource.
// Error is returned when the resource is invalid.
func (r *Conduit) ValidateCreate() (admission.Warnings, error) {
	return validateConnectors(r.Spec.Connectors)
}

// ValidateUpdate implements webhook.Validator for the Conduit resource.
// Error is returned when the changes to the resource are invalid.
func (r *Conduit) ValidateUpdate(runtime.Object) (admission.Warnings, error) {
	return validateConnectors(r.Spec.Connectors)
}

// ValidateDelete implements webhook.Validator for the Conduit resource.
// Error is returned when the object cannot be deleted.
func (r *Conduit) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// validateConnectors validates the attributes of connectors in the slice.
// Error is return when the validation fails.
func validateConnectors(cc []*ConduitConnector) (admission.Warnings, error) {
	var errors error

	for _, c := range cc {
		for _, v := range connectorValidators {
			if err := v(c); err != nil {
				errors = multierror.Append(
					errors,
					fmt.Errorf("connector validation failure %q: %w", c.Name, err),
				)
			}
		}
	}

	return nil, errors
}
