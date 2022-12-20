package controllers_test

import (
	"context"
	"testing"

	"github.com/conduitio-labs/conduit-operator/controllers/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/conduitio-labs/conduit-operator/api/v1"
	ctrls "github.com/conduitio-labs/conduit-operator/controllers"
)

func Test_EnvVars(t *testing.T) {
	c := sampleConduit(false)

	// ordered
	want := []corev1.EnvVar{
		{
			Name: "SETTING3__S_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "setting3-%s-key",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "setting3-secret-name",
					},
				},
			},
		},
		{
			Name: "SETTING2__AKEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "setting2-#akey",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "setting2-secret-name",
					},
				},
			},
		},
	}
	if diff := cmp.Diff(want, ctrls.EnvVars(c)); diff != "" {
		t.Fatalf("env vars mismatch (-want +got): %v", diff)
	}
}

func Test_PipelineConfigYAML(t *testing.T) {
	tests := []struct {
		name    string
		conduit *v1.Conduit
		want    string
	}{
		{
			name:    "with running pipeline",
			conduit: sampleConduit(true),
			want:    mustReadFile("testdata/running-pipeline.yaml"),
		},
		{
			name:    "with stopped pipeline",
			conduit: sampleConduit(false),
			want:    mustReadFile("testdata/stopped-pipeline.yaml"),
		},
		{
			name:    "with processors",
			conduit: sampleConduitWithProcessors(true),
			want:    mustReadFile("testdata/pipeline-with-processors.yaml"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ctrls.PipelineConfigYAML(
				context.Background(),
				mock.NewMockClient(gomock.NewController(t)),
				tc.conduit,
			)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("env vars mismatch (-want +got): %s", diff)
			}
		})
	}
}
