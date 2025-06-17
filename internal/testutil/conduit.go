package testutil

import (
	"testing"

	"github.com/conduitio/conduit-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetupSampleConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Version:     "v0.13.2",
			Description: "my-description",
			Registry: &v1alpha.SchemaRegistry{
				URL: "http://apicurio:8080/apis/ccompat/v7",
			},
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:          "source-connector",
					Type:          "source",
					Plugin:        "builtin:generator",
					PluginVersion: "latest",
					PluginName:    "builtin:generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
					},
				},
				{
					Name:          "destination-connector",
					Type:          "destination",
					Plugin:        "builtin:log",
					PluginVersion: "latest",
					PluginName:    "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
			Processors: []*v1alpha.ConduitProcessor{
				{
					ID:           "proc1",
					Name:         "proc1",
					Plugin:       "builtin:base64.encode",
					Workers:      2,
					Condition:    "{{ eq .Metadata.key \"pipeline\" }}",
					ProcessorURL: "http://127.0.0.1:8090/api/files/processors/RECORD_ID/FILENAME",
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

	return c
}

func SetupBadNameConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()

	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Description: "my-description",
			Version:     "v0.13.2",
			Registry: &v1alpha.SchemaRegistry{
				URL: "http://apicurio:8080/apis/ccompat/v7",
			},
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:   "source-connector",
					Type:   "source",
					Plugin: "generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
		},
	}

	return c
}

func SetupBadValidationConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			ID:          "my-pipeline",
			Image:       "ghcr.io/conduitio/conduit",
			Description: "my-description",
			Version:     "v0.13.2",
			Registry: &v1alpha.SchemaRegistry{
				URL: "http://apicurio:8080/apis/ccompat/v7",
			},
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:          "source-connector",
					ID:            "source-connector",
					Type:          "source",
					Plugin:        "builtin:kafka",
					PluginName:    "builtin:kafka",
					PluginVersion: "latest",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:kafka",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
		},
	}

	return c
}

func SetupSecretConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Version:     "v0.13.2",
			Description: "my-description",
			Registry: &v1alpha.SchemaRegistry{
				URL: "http://apicurio:8080/apis/ccompat/v7",
			},
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:          "source-connector",
					Type:          "source",
					Plugin:        "builtin:generator",
					PluginVersion: "latest",
					PluginName:    "builtin:generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
						{
							Name: "saslUsername",
							SecretRef: &corev1.SecretKeySelector{
								Key: "key1",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "objref1",
								},
							},
						},
					},
				},
				{
					Name:          "destination-connector",
					Type:          "destination",
					Plugin:        "builtin:log",
					PluginVersion: "latest",
					PluginName:    "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
		},
	}

	return c
}

func SetupSourceProcConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Version:     "v0.13.2",
			Description: "my-description",
			Registry: &v1alpha.SchemaRegistry{
				URL: "http://apicurio:8080/apis/ccompat/v7",
			},
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:          "source-connector",
					Type:          "source",
					Plugin:        "builtin:generator",
					PluginVersion: "latest",
					PluginName:    "builtin:generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
					},
					Processors: []*v1alpha.ConduitProcessor{
						{
							ID:           "proc1",
							Name:         "proc1",
							Plugin:       "standalone:foo.encode",
							Workers:      2,
							Condition:    "{{ eq .Metadata.key \"pipeline\" }}",
							ProcessorURL: "http://127.0.0.1:8090/api/files/processors/RECORD_ID/FILENAME",
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
				{
					Name:          "destination-connector",
					Type:          "destination",
					Plugin:        "builtin:log",
					PluginVersion: "latest",
					PluginName:    "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
			Processors: nil,
		},
	}

	return c
}
