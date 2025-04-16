package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	logr "github.com/go-logr/logr/testing"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"

	runtimectrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/controller/mock"
)

var (
	errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "not found")
	errInternal = apierrors.NewInternalError(errors.New("boom"))
)

func Test_Reconciler(t *testing.T) {
	var (
		ctx = context.Background()
		is  = is.New(t)
	)

	tests := []struct {
		name    string
		conduit *v1alpha.Conduit
		setup   func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		result  runtimectrl.Result
		wantErr error
	}{
		{
			name:    "add finalizers",
			conduit: sampleConduit(t, true),
			result: runtimectrl.Result{
				RequeueAfter: 10 * time.Second,
			},
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, cc *v1alpha.Conduit, _ ...kclient.CreateOption) error {
						*cc = *c
						return nil
					})

				withFinalizers := c.DeepCopy()
				ctrlutil.AddFinalizer(withFinalizers, v1alpha.ConduitFinalizer)
				client.EXPECT().Update(ctx, withFinalizers)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: mock.NewMockEventRecorder(ctrl),
					Logger:        logr.NewTestLogger(t),
				}
			},
		},
		{
			name:    "remove finalizers when deleted",
			conduit: sampleConduit(t, true),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				now := metav1.Now()
				deletedConduit := c.DeepCopy()
				deletedConduit.ObjectMeta.DeletionTimestamp = &now
				ctrlutil.AddFinalizer(deletedConduit, v1alpha.ConduitFinalizer)

				client := mock.NewMockClient(ctrl)
				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, cc *v1alpha.Conduit, _ ...kclient.CreateOption) error {
						*cc = *deletedConduit
						return nil
					})

				withoutFinalizers := deletedConduit.DeepCopy()
				ctrlutil.RemoveFinalizer(withoutFinalizers, v1alpha.ConduitFinalizer)
				client.EXPECT().Update(ctx, withoutFinalizers)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().
					Eventf(withoutFinalizers, corev1.EventTypeNormal, v1alpha.DeletedReason, gomock.Any(), gomock.Any(), gomock.Any())

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
					Logger:        logr.NewTestLogger(t),
				}
			},
		},
		{
			name:    "error when fetching conduit",
			conduit: sampleConduit(t, true),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					Return(errInternal)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: mock.NewMockEventRecorder(ctrl),
					Logger:        logr.NewTestLogger(t),
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)
			req := runtimectrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tc.conduit.Name,
					Namespace: tc.conduit.Namespace,
				},
			}

			res, err := r.Reconcile(ctx, req)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}
			is.Equal(res, tc.result)
		})
	}
}

func Test_CreateOrUpdateConfig(t *testing.T) {
	var (
		ctx           = context.Background()
		conduitScheme = conduitScheme(t)
		is            = is.New(t)
	)

	defaultConditions := func() *v1alpha.ConduitStatus {
		s := &v1alpha.ConduitStatus{}
		s.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionUnknown, "", "")

		return s
	}

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "creates config map",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}
				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().
					Get(ctx, nn, mock.NewConfigMapMatcher(cm)).
					Return(errNotFound)

				cmCreated := cm.DeepCopy()
				cmCreated.Data = map[string]string{
					"pipeline.yaml": runningPipelineYAML,
				}
				client.EXPECT().
					Create(ctx, mock.NewConfigMapMatcher(cmCreated)).
					Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.CreatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "updates config map",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}
				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, mock.NewConfigMapMatcher(cm)).Return(nil)

				cmUpdated := cm.DeepCopy()
				cmUpdated.Data = map[string]string{
					"pipeline.yaml": runningPipelineYAML,
				}
				client.EXPECT().Update(ctx, mock.NewConfigMapMatcher(cmUpdated)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "error while updating config map",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				err := errInternal
				nn := c.NamespacedName()
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}
				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, cm).Return(err)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, gomock.Any())

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.CreateOrUpdateConfig(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}

			cmpStatusConditions(t,
				tc.wantStatus.Conditions,
				tc.conduit.Status.Conditions,
			)
		})
	}
}

