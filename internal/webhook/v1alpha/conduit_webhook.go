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

package v1alpha

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Masterminds/semver/v3"
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	internalconduit "github.com/conduitio/conduit-operator/internal/conduit"
)

var conduitVerConstraint *semver.Constraints

func init() {
	var err error
	// validate constraint
	sanitized, _ := strings.CutPrefix(v1alpha.ConduitEarliestAvailable, "v")
	conduitVerConstraint, err = semver.NewConstraint(fmt.Sprint(">= ", sanitized))
	if err != nil {
		panic(fmt.Errorf("failed to create version constraint: %w", err))
	}
}

// SetupConduitWebhookWithManager registers the webhook for Conduit in the manageconduit.
func SetupConduitWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha.Conduit{}).
		WithValidator(&ConduitCustomValidator{}).
		WithDefaulter(&ConduitCustomDefaulter{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-operator-conduit-io-v1alpha-conduit,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1alpha,name=mconduit-v1alpha.kb.io,admissionReviewVersions=v1

// ConduitCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Conduit when those are created or updated.
type ConduitCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &ConduitCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Conduit.
func (d *ConduitCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	conduit, ok := obj.(*v1alpha.Conduit)

	if !ok {
		return fmt.Errorf("expected an Conduit object but got %T", obj)
	}

	for _, t := range v1alpha.ConduitConditions.GetConditionTypes() {
		cond := conduit.Status.GetCondition(t)
		if cond == nil {
			conduit.Status.SetCondition(t, corev1.ConditionUnknown, "", "")
		}
	}

	if conduit.Spec.Running == nil {
		conduit.Spec.Running = new(bool)
		*conduit.Spec.Running = true
	}

	if conduit.Spec.Name == "" {
		conduit.Spec.Name = conduit.ObjectMeta.Name
	}

	if conduit.Spec.ID == "" {
		conduit.Spec.ID = conduit.Spec.Name
	}

	if conduit.Spec.Image == "" {
		conduit.Spec.Image = v1alpha.ConduitImage
	}

	if conduit.Spec.Version == "" {
		conduit.Spec.Version = v1alpha.ConduitVersion
	}

	for _, c := range conduit.Spec.Connectors {
		if c.ID == "" {
			c.ID = c.Name
		}

		d.proccessorDefaulter(c.Processors)

		if c.PluginVersion == "" {
			c.PluginVersion = "latest"
		}

		plugin := strings.ToLower(c.Plugin)

		switch {
		case slices.Contains(internalconduit.BuiltinConnectors, plugin):
			c.Plugin = "builtin:" + plugin
			c.PluginName = c.Plugin
		case strings.HasPrefix(plugin, "builtin:"):
			c.PluginName = c.Plugin
		default:
			pluginName := strings.TrimPrefix(filepath.Base(c.Plugin), "conduit-connector-")
			c.PluginPkg = fmt.Sprintf("github.com/%s/cmd/connector@%s", c.Plugin, c.PluginVersion)
			c.PluginName = fmt.Sprintf("standalone:%s", pluginName)
		}
	}

	d.proccessorDefaulter(conduit.Spec.Processors)

	return nil
}

// processorDefaulter adds defaults for processor variables which have not been specified.
func (*ConduitCustomDefaulter) proccessorDefaulter(pp []*v1alpha.ConduitProcessor) {
	for _, p := range pp {
		if p.Workers == 0 {
			p.Workers = 1
		}

		if p.ID == "" {
			p.ID = p.Name
		}
	}
}

//+kubebuilder:webhook:path=/validate-operator-conduit-io-v1alpha-conduit,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.conduit.io,resources=conduits,verbs=create;update,versions=v1alpha,name=vconduit-v1alpha.kb.io,admissionReviewVersions=v1

// ConduitCustomValidator struct is responsible for validating the Conduit resource
// when it is created, updated, or deleted.
type ConduitCustomValidator struct{}

var _ webhook.CustomValidator = &ConduitCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Conduit.
func (v *ConduitCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	conduit, ok := obj.(*v1alpha.Conduit)
	if !ok {
		return nil, fmt.Errorf("expected a Conduit object but got %T", obj)
	}

	var errs field.ErrorList

	if err := v.validateConduitVersion(conduit.Spec.Version); err != nil {
		errs = append(errs, err)
	}

	if verrs := v.validateConnectors(conduit.Spec.Connectors); len(verrs) > 0 {
		errs = append(errs, verrs...)
	}

	if verrs := v.validateProcessors(
		conduit.Spec.Processors,
		field.NewPath("spec").Child("processors"),
	); len(verrs) > 0 {
		errs = append(errs, verrs...)
	}

	if err := v.validateRegistry(conduit.Spec.Registry); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// TODO should this be conduit.Spec.Name?
		return nil, apierrors.NewInvalid(v1alpha.GroupKind, conduit.Name, errs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Conduit.
func (v *ConduitCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	conduit, ok := newObj.(*v1alpha.Conduit)
	if !ok {
		return nil, fmt.Errorf("expected a Conduit object for the newObj but got %T", newObj)
	}

	if errs := v.validateConnectors(conduit.Spec.Connectors); len(errs) > 0 {
		return nil, apierrors.NewInvalid(v1alpha.GroupKind, conduit.Name, errs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Conduit.
func (v *ConduitCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if _, ok := obj.(*v1alpha.Conduit); !ok {
		return nil, fmt.Errorf("expected a Conduit object but got %T", obj)
	}

	return nil, nil
}

// validateConnectors validates the attributes of connectors in the slice.
// Error is return when the validation fails.
func (v *ConduitCustomValidator) validateConnectors(cc []*v1alpha.ConduitConnector) field.ErrorList {
	var errs field.ErrorList

	fp := field.NewPath("spec").Child("connectors")
	for _, c := range cc {
		err := validateConnectorParameters(c, fp)
		errs = append(errs, err)

		for _, fn := range connectorValidators {
			if err := fn(c, fp); err != nil {
				errs = append(errs, err)
			}
		}

		if procErrs := v.validateProcessors(c.Processors, fp); procErrs != nil {
			errs = append(errs, procErrs...)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (*ConduitCustomValidator) validateProcessors(pp []*v1alpha.ConduitProcessor, fp *field.Path) field.ErrorList {
	var errs field.ErrorList

	for _, p := range pp {
		if err := validateProcessorPlugin(p, fp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (*ConduitCustomValidator) validateConduitVersion(ver string) *field.Error {
	sanitized, _ := strings.CutPrefix(ver, "v")
	fp := field.NewPath("spec").Child("version")

	v, err := semver.NewVersion(sanitized)
	if err != nil {
		return field.Invalid(fp, ver, err.Error())
	}

	if ok := conduitVerConstraint.Check(v); !ok {
		return field.Invalid(fp, ver, fmt.Sprintf(
			"unsupported conduit version %q, minimum required %q", ver, v1alpha.ConduitEarliestAvailable,
		))
	}

	return nil
}

func (*ConduitCustomValidator) validateRegistry(sr *v1alpha.SchemaRegistry) *field.Error {
	if sr == nil || sr.URL == "" {
		return nil
	}

	if _, err := url.Parse(sr.URL); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("schemaRegistry").Child("url"),
			sr.URL,
			err.Error(),
		)
	}

	return nil
}
