package controller

import (
	"context"
	_ "embed"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/matryer/is"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	webhook "github.com/conduitio/conduit-operator/internal/webhook/v1alpha"
)

//go:embed testdata/running-pipeline.yaml
var runningPipelineYAML string

//go:embed testdata/stopped-pipeline.yaml
var stoppedPipelineYAML string

//go:embed testdata/pipeline-with-processors.yaml
var pipelineWithProcsYAML string

func cmpStatusConditions(t *testing.T, want, got v1alpha.Conditions) {
	t.Helper()

	is := is.New(t)
	is.Equal("", cmp.Diff(want, got, cmpopts.IgnoreFields(v1alpha.Condition{}, []string{
		"LastTransitionTime",
		"Message",
		"Reason",
	}...)))
}

func sampleConduitWithProcessors(t *testing.T, running bool) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	c := sampleConduit(t, running)
	defaulter := &webhook.ConduitCustomDefaulter{}

	c.Spec.Processors = []*v1alpha.ConduitProcessor{
		{
			Name:      "proc1",
			Plugin:    "builtin:base64.encode",
			Workers:   2,
			Condition: "{{ eq .Metadata.key \"pipeline\" }}",
			Settings: []v1alpha.SettingsVar{
				{
					Name: "setting101",
					SecretRef: &corev1.SecretKeySelector{
						Key: "setting101-%p-key",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "setting101-secret-name",
						},
					},
				},
				{
					Name:  "setting200",
					Value: "setting200-val",
				},
			},
		},
	}

	is.Equal(len(c.Spec.Connectors), 2)

	// source
	c.Spec.Connectors[0].Processors = []*v1alpha.ConduitProcessor{
		{
			Name:      "proc1src",
			Plugin:    "builtin:base64.decode",
			Workers:   1,
			Condition: "{{ eq .Metadata.key \"source\" }}",
			Settings: []v1alpha.SettingsVar{
				{
					Name: "setting0",
					SecretRef: &corev1.SecretKeySelector{
						Key: "setting0-%p-key",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "setting0-secret-name",
						},
					},
				},
				{
					Name:  "setting100",
					Value: "setting100-val",
				},
			},
		},
	}

	// dest
	c.Spec.Connectors[1].Processors = []*v1alpha.ConduitProcessor{
		{
			Name:      "proc1dest",
			Plugin:    "builtin:error",
			Workers:   3,
			Condition: "{{ eq .Metadata.key \"dest\" }}",
			Settings: []v1alpha.SettingsVar{
				{
					Name: "setting0",
					SecretRef: &corev1.SecretKeySelector{
						Key: "setting0-%p-key",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "setting0-secret-name",
						},
					},
				},
				{
					Name:  "setting100",
					Value: "setting100-val",
				},
			},
		},
	}

	// apply defaults as they would
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}

func sampleConduit(t *testing.T, running bool) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	defaulter := &webhook.ConduitCustomDefaulter{}

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
		},
	}

	// apply defaults as they would
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}

func sampleConduitWithRegistry(t *testing.T, running bool) *v1alpha.Conduit {
	t.Helper()

	c := sampleConduit(t, running)

	c.Spec.Registry = &v1alpha.SchemaRegistry{
		URL: "http://localhost:9091/v1",
	}

	return c
}