func Test_CreateOrUpdateSecret(t *testing.T) {
	var (
		ctx           = context.Background()
		conduitScheme = conduitScheme(t)
		is            = is.New(t)
	)

	defaultConditions := func() *v1alpha.ConduitStatus {
		s := &v1alpha.ConduitStatus{}
		s.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionUnknown, "", "")

		return s
	}

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "creates secret",
			conduit: sampleConduitWithRegistry(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().
					Get(ctx, nn, mock.NewSecretMatcher(secret)).
					Return(errNotFound)

				secretCreated := secret.DeepCopy()
				secretCreated.Data = map[string][]byte{
					"CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING": []byte("http://localhost:9091/v1"),
					"CONDUIT_SCHEMA_REGISTRY_TYPE":                        []byte("confluent"),
				}

				client.EXPECT().
					Create(ctx, mock.NewSecretMatcher(secretCreated)).
					Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.CreatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "updates secret",
			conduit: sampleConduitWithRegistry(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, mock.NewSecretMatcher(secret)).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...kclient.GetOption) error {
						s.Data = map[string][]byte{
							"CONDUIT_SCHEMA_REGISTRY_TYPE": []byte("builtin"),
						}
						return nil
					})

				secretUpdated := secret.DeepCopy()
				secretUpdated.Data = map[string][]byte{
					"CONDUIT_SCHEMA_REGISTRY_CONFLUENT_CONNECTION_STRING": []byte("http://localhost:9091/v1"),
					"CONDUIT_SCHEMA_REGISTRY_TYPE":                        []byte("confluent"),
				}

				client.EXPECT().Update(ctx, mock.NewSecretMatcher(secretUpdated)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "error while updating secret",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, secret).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, gomock.Any())

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.CreateOrUpdateSecret(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}

			cmpStatusConditions(t,
				tc.wantStatus.Conditions,
				tc.conduit.Status.Conditions,
			)
		})
	}
}

func Test_CreateOrUpdateVolume(t *testing.T) {
	var (
		ctx           = context.Background()
		conduitScheme = conduitScheme(t)
		is            = is.New(t)
	)

	defaultConditions := func() *v1alpha.ConduitStatus {
		s := &v1alpha.ConduitStatus{}
		s.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionUnknown, "", "")

		return s
	}

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "creates volume",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				pvc := ConduitVolumeClaim(nn, "1Gi")

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, pvc).Return(errNotFound)
				client.EXPECT().Create(ctx, mock.NewPvcMatcher(pvc)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.CreatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "updates volume",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				pvc := ConduitVolumeClaim(nn, "1Gi")

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, pvc).Return(nil)
				client.EXPECT().Update(ctx, mock.NewPvcMatcher(pvc)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "updates volume to bound",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				pvc := ConduitVolumeClaim(nn, "1Gi")

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, mock.NewPvcMatcher(pvc)).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, p *corev1.PersistentVolumeClaim, _ ...kclient.CreateOption) error {
						newp := pvc.DeepCopy()
						newp.Status.Phase = corev1.ClaimBound
						*p = *newp
						return nil
					})
				c.Status.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionFalse, "", "")
				client.EXPECT().Update(ctx, mock.NewPvcMatcher(pvc)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.VolBoundReason, gomock.Any(), nn)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "error when updating volume",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				pvc := ConduitVolumeClaim(nn, "1Gi")

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, pvc).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, errInternal)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.CreateOrUpdateVolume(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}

			cmpStatusConditions(t,
				tc.wantStatus.Conditions,
				tc.conduit.Status.Conditions,
			)
		})
	}
}

func Test_CreateOrUpdateService(t *testing.T) {
	var (
		ctx           = context.Background()
		conduitScheme = conduitScheme(t)
		is            = is.New(t)
	)

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "service is created",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := &v1alpha.ConduitStatus{}
				status.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, svc).Return(errNotFound)

				svcCreated := svc.DeepCopy()
				svcCreated.Spec = corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"app.kubernetes.io/name": nn.Name,
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							TargetPort: intstr.IntOrString{
								IntVal: 8080,
							},
						},
					},
				}
				client.EXPECT().Create(ctx, mock.NewServiceMatcher(svcCreated)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.CreatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "service is updated",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := &v1alpha.ConduitStatus{}
				status.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, svc).Return(nil)

				svcCreated := svc.DeepCopy()
				svcCreated.Spec = corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"app.kubernetes.io/name": nn.Name,
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							TargetPort: intstr.IntOrString{
								IntVal: 8080,
							},
						},
					},
				}
				client.EXPECT().Update(ctx, mock.NewServiceMatcher(svcCreated)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "error creating or updating service",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := &v1alpha.ConduitStatus{}
				status.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, svc).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, errInternal)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.CreateOrUpdateService(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}
		})
	}
}

