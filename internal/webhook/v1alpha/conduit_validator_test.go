package v1alpha

import (
	"context"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/matryer/is"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidator_ConnectorPlugin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "connector plugin is valid",
			setup: func() *v1alpha.Conduit {
				return createConduit(t, true)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := createConduit(t, true)
				c.Spec.Connectors[0].Plugin = ""
				return c
			},
			wantErr: field.Invalid(
				field.NewPath("spec", "connectors", "plugin"),
				"",
				"plugin \"\" is not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			fp := field.NewPath("spec").Child("connectors")

			err := validateConnectorPlugin(c.Spec.Connectors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func TestValidator_ConnectorPluginType(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "connector plugin is valid",
			setup: func() *v1alpha.Conduit {
				return createConduit(t, true)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := createConduit(t, true)
				c.Spec.Connectors[0].Type = ""
				return c
			},
			wantErr: field.Invalid(
				field.NewPath("spec", "connectors", "type"),
				"",
				"plugin type \"\" is not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			fp := field.NewPath("spec").Child("connectors")

			err := validateConnectorPluginType(c.Spec.Connectors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func TestValidator_ProcessorPlugin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "processor plugin is valid",
			setup: func() *v1alpha.Conduit {
				return createConduit(t, true)
			},
		},
		{
			name: "processor plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := createConduit(t, true)
				c.Spec.Processors[0].Plugin = ""
				return c
			},
			wantErr: field.Required(
				field.NewPath("spec", "processors", "plugin"),
				"plugin cannot be empty"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			fp := field.NewPath("spec").Child("processors")

			err := validateProcessorPlugin(c.Spec.Processors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

// TODO using similar to controller -- should fix?
func createConduit(t *testing.T, running bool) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	defaulter := ConduitCustomDefaulter{}

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Description: "my-description",
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:   "source-connector",
					Type:   "source",
					Plugin: "builtin:generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "setting1",
							Value: "setting1-val",
						},
						{
							Name:  "setting2",
							Value: "setting2-val",
						},
						{
							Name: "setting3",
							SecretRef: &corev1.SecretKeySelector{
								Key: "setting3-%s-key",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "setting3-secret-name",
								},
							},
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:file",
					Settings: []v1alpha.SettingsVar{
						{
							Name: "setting2",
							SecretRef: &corev1.SecretKeySelector{
								Key: "setting2-#akey",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "setting2-secret-name",
								},
							},
						},
					},
				},
			},
			Processors: []*v1alpha.ConduitProcessor{
				{
					Name:      "proc1",
					Plugin:    "builtin:base64.encode",
					Workers:   2,
					Condition: "{{ eq .Metadata.key \"pipeline\" }}",
					Settings: []v1alpha.SettingsVar{
						{
							Name: "setting01",
							SecretRef: &corev1.SecretKeySelector{
								Key: "setting01-%p-key",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "setting01-secret-name",
								},
							},
						},
						{
							Name:  "setting02",
							Value: "setting02-val",
						},
					},
				},
			},
		},
	}

	// apply defaults
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}
