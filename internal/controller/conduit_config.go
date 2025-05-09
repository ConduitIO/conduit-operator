package controller

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit/pkg/conduit"
	cyaml "github.com/conduitio/conduit/pkg/provisioning/config/yaml/v2"

	"github.com/conduitio/yaml/v3"
	corev1 "k8s.io/api/core/v1"
)

const (
	pipelineConfigVersion = "2.2"
)

const (
	schemaRegistryConnStringEnvVar = "CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING"
	schemaRegistryTypeEnvVar       = "CONDUIT_SCHEMA_REGISTRY_TYPE"
)

func SchemaRegistryConfig(ctx context.Context, cl client.Client, c *v1alpha.Conduit) (map[string][]byte, error) {
	data := make(map[string][]byte)

	if c.Spec.Registry == nil || c.Spec.Registry.URL == "" {
		return data, nil
	}

	u, err := url.Parse(c.Spec.Registry.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry URL: %w", err)
	}

	// scope the secret lookup to the conduit namespace
	nsclient := client.NewNamespacedClient(cl, c.Namespace)

	user, err := valueOrSecret(ctx, nsclient, c.Spec.Registry.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve registry username: %w", err)
	}

	password, err := valueOrSecret(ctx, nsclient, c.Spec.Registry.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve registry username: %w", err)
	}

	if password != "" {
		u.User = url.UserPassword(user, password)
	}

	data[schemaRegistryTypeEnvVar] = []byte(conduit.SchemaRegistryTypeConfluent)
	data[schemaRegistryConnStringEnvVar] = []byte(u.String())

	return data, nil
}

// PipelineConfigYAML produces a conduit pipeline configuration in YAML.
// Invalid configuration will result in a marshalling error.
func PipelineConfigYAML(ctx context.Context, client client.Client, conduit *v1alpha.Conduit) (string, error) {
	var (
		spec           = conduit.Spec
		pipelineStatus = "stopped"
	)

	if *conduit.Spec.Running {
		pipelineStatus = "running"
	}

	pipelineConfig := cyaml.Configuration{
		Version: pipelineConfigVersion,
		Pipelines: []cyaml.Pipeline{
			{
				ID:          spec.ID,
				Name:        spec.Name,
				Description: spec.Description,
				Status:      pipelineStatus,
			},
		},
	}

	var connectors []cyaml.Connector
	for _, cc := range spec.Connectors {
		cfg, err := connectorConfig(ctx, client, cc)
		if err != nil {
			return "", fmt.Errorf("failed building connector: %w", err)
		}
		connectors = append(connectors, cfg)
	}
	pipelineConfig.Pipelines[0].Connectors = connectors

	var processors []cyaml.Processor
	for _, p := range spec.Processors {
		procCfg, err := processorConfig(ctx, client, p)
		if err != nil {
			return "", fmt.Errorf("failed building processor: %w", err)
		}
		processors = append(processors, procCfg)
	}
	pipelineConfig.Pipelines[0].Processors = processors

	b, err := yaml.Marshal(pipelineConfig)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// EnvVars returns a slice of EnvVar with all connector settings.
// Only secrets are put into environment variables.
func EnvVars(c *v1alpha.Conduit) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for _, cc := range c.Spec.Connectors {
		for _, v := range cc.Settings {
			if v.SecretRef == nil {
				continue
			}

			envVars = append(envVars, corev1.EnvVar{
				Name: envVarName(v.SecretRef.Key),
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: v.SecretRef,
				},
			})
		}
	}

	return envVars
}

// envVarsFromSecret maps each key in a Secret to an EnvVar.
func envVarsFromSecret(secret *corev1.Secret) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if len(secret.Data) == 0 {
		return envVars
	}

	for k := range secret.Data {
		envVars = append(envVars, corev1.EnvVar{
			Name: k,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
					Key: k,
				},
			},
		})
	}

	return envVars
}

// settingsWithEnvVars converts settings to a map, every secret key
// reference is converted to env var reference.
func settingsWithEnvVars(ctx context.Context, client client.Client, s []v1alpha.SettingsVar) (map[string]string, error) {
	settings := make(map[string]string)
	for _, v := range s {
		switch {
		case v.SecretRef != nil && v.Value == "":
			settings[v.Name] = fmt.Sprintf("${%s}", envVarName(v.SecretRef.Key))
		case v.ConfigMapRef != nil:
			val, err := configMapValue(ctx, client, v.ConfigMapRef)
			if err != nil {
				return nil, fmt.Errorf("failed getting value for %v", v.ConfigMapRef)
			}
			settings[v.Name] = val
		default:
			settings[v.Name] = v.Value
		}
	}

	return settings, nil
}

func configMapValue(ctx context.Context, cl client.Client, ref *v1alpha.GlobalConfigMapRef) (string, error) {
	configMap := &corev1.ConfigMap{}

	// Fetch the ConfigMap using the client's Get method
	err := cl.Get(
		ctx,
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		configMap,
	)
	if err != nil {
		return "", fmt.Errorf("error fetching configmap %v/%v: %w", ref.Namespace, ref.Name, err)
	}

	val, ok := configMap.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("configmap %v/%v has no key %v", ref.Namespace, ref.Name, ref.Key)
	}

	return val, nil
}

// envVarName returns a catapilized copy of s with all special characters
// replaced with an underscore.
func envVarName(s string) string {
	re := regexp.MustCompile("[^[:alnum:]_]")
	return re.ReplaceAllString(strings.ToUpper(s), "_")
}

// connectorConfig returns a conduit connector config which is created
// from a connector resource settings. Settings which refer to a secret are
// converted to env variables.
func connectorConfig(ctx context.Context, cl client.Client, c *v1alpha.ConduitConnector) (cyaml.Connector, error) {
	var processors []cyaml.Processor
	for _, p := range c.Processors {
		procCfg, err := processorConfig(ctx, cl, p)
		if err != nil {
			return cyaml.Connector{}, fmt.Errorf("failed building processor: %w", err)
		}
		processors = append(processors, procCfg)
	}

	settings, err := settingsWithEnvVars(ctx, cl, c.Settings)
	if err != nil {
		if err != nil {
			return cyaml.Connector{}, fmt.Errorf("failed getting settings for connector %v: %w", c.Name, err)
		}
	}

	return cyaml.Connector{
		ID:         c.ID,
		Name:       c.Name,
		Plugin:     c.PluginName,
		Type:       c.Type,
		Settings:   settings,
		Processors: processors,
	}, nil
}

func processorConfig(ctx context.Context, cl client.Client, p *v1alpha.ConduitProcessor) (cyaml.Processor, error) {
	settings, err := settingsWithEnvVars(ctx, cl, p.Settings)
	if err != nil {
		return cyaml.Processor{}, fmt.Errorf("failed getting settings for processor %v: %w", p.Name, err)
	}

	return cyaml.Processor{
		ID:        p.ID,
		Plugin:    p.Plugin,
		Condition: p.Condition,
		Workers:   p.Workers,
		Settings:  settings,
	}, nil
}

func valueOrSecret(ctx context.Context, cl client.Client, v v1alpha.SettingsVar) (string, error) {
	if v.SecretRef == nil {
		return v.Value, nil
	}

	var secret corev1.Secret

	if err := cl.Get(
		ctx,
		client.ObjectKey{
			Name: v.SecretRef.Name,
		},
		&secret,
	); err != nil {
		return "", fmt.Errorf("failed to get %q secret: %w", v.SecretRef.Name, err)
	}

	if val, ok := secret.Data[v.SecretRef.Key]; ok {
		return string(val), nil
	}

	return "", nil
}
