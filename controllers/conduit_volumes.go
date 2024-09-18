package controllers

import (
	v1 "github.com/conduitio/conduit-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConduitVolumeClaim returns pvc defintion of specified size and name.
func ConduitVolumeClaim(nn types.NamespacedName, size string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}

// ConduitVolume returns a kubernetes volume definition for the provided claim
func ConduitVolume(claimName string) corev1.Volume {
	return corev1.Volume{
		Name: v1.ConduitStorageVolumeMount,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		},
	}
}

// ConduitVolume returns a kubernetes configmap volume definition for the provided reference.
func ConduitPipelineVol(refName string) corev1.Volume {
	defaultMode := int32(0o440) // octal, user/group readable
	return corev1.Volume{
		Name: v1.ConduitPipelineVolumeMount,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: refName,
				},
				DefaultMode: &defaultMode,
			},
		},
	}
}
