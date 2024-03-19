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

func builderContainer(cc []*v1.ConduitConnector) *corev1.Container {
	builder := &commandBuilder{}

	for _, c := range cc {
		switch {
		case
			strings.HasPrefix(c.Plugin, "builtin"),
			strings.Contains(c.Plugin, "conduit-kafka-connect-wrapper"):
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
		return nil
	}

	return &corev1.Container{
		Name:            fmt.Sprint(v1.ConduitInitContainerName, "-connectors"),
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
	}
}

func initContainer(image, version string) *corev1.Container {
	initscript := []string{
		// initialize connector/processors storage
		fmt.Sprint("mkdir -p ", v1.ConduitProcessorsPath, " ", v1.ConduitConnectorsPath),
		// copy vendored connectors directory
		fmt.Sprintf("(test -d /connectors && cp -pr /connectors/. %s/) || true", v1.ConduitConnectorsPath),
	}

	return &corev1.Container{
		Name:            v1.ConduitInitContainerName,
		Image:           fmt.Sprint(image, ":", version),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"sh", "-xe", "-c", strings.Join(initscript, " && "),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      v1.ConduitStorageVolumeMount,
				MountPath: v1.ConduitVolumePath,
			},
		},
	}
}

// ConduitInitContainers returns a slice of kubernetes init container definitions
func ConduitInitContainers(image, version string, cc []*v1.ConduitConnector) []corev1.Container {
	var containers []corev1.Container

	if ic := initContainer(image, version); ic != nil {
		containers = append(containers, *ic)
	}

	if bc := builderContainer(cc); bc != nil {
		containers = append(containers, *bc)
	}

	return containers
}

// ConduitRuntimeContainer returns a Kubernetes container definition
// todo is the pipelineName supposed to be used?
func ConduitRuntimeContainer(image, version string, envVars []corev1.EnvVar) corev1.Container {
	args := []string{
		"/app/conduit",
		"-pipelines.path", v1.ConduitPipelineFile,
		"-connectors.path", v1.ConduitConnectorsPath,
		"-db.type", "badger",
		"-db.badger.path", v1.ConduitDBPath,
		"-pipelines.exit-on-error",
	}

	if withProcessors(version) {
		args = append(args, "-processors.path", v1.ConduitProcessorsPath)
	}

	return corev1.Container{
		Name:            v1.ConduitContainerName,
		Image:           fmt.Sprint(image, ":", version),
		ImagePullPolicy: corev1.PullAlways,
		Args:            args,
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
