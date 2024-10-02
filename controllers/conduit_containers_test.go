package controllers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	ctrls "github.com/conduitio/conduit-operator/controllers"
)

func Test_ConduitInitContainers(t *testing.T) {
	initContainer := corev1.Container{
		Name:            "conduit-init",
		Image:           "golang:1.23-alpine",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"sh", "-xe", "-c",
			"mkdir -p /conduit.storage/processors /conduit.storage/connectors",
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "conduit-storage", MountPath: "/conduit.storage"}},
	}

	tests := []struct {
		name       string
		connectors []*v1alpha.ConduitConnector
		imageVer   string
		want       []corev1.Container
	}{
		{
			name: "only builtin connectors",
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
			},
			want: []corev1.Container{initContainer},
		},
		{
			name: "with latest standalone connector",
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test@latest",
					PluginVersion: "latest",
					Plugin:        "conduitio/conduit-connector-test",
				},
			},
			want: []corev1.Container{
				initContainer,
				{
					Name:            "conduit-init-connectors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c", `mkdir -p /tmp/connectors /conduit.storage/connectors && env CGO_ENABLED=0 GOBIN=/tmp/connectors/conduit-connector-test-latest go install -ldflags "-X 'github.com/conduitio/conduit-connector-test.version=latest'" github.com/conduitio/conduit-connector-test@latest && install -D /tmp/connectors/conduit-connector-test-latest/connector /conduit.storage/connectors/conduit-connector-test-latest`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "conduit-storage",
							MountPath: "/conduit.storage",
						},
					},
				},
			},
		},
		{
			name: "with version standalone connector",
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test@v1.0.0",
					PluginVersion: "v1.0.0",
					Plugin:        "conduitio/conduit-connector-test",
				},
			},
			want: []corev1.Container{
				initContainer,
				{
					Name:            "conduit-init-connectors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c", `mkdir -p /tmp/connectors /conduit.storage/connectors && env CGO_ENABLED=0 GOBIN=/tmp/connectors/conduit-connector-test-v1.0.0 go install -ldflags "-X 'github.com/conduitio/conduit-connector-test.version=v1.0.0'" github.com/conduitio/conduit-connector-test@v1.0.0 && install -D /tmp/connectors/conduit-connector-test-v1.0.0/connector /conduit.storage/connectors/conduit-connector-test-v1.0.0`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "conduit-storage",
							MountPath: "/conduit.storage",
						},
					},
				},
			},
		},
		{
			name: "with multiple standalone connector",
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test-1@v1.0.0",
					PluginVersion: "v1.0.0",
					Plugin:        "conduitio/conduit-connector-test-1",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test-2@v2.0.0",
					PluginVersion: "v2.0.0",
					Plugin:        "conduitio/conduit-connector-test-2",
				},
			},
			want: []corev1.Container{
				initContainer,
				{
					Name:            "conduit-init-connectors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c", `mkdir -p /tmp/connectors /conduit.storage/connectors && env CGO_ENABLED=0 GOBIN=/tmp/connectors/conduit-connector-test-1-v1.0.0 go install -ldflags "-X 'github.com/conduitio/conduit-connector-test-1.version=v1.0.0'" github.com/conduitio/conduit-connector-test-1@v1.0.0 && install -D /tmp/connectors/conduit-connector-test-1-v1.0.0/connector /conduit.storage/connectors/conduit-connector-test-1-v1.0.0 && mkdir -p /tmp/connectors /conduit.storage/connectors && env CGO_ENABLED=0 GOBIN=/tmp/connectors/conduit-connector-test-2-v2.0.0 go install -ldflags "-X 'github.com/conduitio/conduit-connector-test-2.version=v2.0.0'" github.com/conduitio/conduit-connector-test-2@v2.0.0 && install -D /tmp/connectors/conduit-connector-test-2-v2.0.0/connector /conduit.storage/connectors/conduit-connector-test-2-v2.0.0`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "conduit-storage",
							MountPath: "/conduit.storage",
						},
					},
				},
			},
		},
		{
			name: "with duplicate standalone connector",
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test-1@v1.0.0",
					PluginVersion: "v1.0.0",
					Plugin:        "conduitio/conduit-connector-test-1",
				},
				{
					PluginPkg:     "github.com/conduitio/conduit-connector-test-1@v1.0.0",
					PluginVersion: "v1.0.0",
					Plugin:        "conduitio/conduit-connector-test-1",
				},
			},
			want: []corev1.Container{
				initContainer,
				{
					Name:            "conduit-init-connectors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c", `mkdir -p /tmp/connectors /conduit.storage/connectors && env CGO_ENABLED=0 GOBIN=/tmp/connectors/conduit-connector-test-1-v1.0.0 go install -ldflags "-X 'github.com/conduitio/conduit-connector-test-1.version=v1.0.0'" github.com/conduitio/conduit-connector-test-1@v1.0.0 && install -D /tmp/connectors/conduit-connector-test-1-v1.0.0/connector /conduit.storage/connectors/conduit-connector-test-1-v1.0.0`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "conduit-storage",
							MountPath: "/conduit.storage",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ctrls.ConduitInitContainers(tc.connectors)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("container mismatch (-want +got): %v", diff)
			}
		})
	}
}