func Test_CreateOrUpdateDeployment(t *testing.T) {
	var (
		ctx           = context.Background()
		conduitScheme = conduitScheme(t)
		is            = is.New(t)
		resourceVer   = "resource-version-121"
	)

	defaultConditions := func() *v1alpha.ConduitStatus {
		s := &v1alpha.ConduitStatus{}
		s.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitVolumeReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitConfigReady, corev1.ConditionUnknown, "", "")
		s.SetCondition(v1alpha.ConditionConduitSecretReady, corev1.ConditionUnknown, "", "")

		return s
	}

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "deployment is created",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)

				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.GetOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...kclient.GetOption) error {
						s.Name = nn.Name
						s.Data = map[string][]byte{
							"key1": []byte("data1"),
							"key2": []byte("data2"),
						}
						return nil
					})

				client.EXPECT().Get(ctx, nn, deployment).Return(errNotFound)

				createdDeployment := deployment.DeepCopy()
				createdDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": resourceVer,
				}
				createdDeployment.Spec.Template.Spec.Containers = []corev1.Container{
					{
						Env: []corev1.EnvVar{
							{Name: "SETTING2__AKEY", ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "setting2-secret-name"},
									Key:                  "setting2-#akey",
								},
							}},
							{Name: "SETTING3__S_KEY", ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "setting3-secret-name"},
									Key:                  "setting3-%s-key",
								},
							}},
							{Name: "key1", ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "conduit-server-sample"},
									Key:                  "key1",
								},
							}},
							{Name: "key2", ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "conduit-server-sample"},
									Key:                  "key2",
								},
							}},
						},
					},
				}
				client.EXPECT().Create(ctx, mock.NewDeploymentMatcher(createdDeployment)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.PendingReason, gomock.Any(), nn, gomock.Any())
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.CreatedReason, gomock.Any(), nn)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
		},
		{
			name:    "deployment is updated",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionTrue, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionUnknown, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.CreateOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(nil)

				client.EXPECT().Get(ctx, nn, deployment).
					DoAndReturn(func(_ context.Context, n types.NamespacedName, d *appsv1.Deployment, _ ...kclient.CreateOption) error {
						is.Equal(n, nn)

						d.Status = appsv1.DeploymentStatus{
							Conditions: []appsv1.DeploymentCondition{
								{
									Type:   appsv1.DeploymentAvailable,
									Status: corev1.ConditionTrue,
								},
							},
							Replicas:      1,
							ReadyReplicas: 1,
						}

						return nil
					})

				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": resourceVer,
				}

				client.EXPECT().Update(ctx, mock.NewDeploymentMatcher(updatedDeployment)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.RunningReason, gomock.Any(), nn, gomock.Any())
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
		},
		{
			name:    "deployment is updated but not running",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.CreateOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(nil)

				client.EXPECT().Get(ctx, nn, deployment).
					DoAndReturn(func(_ context.Context, n types.NamespacedName, d *appsv1.Deployment, _ ...kclient.CreateOption) error {
						is.Equal(n, nn)

						d.Status = appsv1.DeploymentStatus{
							Conditions: []appsv1.DeploymentCondition{
								{
									Type:   appsv1.DeploymentAvailable,
									Status: corev1.ConditionFalse,
								},
							},
							Replicas:      1,
							ReadyReplicas: 0,
						}

						return nil
					})

				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": "resource-version-121",
				}
				c.Status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
				client.EXPECT().Update(ctx, mock.NewDeploymentMatcher(updatedDeployment)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.PendingReason, gomock.Any(), nn)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
		},
		{
			name:    "deployment is stopped",
			conduit: sampleConduit(t, false),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.CreateOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(nil)

				client.EXPECT().Get(ctx, nn, deployment).
					DoAndReturn(func(_ context.Context, n types.NamespacedName, d *appsv1.Deployment, _ ...kclient.CreateOption) error {
						is.Equal(n, nn)

						d.Status = appsv1.DeploymentStatus{
							Conditions: []appsv1.DeploymentCondition{
								{
									Type:   appsv1.DeploymentAvailable,
									Status: corev1.ConditionTrue,
								},
							},
						}

						return nil
					})

				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": "resource-version-121",
				}
				client.EXPECT().Update(ctx, mock.NewDeploymentMatcher(updatedDeployment)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.StoppedReason, gomock.Any(), nn)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
		},
		{
			name:    "deployment is degraded",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.CreateOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(nil)

				client.EXPECT().Get(ctx, nn, deployment).
					DoAndReturn(func(_ context.Context, n types.NamespacedName, d *appsv1.Deployment, _ ...kclient.CreateOption) error {
						is.Equal(n, nn)

						d.Status = appsv1.DeploymentStatus{
							Conditions: []appsv1.DeploymentCondition{
								{
									Type:   appsv1.DeploymentAvailable,
									Status: corev1.ConditionTrue,
								},
							},
							Replicas:            1,
							UnavailableReplicas: 1,
						}

						return nil
					})

				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": "resource-version-121",
				}
				client.EXPECT().Update(ctx, mock.NewDeploymentMatcher(updatedDeployment)).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.DegradedReason, gomock.Any(), nn)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, gomock.Any(), nn)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
		},
		{
			name:    "error when getting config map",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionUnknown, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, errInternal)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
		{
			name:    "error when getting secret",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionUnknown, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionUnknown, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).Return(nil)
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, errInternal)

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
		{
			name:    "error when creating or updating deployment",
			conduit: sampleConduit(t, true),
			wantStatus: func() *v1alpha.ConduitStatus {
				status := defaultConditions()
				status.SetCondition(v1alpha.ConditionConduitDeploymentRunning, corev1.ConditionFalse, "", "")
				status.SetCondition(v1alpha.ConditionConduitReady, corev1.ConditionFalse, "", "")

				return status
			}(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				nn := c.NamespacedName()
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
				}

				client := mock.NewMockClient(ctrl)
				client.EXPECT().Scheme().Return(conduitScheme)
				client.EXPECT().Get(ctx, nn, &corev1.ConfigMap{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, c *corev1.ConfigMap, _ ...kclient.CreateOption) error {
						c.ResourceVersion = resourceVer
						return nil
					})
				client.EXPECT().Get(ctx, nn, &corev1.Secret{}).Return(nil)

				client.EXPECT().Get(ctx, nn, deployment).Return(errNotFound)

				createdDeployment := deployment.DeepCopy()
				createdDeployment.Spec.Template.Annotations = map[string]string{
					"operator.conduit.io/config-map-version": "resource-version-121",
				}
				client.EXPECT().Create(ctx, mock.NewDeploymentMatcher(createdDeployment)).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), nn, errInternal)
				recorder.EXPECT().Eventf(c, corev1.EventTypeNormal, v1alpha.PendingReason, gomock.Any(), nn, gomock.Any())

				return NewConduitReconciler(&v1alpha.ConduitInstanceMetadata{}, client, logr.NewTestLogger(t), recorder)
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.CreateOrUpdateDeployment(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}

			cmpStatusConditions(t,
				tc.wantStatus.Conditions,
				tc.conduit.Status.Conditions,
			)
		})
	}
}

