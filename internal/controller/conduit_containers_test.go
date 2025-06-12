package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/matryer/is"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/pkg/conduit"
)

func Test_ConduitInitConnectorContainers(t *testing.T) {
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
			got := ConduitInitContainers(tc.connectors, []*v1alpha.ConduitProcessor{})
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("container mismatch (-want +got): %v", diff)
			}
		})
	}
}

func Test_ConduitInitProcessorsContainers(t *testing.T) {
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
		processors []*v1alpha.ConduitProcessor
		imageVer   string
		want       []corev1.Container
	}{
		{
			name:       "only builtin processors",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin: "builtin:builtin-processor",
						},
					},
				},
			},
			want: []corev1.Container{initContainer},
		},
		{
			name:       "standalone processor but no URL",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin: "standalone-processor",
						},
					},
				},
			},
			want: []corev1.Container{initContainer},
		},
		{
			name:       "standalone processor with URL",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			name:       "same standalone processor in multiple connectors",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
					},
				},
				{
					Plugin:        "builtin:builtin-test1",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			name:       "multiple standalone processors",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
						{
							Plugin:       "standalone-processor1",
							ProcessorURL: "http://example.com/processor1",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor && wget -O /conduit.storage/processors/processor1 http://example.com/processor1",
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
			name:       "multiple processors - one builtin, one standalone",
			processors: []*v1alpha.ConduitProcessor{},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
						{
							Plugin: "builtin:builtin-processor",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			name: "standalone processor at pipeline level",
			processors: []*v1alpha.ConduitProcessor{
				{
					Plugin:       "standalone-processor",
					ProcessorURL: "http://example.com/processor",
				},
			},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			name: "same standalone processor at pipeline level and at connector level",
			processors: []*v1alpha.ConduitProcessor{
				{
					Plugin:       "standalone-processor",
					ProcessorURL: "http://example.com/processor",
				},
			},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			name: "different standalone processor at pipeline level and at connector level",
			processors: []*v1alpha.ConduitProcessor{
				{
					Plugin:       "standalone-processor1",
					ProcessorURL: "http://example.com/processor1",
				},
			},
			connectors: []*v1alpha.ConduitConnector{
				{
					Plugin:        "builtin:builtin-test",
					PluginVersion: "latest",
					Processors: []*v1alpha.ConduitProcessor{
						{
							Plugin:       "standalone-processor",
							ProcessorURL: "http://example.com/processor",
						},
					},
				},
			},
			want: []corev1.Container{
				initContainer, {
					Name:            "conduit-init-processors",
					Image:           "golang:1.23-alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"sh", "-xe",
						"-c",
						"wget -O /conduit.storage/processors/processor1 http://example.com/processor1 && wget -O /conduit.storage/processors/processor http://example.com/processor",
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
			got := ConduitInitContainers(tc.connectors, tc.processors)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("container mismatch (-want +got): %v", diff)
			}
		})
	}
}

func Test_ConduitRuntimeContainer(t *testing.T) {
	flags := conduit.NewFlags(
		conduit.WithPipelineFile(v1alpha.ConduitPipelineFile),
		conduit.WithConnectorsPath(v1alpha.ConduitConnectorsPath),
		conduit.WithDBPath(v1alpha.ConduitDBPath),
		conduit.WithProcessorsPath(v1alpha.ConduitProcessorsPath),
		conduit.WithLogFormat(v1alpha.ConduitLogFormatJSON),
	)

	runtimeContainer := corev1.Container{
		Name:            "conduit-server",
		Image:           "my-image:v0.13.2",
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/app/conduit"},
		Args: []string{
			"run",
			"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
			"--connectors.path", "/conduit.storage/connectors",
			"--db.type", "sqlite",
			"--db.sqlite.path", "/conduit.storage/db",
			"--pipelines.exit-on-degraded",
			"--pipelines.error-recovery.max-retries", "0",
			"--processors.path", "/conduit.storage/processors",
			"--log.format", "json",
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
			InitialDelaySeconds: 5,
			TimeoutSeconds:      1,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
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

	tests := []struct {
		name    string
		version string
		want    corev1.Container
		wantErr error
	}{
		{
			name:    "runtime container is created",
			version: "v0.13.2",
			want:    runtimeContainer,
			wantErr: nil,
		},
		{
			name:    "error occurs creating runtime container",
			version: "v0.14.0",
			want:    corev1.Container{},
			wantErr: errors.Errorf("version v0.14.0 not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			got, err := ConduitRuntimeContainer(
				"my-image",
				tc.version,
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
				flags,
			)
			if err != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("container mismatch (-want +got): %v", diff)
			}
		})
	}
}
