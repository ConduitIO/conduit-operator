package controllers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/conduitio/conduit-operator/controllers/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/matryer/is"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	ctrls "github.com/conduitio/conduit-operator/controllers"
)

func Test_SchemaRegistryConfig(t *testing.T) {
	ctx := context.Background()
	internalErr := errors.New("boom error")

	tests := []struct {
		name    string
		conduit *v1alpha.Conduit
		client  func(t *testing.T) kclient.Client
		want    map[string][]byte
		wantErr error
	}{
		{
			name: "config with values",
			client: func(t *testing.T) kclient.Client {
				return mock.NewMockClient(gomock.NewController(t))
			},
			conduit: sampleConduitWithRegistry(false),
			want: map[string][]byte{
				"CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING": []byte("http://localhost:9091/v1"),
				"CONDUIT_SCHEMA_REGISTRY_TYPE":                        []byte("confluent"),
			},
		},
		{
			name: "override url with provided password",
			client: func(t *testing.T) kclient.Client {
				return mock.NewMockClient(gomock.NewController(t))
			},
			conduit: func() *v1alpha.Conduit {
				c := sampleConduitWithRegistry(false)
				c.Spec.Registry.URL = "http://foo:baz@localhost:9091/v1"
				c.Spec.Registry.Username = v1alpha.SettingsVar{Value: "foo"}
				c.Spec.Registry.Password = v1alpha.SettingsVar{Value: "bar"}
				return c
			}(),
			want: map[string][]byte{
				"CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING": []byte("http://foo:bar@localhost:9091/v1"),
				"CONDUIT_SCHEMA_REGISTRY_TYPE":                        []byte("confluent"),
			},
		},
		{
			name: "config with secret values",
			client: func(t *testing.T) kclient.Client {
				nn := types.NamespacedName{
					Name:      "conduit-secret",
					Namespace: "sample",
				}
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(gomock.NewController(t))
				client.EXPECT().IsObjectNamespaced(&corev1.Secret{}).
					DoAndReturn(func(s *corev1.Secret) (bool, error) {
						s.Namespace = "sample"
						s.Name = "conduit-secret"
						return true, nil
					}).Times(2)
				client.EXPECT().Get(ctx, nn, mock.NewSecretMatcher(secret)).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...kclient.GetOption) error {
						s.Data = map[string][]byte{
							"username": []byte("ping"),
							"password": []byte("pong"),
						}
						return nil
					}).Times(2)

				return client
			},
			conduit: func() *v1alpha.Conduit {
				c := sampleConduitWithRegistry(false)
				c.Spec.Registry.URL = "http://localhost:9091/v1"
				c.Spec.Registry.Username = v1alpha.SettingsVar{
					SecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "conduit-secret",
						},
						Key: "username",
					},
				}
				c.Spec.Registry.Password = v1alpha.SettingsVar{
					SecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "conduit-secret",
						},
						Key: "password",
					},
				}
				return c
			}(),
			want: map[string][]byte{
				"CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING": []byte("http://ping:pong@localhost:9091/v1"),
				"CONDUIT_SCHEMA_REGISTRY_TYPE":                        []byte("confluent"),
			},
		},
		{
			name: "error when parsing schema url",
			client: func(t *testing.T) kclient.Client {
				return mock.NewMockClient(gomock.NewController(t))
			},
			conduit: func() *v1alpha.Conduit {
				c := sampleConduitWithRegistry(false)
				c.Spec.Registry.URL = "$something%"
				return c
			}(),
			wantErr: errors.New(`failed to parse registry URL: parse "$something%": invalid URL escape "%"`),
		},
		{
			name: "error when getting secret",
			client: func(t *testing.T) kclient.Client {
				client := mock.NewMockClient(gomock.NewController(t))
				client.EXPECT().IsObjectNamespaced(&corev1.Secret{}).Return(false, internalErr)

				return client
			},
			conduit: func() *v1alpha.Conduit {
				c := sampleConduitWithRegistry(false)
				c.Spec.Registry.URL = "http://localhost:9091/v1"
				c.Spec.Registry.Username = v1alpha.SettingsVar{
					SecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "conduit-secret",
						},
						Key: "username",
					},
				}
				return c
			}(),
			wantErr: errors.New(`failed to resolve registry username: failed to get "conduit-secret" secret: error finding the scope of the object: boom error`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			got, err := ctrls.SchemaRegistryConfig(ctx, tc.client(t), tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
				is.Equal("", cmp.Diff(tc.want, got))
			}
		})
	}
}

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
		conduit *v1alpha.Conduit
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
