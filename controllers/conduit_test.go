package controllers_test

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	v1 "github.com/conduitio-labs/conduit-operator/api/v1"
)

func compareStatusConditions(want, got v1.Conditions) string {
	return cmp.Diff(want, got, cmpopts.IgnoreFields(v1.Condition{}, []string{
		"LastTransitionTime",
		"Message",
		"Reason",
	}...))
}

func mustReadFile(file string) string {
	v, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return string(v)
}

func sampleConduitWithProcessors(running bool) *v1.Conduit {
	c := sampleConduit(running)

	c.Spec.Processors = []*v1.ConduitProcessor{
		{
			Name: "proc1",
			Type: "yaml",
			Settings: []v1.SettingsVar{
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

	if len(c.Spec.Connectors) != 2 {
		panic("unexpected number of connectors")
	}

	// source
	c.Spec.Connectors[0].Processors = []*v1.ConduitProcessor{
		{
			Name: "proc1src",
			Type: "js",
			Settings: []v1.SettingsVar{
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
	c.Spec.Connectors[1].Processors = []*v1.ConduitProcessor{
		{
			Name: "proc1dest",
			Type: "js",
			Settings: []v1.SettingsVar{
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

	return c
}

func sampleConduit(running bool) *v1.Conduit {
	return &v1.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1.ConduitSpec{
			Running:     running,
			Name:        "my-pipeline",
			Description: "my-description",
			Connectors: []*v1.ConduitConnector{
				{
					Name:       "source-connector",
					Type:       "source",
					PluginName: "standalone:generator",
					Settings: []v1.SettingsVar{
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
					Name:       "destination-connector",
					Type:       "destination",
					PluginName: "builtin:file",
					Settings: []v1.SettingsVar{
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
}