func Test_ConduitRuntimeContainer(t *testing.T) {
	want := corev1.Container{
		Name:            "conduit-server",
		Image:           "my-image:v0.8.0",
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"/app/conduit",
			"-pipelines.path", "/conduit.pipelines/pipeline.yaml",
			"-connectors.path", "/conduit.storage/connectors",
			"-db.type", "sqlite",
			"-db.sqlite.path", "/conduit.storage/db",
			"-pipelines.exit-on-error",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "grpc",
				ContainerPort: 8084,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Scheme: "HTTP",
					Port:   intstr.FromString("http"),
				},
			},
			TimeoutSeconds:   1,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "conduit-storage",
				MountPath: "/conduit.storage",
			},
			{
				Name:      "conduit-pipelines",
				MountPath: "/conduit.pipelines",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "var-1",
				Value: "val-1",
			},
			{
				Name:  "var-2",
				Value: "val-2",
			},
		},
	}

	got := ctrls.ConduitRuntimeContainer(
		"my-image",
		"v0.8.0",
		[]corev1.EnvVar{
			{
				Name:  "var-1",
				Value: "val-1",
			},
			{
				Name:  "var-2",
				Value: "val-2",
			},
		},
	)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("container mismatch (-want +got): %v", diff)
	}
}

func Test_ConduitRuntimeContainerProcessors(t *testing.T) {
	want := corev1.Container{
		Name:            "conduit-server",
		Image:           "my-image:v0.9.1",
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"/app/conduit",
			"-pipelines.path", "/conduit.pipelines/pipeline.yaml",
			"-connectors.path", "/conduit.storage/connectors",
			"-db.type", "sqlite",
			"-db.sqlite.path", "/conduit.storage/db",
			"-pipelines.exit-on-error",
			"-processors.path",
			"/conduit.storage/processors",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "grpc",
				ContainerPort: 8084,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Scheme: "HTTP",
					Port:   intstr.FromString("http"),
				},
			},
			TimeoutSeconds:   1,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "conduit-storage",
				MountPath: "/conduit.storage",
			},
			{
				Name:      "conduit-pipelines",
				MountPath: "/conduit.pipelines",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "var-1",
				Value: "val-1",
			},
			{
				Name:  "var-2",
				Value: "val-2",
			},
		},
	}

	got := ctrls.ConduitRuntimeContainer(
		"my-image",
		"v0.9.1",
		[]corev1.EnvVar{
			{
				Name:  "var-1",
				Value: "val-1",
			},
			{
				Name:  "var-2",
				Value: "val-2",
			},
		},
	)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("container mismatch (-want +got): %v", diff)
	}
}