func Test_UpdateStatus(t *testing.T) {
	var (
		ctx           = context.Background()
		is            = is.New(t)
		conduitSample = sampleConduit(t, true)
	)

	tests := []struct {
		name       string
		conduit    *v1alpha.Conduit
		setup      func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler
		wantStatus *v1alpha.ConduitStatus
		wantErr    error
	}{
		{
			name:    "status is updated",
			conduit: conduitSample.DeepCopy(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				statusWriter := mock.NewMockStatusWriter(ctrl)

				c.Status = v1alpha.ConduitStatus{}
				c.Status.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionFalse, "", "")

				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					Return(nil)

				client.EXPECT().Status().Return(statusWriter)
				statusWriter.EXPECT().Update(ctx, c).Return(nil)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().
					Event(c, corev1.EventTypeNormal, v1alpha.UpdatedReason, "Status updated")

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
		},
		{
			name:    "status is unchaged",
			conduit: conduitSample.DeepCopy(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, pc *v1alpha.Conduit, _ ...kclient.CreateOption) error {
						pc.Status = c.Status
						return nil
					})

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: mock.NewMockEventRecorder(ctrl),
				}
			},
		},
		{
			name:    "error getting latest conduit",
			conduit: conduitSample.DeepCopy(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					Return(errInternal)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: mock.NewMockEventRecorder(ctrl),
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
		{
			name:    "error updating status",
			conduit: conduitSample.DeepCopy(),
			setup: func(ctrl *gomock.Controller, c *v1alpha.Conduit) *ConduitReconciler {
				client := mock.NewMockClient(ctrl)
				statusWriter := mock.NewMockStatusWriter(ctrl)

				client.EXPECT().
					Get(ctx, kclient.ObjectKeyFromObject(c), &v1alpha.Conduit{}).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, pc *v1alpha.Conduit, _ ...kclient.CreateOption) error {
						pc.Status = v1alpha.ConduitStatus{}
						pc.Status.SetCondition(v1alpha.ConditionConduitServiceReady, corev1.ConditionFalse, "", "")
						return nil
					})
				client.EXPECT().Status().Return(statusWriter)
				statusWriter.EXPECT().Update(ctx, c).Return(errInternal)

				recorder := mock.NewMockEventRecorder(ctrl)
				recorder.EXPECT().Eventf(c, corev1.EventTypeWarning, v1alpha.ErroredReason, gomock.Any(), errInternal)

				return &ConduitReconciler{
					Client:        client,
					EventRecorder: recorder,
				}
			},
			wantErr: errors.New("Internal error occurred: boom"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := tc.setup(ctrl, tc.conduit)

			err := r.UpdateStatus(ctx, tc.conduit)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}
		})
	}
}

func conduitScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	is := is.New(t)
	scheme := runtime.NewScheme()
	is.NoErr(v1alpha.AddToScheme(scheme))
	v1alpha.SchemeBuilder.Register(&v1alpha.Conduit{}, &v1alpha.ConduitList{})

	return scheme
}
