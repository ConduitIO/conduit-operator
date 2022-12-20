package controllers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/conduitio-labs/conduit-operator/api/v1"
	ctrls "github.com/conduitio-labs/conduit-operator/controllers"
)

func Test_ConduitVolumeClaim(t *testing.T) {
	nn := types.NamespacedName{
		Name:      "pvc-name",
		Namespace: "pvc-ns",
	}
	want := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-name",
			Namespace: "pvc-ns",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	got := ctrls.ConduitVolumeClaim(nn, "10Gi")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("volume claim mismatch (-want +got): %v", diff)
	}
}

func Test_ConduitVolume(t *testing.T) {
	want := corev1.Volume{
		Name: v1.ConduitStorageVolumeMount,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "my-claim-name",
			},
		},
	}

	got := ctrls.ConduitVolume("my-claim-name")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("volume mismatch (-want +got): %v", diff)
	}
}

func Test_ConduitPipelineVol(t *testing.T) {
	mode := int32(0o440)
	want := corev1.Volume{
		Name: v1.ConduitPipelineVolumeMount,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "my-ref-name",
				},
				DefaultMode: &mode,
			},
		},
	}

	got := ctrls.ConduitPipelineVol("my-ref-name")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("volume mismatch (-want +got): %v", diff)
	}
}
