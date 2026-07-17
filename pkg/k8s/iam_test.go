package k8s

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestGetPodsWithKube2IAMWithConfig_FiltersAnnotation(t *testing.T) {
	// Start a fake API server that returns a pod list with mixed annotations
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/pods" {
			podList := corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-with-kube2iam",
							Namespace: "default",
							Annotations: map[string]string{
								"iam.amazonaws.com/role": "arn:aws:iam::123456789012:role/test-role",
							},
						},
						Spec: corev1.PodSpec{
							NodeName: "node-1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-without-annotation",
							Namespace: "default",
							Annotations: map[string]string{
								"some-other-annotation": "value",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-with-empty-annotation",
							Namespace: "default",
							Annotations: map[string]string{
								"iam.amazonaws.com/role": "",
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(podList)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	// Build rest.Config pointing to the fake server
	restConfig := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	// Call GetPodsWithKube2IAMWithConfig with all namespaces
	pods, err := GetPodsWithKube2IAMWithConfig(context.Background(), restConfig, "")
	require.NoError(t, err)
	require.Len(t, pods, 1, "should only return 1 pod with kube2iam annotation")
	assert.Equal(t, "pod-with-kube2iam", pods[0].Name)
	assert.Equal(t, "default", pods[0].Namespace)
	assert.Equal(t, "arn:aws:iam::123456789012:role/test-role", pods[0].Annotations["iam.amazonaws.com/role"])
}

func TestGetPodsWithKube2IAMWithConfig_SpecificNamespace(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/production/pods" {
			podList := corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "prod-pod",
							Namespace: "production",
							Annotations: map[string]string{
								"iam.amazonaws.com/role": "arn:aws:iam::123456789012:role/prod-role",
							},
						},
						Spec: corev1.PodSpec{
							NodeName: "node-2",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(podList)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	restConfig := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	pods, err := GetPodsWithKube2IAMWithConfig(context.Background(), restConfig, "production")
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "prod-pod", pods[0].Name)
}

func TestGetPodsWithKube2IAMWithConfig_NoAnnotationPods(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/pods" {
			podList := corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-a",
							Namespace: "default",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-b",
							Namespace: "default",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(podList)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	restConfig := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	pods, err := GetPodsWithKube2IAMWithConfig(context.Background(), restConfig, "")
	require.NoError(t, err)
	assert.Empty(t, pods, "no pods should be returned since none have kube2iam annotations")
}

func TestGetPodsWithKube2IAMWithConfig_APIServerError(t *testing.T) {
	// Use a server that returns 500
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	restConfig := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	_, err := GetPodsWithKube2IAMWithConfig(context.Background(), restConfig, "")
	assert.Error(t, err, "should return error when API server fails")
}
