package controller

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"
	"sync"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/pkg/conduit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	builderTempPath = "/tmp/connectors"
)

type Buildable interface {
	steps() []string
	key() string
}

type commandBuilder[T Buildable] struct {
	sync.Mutex
	done   map[string]bool
	builds []T // ordered
}

type connectorBuild struct {
	name      string
	goPkg     string
	tmpDir    string
	targetDir string
	ldflags   string
}

func (cb connectorBuild) steps() []string {
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

func (cb connectorBuild) key() string {
	return cb.goPkg
}

type processorBuild struct {
	targetDir string
	procUrl   string
	name      string
}

func (pb processorBuild) steps() []string {
	return []string{
		fmt.Sprintf(
			"wget -O %s/%s %s", pb.targetDir, filepath.Base(pb.procUrl), pb.procUrl,
		),
	}
}

func (pb processorBuild) key() string {
	return pb.name
}

func (c *commandBuilder[T]) renderScript() string {
	var final []string
	for _, build := range c.builds {
		final = append(final, build.steps()...)
	}
	log.Println(strings.Join(final, " && "))
	return strings.Join(final, " && ")
}

func (c *commandBuilder[T]) empty() bool {
	c.Lock()
	defer c.Unlock()
	return len(c.builds) == 0
}

func (c *commandBuilder[T]) addBuild(b T) {
	c.Lock()
	defer c.Unlock()
	if c.done == nil {
		c.done = make(map[string]bool)
	}
	key := b.key()
	if _, ok := c.done[key]; ok {
		return
	}
	c.builds = append(c.builds, b)
	c.done[key] = true
}

// ConduitInitContainers returns a slice of kubernetes container definitions
func ConduitInitContainers(cc []*v1alpha.ConduitConnector) []corev1.Container {
	builder := &commandBuilder[connectorBuild]{}
	pBuilder := &commandBuilder[processorBuild]{}

	containers := []corev1.Container{
		{
			Name:            v1alpha.ConduitInitContainerName,
			Image:           v1alpha.ConduitInitImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"sh", "-xe", "-c",
				fmt.Sprintf("mkdir -p %s %s", v1alpha.ConduitProcessorsPath, v1alpha.ConduitConnectorsPath),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      v1alpha.ConduitStorageVolumeMount,
					MountPath: v1alpha.ConduitVolumePath,
				},
			},
		},
	}

	for _, c := range cc {
		if !strings.HasPrefix(c.Plugin, "builtin") {
			builder.addBuild(connectorBuild{
				name:      fmt.Sprintf("%s-%s", filepath.Base(c.Plugin), c.PluginVersion),
				goPkg:     c.PluginPkg,
				tmpDir:    builderTempPath,
				targetDir: v1alpha.ConduitConnectorsPath,
				ldflags:   fmt.Sprintf(`-ldflags "-X 'github.com/%s.version=%s'"`, c.Plugin, c.PluginVersion),
			})
		}
		for _, p := range c.Processors {
			if !strings.HasPrefix(p.Plugin, "builtin") && p.ProcessorURL != "" {
				pBuilder.addBuild(processorBuild{name: p.Plugin, procUrl: p.ProcessorURL, targetDir: v1alpha.ConduitProcessorsPath})
			}
		}
	}

	if !builder.empty() {
		containers = append(containers, corev1.Container{
			Name:            fmt.Sprint(v1alpha.ConduitInitContainerName, "-connectors"),
			Image:           v1alpha.ConduitInitImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"sh", "-xe",
				"-c", builder.renderScript(),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      v1alpha.ConduitStorageVolumeMount,
					MountPath: v1alpha.ConduitVolumePath,
				},
			},
		})
	}

	if !pBuilder.empty() {
		containers = append(containers, corev1.Container{
			Name:            fmt.Sprint(v1alpha.ConduitInitContainerName, "-processors"),
			Image:           v1alpha.ConduitInitImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"sh", "-xe",
				"-c", pBuilder.renderScript(),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      v1alpha.ConduitStorageVolumeMount,
					MountPath: v1alpha.ConduitVolumePath,
				},
			},
		})
	}

	return containers
}

// ConduitRuntimeContainer returns a Kubernetes container definition
func ConduitRuntimeContainer(image, version string, envVars []corev1.EnvVar, flags *conduit.Flags) (corev1.Container, error) {
	args, err := flags.ForVersion(version)
	if err != nil {
		return corev1.Container{}, err
	}

	return corev1.Container{
		Name:            v1alpha.ConduitContainerName,
		Image:           fmt.Sprint(image, ":", version),
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/app/conduit"},
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
			InitialDelaySeconds: 5,
			TimeoutSeconds:      1,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      v1alpha.ConduitStorageVolumeMount,
				MountPath: v1alpha.ConduitVolumePath,
			},
			{
				Name:      v1alpha.ConduitPipelineVolumeMount,
				MountPath: v1alpha.ConduitPipelinePath,
				ReadOnly:  true,
			},
		},
		Env: envVars,
	}, nil
}
