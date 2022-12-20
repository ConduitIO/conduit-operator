package controllers

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"

	v1 "github.com/conduitio-labs/conduit-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	builderTempPath = "/tmp/connectors"
)

type commandBuilder struct {
	sync.Mutex
	done   map[string]bool
	builds []connectorBuild // ordered
}

type connectorBuild struct {
	name      string
	goPkg     string
	tmpDir    string
	targetDir string
	ldflags   string
}

func (cb *connectorBuild) steps() []string {
	return []string{
		fmt.Sprint("mkdir -p ", cb.tmpDir, " ", cb.targetDir),
		fmt.Sprint(
			"env CGO_ENABLED=0 GOBIN=", path.Join(cb.tmpDir, cb.name), " ",
			"go install ", cb.ldflags, " ", cb.goPkg,
		),
		fmt.Sprint(
			"install -D ", path.Join(cb.tmpDir, cb.name, "connector"), " ", path.Join(cb.targetDir, cb.name),
		),
	}
}

func (c *commandBuilder) renderScript() string {
	var final []string
	for _, build := range c.builds {
		final = append(final, build.steps()...)
	}
	return strings.Join(final, " && ")
}

func (c *commandBuilder) empty() bool {
	c.Lock()
	defer c.Unlock()
	return len(c.builds) == 0
}

func (c *commandBuilder) addConnectorBuild(b connectorBuild) {
	c.Lock()
	defer c.Unlock()

	if c.done == nil {
		c.done = make(map[string]bool)
	}

	if _, ok := c.done[b.goPkg]; ok {
		return
	}

	c.builds = append(c.builds, b)
	c.done[b.goPkg] = true
}

// ConduitInitContainers returns a slice of kubernetes container definitions
func ConduitInitContainers(cc []*v1.ConduitConnector) []corev1.Container {
	builder := &commandBuilder{}

	for _, c := range cc {
		if strings.HasPrefix(c.Plugin, "builtin") {
			continue
		}
		builder.addConnectorBuild(connectorBuild{
			name:      fmt.Sprintf("%s-%s", filepath.Base(c.Plugin), c.PluginVersion),
			goPkg:     c.PluginPkg,
			tmpDir:    builderTempPath,
			targetDir: v1.ConduitConnectorsPath,
			ldflags:   fmt.Sprintf(`-ldflags "-X 'github.com/%s.version=%s'"`, c.Plugin, c.PluginVersion),
		})
	}

	if builder.empty() {
		return []corev1.Container{}
	}

	return []corev1.Container{
		{
			Name:            v1.ConduitInitContainerName,
			Image:           v1.ConduitInitImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"sh", "-xe",
				"-c", builder.renderScript(),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      v1.ConduitStorageVolumeMount,
					MountPath: v1.ConduitVolumePath,
				},
			},
		},
	}
}

// ConduitRuntimeContainer returns a Kubernetes container definition
// todo is the pipelineName supposed to be used?
func ConduitRuntimeContainer(image string, envVars []corev1.EnvVar) corev1.Container {
	return corev1.Container{
		Name:            v1.ConduitContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"/app/conduit",
			"-pipelines.path", v1.ConduitPipelineFile,
			"-connectors.path", v1.ConduitConnectorsPath,
			"-db.type", "badger",
			"-db.badger.path", v1.ConduitDBPath,
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
				Name:      v1.ConduitStorageVolumeMount,
				MountPath: v1.ConduitVolumePath,
			},
			{
				Name:      v1.ConduitPipelineVolumeMount,
				MountPath: v1.ConduitPipelinePath,
				ReadOnly:  true,
			},
		},
		Env: envVars,
	}
}
