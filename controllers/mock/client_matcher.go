package mock

import (
	"fmt"

	v1 "github.com/conduitio-labs/conduit-operator/api/v1"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type configMapMatcher struct {
	want, got *corev1.ConfigMap
}

func NewConfigMapMatcher(cm *corev1.ConfigMap) gomock.Matcher {
	return &configMapMatcher{want: cm}
}

func (m *configMapMatcher) String() string {
	s := fmt.Sprintf("%+v", m.want)
	if m.got != nil {
		s += fmt.Sprintf("%s\n\tDiff:\n%s\n", s, cmp.Diff(m.want, m.got))
	}

	return s
}

func (m *configMapMatcher) Matches(x interface{}) bool {
	other, ok := x.(*corev1.ConfigMap)
	if !ok {
		return false
	}
	m.got = other

	if m.want.Name != m.got.Name {
		return false
	}
	if m.want.Namespace != m.got.Namespace {
		return false
	}
	if diff := cmp.Diff(m.want.Data, m.got.Data); diff != "" {
		return false
	}
	return true
}

type pvcMatcher struct {
	want, got *corev1.PersistentVolumeClaim
}

func NewPvcMatcher(pvc *corev1.PersistentVolumeClaim) gomock.Matcher {
	return &pvcMatcher{want: pvc}
}

func (m *pvcMatcher) String() string {
	s := fmt.Sprintf("%+v", m.want)
	if m.got != nil {
		s += fmt.Sprintf("%s\n\tDiff:\n%s\n", s, cmp.Diff(m.want, m.got))
	}

	return s
}

func (m *pvcMatcher) Matches(x interface{}) bool {
	other, ok := x.(*corev1.PersistentVolumeClaim)
	if !ok {
		return false
	}
	m.got = other

	if m.want.Name != m.got.Name {
		return false
	}
	if m.want.Namespace != m.got.Namespace {
		return false
	}

	if diff := cmp.Diff(m.want.Spec, m.got.Spec); diff != "" {
		return false
	}
	return true
}

type conduitMatcher struct {
	want, got *v1.Conduit
}

func NewConduitMatcher(c *v1.Conduit) gomock.Matcher {
	return &conduitMatcher{want: c}
}

func (m *conduitMatcher) Matches(x interface{}) bool {
	other, ok := x.(*v1.Conduit)
	if !ok {
		return false
	}
	m.got = other

	if m.want.Name != m.got.Name {
		return false
	}
	if m.want.Namespace != m.got.Namespace {
		return false
	}

	return true
}

func (m *conduitMatcher) String() string {
	s := fmt.Sprintf("%+v", m.want)
	if m.got != nil {
		s += fmt.Sprintf("%s\n\tDiff:\n%s\n", s, cmp.Diff(m.want, m.got))
	}

	return s
}

type serviceMatcher struct {
	want, got *corev1.Service
}

func NewServiceMatcher(s *corev1.Service) gomock.Matcher {
	return &serviceMatcher{want: s}
}

func (m *serviceMatcher) Matches(x interface{}) bool {
	other, ok := x.(*corev1.Service)
	if !ok {
		return false
	}
	m.got = other

	if m.want.Name != m.got.Name {
		return false
	}
	if m.want.Namespace != m.got.Namespace {
		return false
	}
	if diff := cmp.Diff(m.want.Spec, m.got.Spec); diff != "" {
		return false
	}

	return true
}

func (m *serviceMatcher) String() string {
	s := fmt.Sprintf("%+v", m.want)
	if m.got != nil {
		s += fmt.Sprintf("%s\n\tDiff:\n%s\n", s, cmp.Diff(m.want, m.got))
	}

	return s
}

type deploymentMatcher struct {
	want, got *appsv1.Deployment
}

func NewDeploymentMatcher(d *appsv1.Deployment) gomock.Matcher {
	return &deploymentMatcher{want: d}
}

func (m *deploymentMatcher) Matches(x interface{}) bool {
	other, ok := x.(*appsv1.Deployment)
	if !ok {
		return false
	}
	m.got = other

	if m.want.Name != m.got.Name {
		return false
	}
	if m.want.Namespace != m.got.Namespace {
		return false
	}
	if diff := cmp.Diff(
		m.want.Spec.Template.Annotations,
		m.got.Spec.Template.Annotations,
	); diff != "" {
		return false
	}

	return true
}

func (m *deploymentMatcher) String() string {
	s := fmt.Sprintf("%+v", m.want)
	if m.got != nil {
		s += fmt.Sprintf("%s\n\tDiff:\n%s\n", s, cmp.Diff(m.want, m.got))
	}

	return s
}
