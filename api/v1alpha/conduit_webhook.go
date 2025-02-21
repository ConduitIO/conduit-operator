package v1alpha

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var conduitVerConstraint *semver.Constraints

func init() {
	var err error
	// validate constraint
	sanitized, _ := strings.CutPrefix(ConduitEarliestAvailable, "v")
	conduitVerConstraint, err = semver.NewConstraint(fmt.Sprint(">= ", sanitized))
	if err != nil {
		panic(fmt.Errorf("failed to create version constraint: %w", err))
	}
}

func (r *Conduit) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-operator-conduit-io-v1alpha-conduit,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1alpha,name=mconduit.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Conduit{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Conduit) Default() {
	for _, t := range conduitConditions.GetConditionTypes() {
		cond := r.Status.GetCondition(t)
		if cond == nil {
			r.Status.SetCondition(t, corev1.ConditionUnknown, "", "")
		}
	}

	if r.Spec.Running == nil {
		r.Spec.Running = new(bool)
		*r.Spec.Running = true
	}

	if r.Spec.Name == "" {
		r.Spec.Name = r.ObjectMeta.Name
	}

	if r.Spec.ID == "" {
		r.Spec.ID = r.Spec.Name
	}

	if r.Spec.Image == "" {
		r.Spec.Image = ConduitImage
	}

	if r.Spec.Version == "" {
		r.Spec.Version = ConduitVersion
	}

	for _, c := range r.Spec.Connectors {
		if c.ID == "" {
			c.ID = c.Name
		}

		r.proccessorDefaulter(c.Processors)

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

	r.proccessorDefaulter(r.Spec.Processors)
}

// processorDefaulter adds defaults for processor variables which have not been specified.
func (*Conduit) proccessorDefaulter(pp []*ConduitProcessor) {
	for _, p := range pp {
		if p.Workers == 0 {
			p.Workers = 1
		}

		if p.ID == "" {
			p.ID = p.Name
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-operator-conduit-io-v1alpha-conduit,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1alpha,name=vconduit.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Conduit{}

// ValidateCreate implements webhook.Validator for the Conduit resource.
// Error is returned when the resource is invalid.
func (r *Conduit) ValidateCreate() (admission.Warnings, error) {
	var errs error

	if ok := validateConduitVersion(r.Spec.Version); !ok {
		errs = multierror.Append(errs, fmt.Errorf("unsupported conduit version %s, minimum required %s",
			r.Spec.Version,
			ConduitEarliestAvailable,
		))
	}

	if err := validateConnectors(r.Spec.Connectors); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := validateProcessors(r.Spec.Processors); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := validateRegistry(r.Spec.Registry); err != nil {
		errs = multierror.Append(errs, err)
	}

	return nil, errs
}

// ValidateUpdate implements webhook.Validator for the Conduit resource.
// Error is returned when the changes to the resource are invalid.
func (r *Conduit) ValidateUpdate(runtime.Object) (admission.Warnings, error) {
	return nil, validateConnectors(r.Spec.Connectors)
}

// ValidateDelete implements webhook.Validator for the Conduit resource.
// Error is returned when the object cannot be deleted.
func (r *Conduit) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// validateConnectors validates the attributes of connectors in the slice.
// Error is return when the validation fails.
func validateConnectors(cc []*ConduitConnector) error {
	var errs error

	for _, c := range cc {
		for _, fn := range connectorValidators {
			if err := fn(c); err != nil {
				errs = multierror.Append(
					errs,
					fmt.Errorf("connector validation failure %q: %w", c.Name, err),
				)
			}
		}

		if err := validateProcessors(c.Processors); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func validateProcessors(pp []*ConduitProcessor) error {
	var errs error

	for _, p := range pp {
		for _, fn := range processorValidators {
			if err := fn(p); err != nil {
				errs = multierror.Append(
					errs,
					fmt.Errorf("processor validation failure %q: %w", p.Name, err),
				)
			}
		}
	}

	return errs
}

func validateConduitVersion(ver string) bool {
	sanitized, _ := strings.CutPrefix(ver, "v")
	v, err := semver.NewVersion(sanitized)
	if err != nil {
		return false
	}
	return conduitVerConstraint.Check(v)
}

func validateRegistry(sr *SchemaRegistry) error {
	if sr == nil {
		return nil
	}

	if sr.URL == "" {
		return nil
	}

	_, err := url.Parse(sr.URL)
	if err != nil {
		return fmt.Errorf("failed to validate registry url: %w", err)
	}

	return nil
}
