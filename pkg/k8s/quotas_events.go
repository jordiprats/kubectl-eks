package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetResourceQuotas(ctx context.Context, namespace string) ([]corev1.ResourceQuota, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var quotas []corev1.ResourceQuota

	if namespace == "" {
		// Get quotas from all namespaces
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaceList.Items {
			quotaList, err := clientset.CoreV1().ResourceQuotas(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			quotas = append(quotas, quotaList.Items...)
		}
	} else {
		// Get quotas from specific namespace
		quotaList, err := clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		quotas = quotaList.Items
	}

	return quotas, nil
}

func GetEvents(ctx context.Context, namespace string) ([]corev1.Event, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var events []corev1.Event

	if namespace == "" {
		// Get events from all namespaces
		eventList, err := clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		events = eventList.Items
	} else {
		// Get events from specific namespace
		eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		events = eventList.Items
	}

	return events, nil
}

func GetResourceQuotasWithConfig(ctx context.Context, restConfig *rest.Config, namespace string) ([]corev1.ResourceQuota, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var quotas []corev1.ResourceQuota

	if namespace == "" {
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaceList.Items {
			quotaList, err := clientset.CoreV1().ResourceQuotas(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			quotas = append(quotas, quotaList.Items...)
		}
	} else {
		quotaList, err := clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		quotas = quotaList.Items
	}

	return quotas, nil
}

func GetEventsWithConfig(ctx context.Context, restConfig *rest.Config, namespace string) ([]corev1.Event, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var events []corev1.Event

	if namespace == "" {
		eventList, err := clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		events = eventList.Items
	} else {
		eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		events = eventList.Items
	}

	return events, nil
}
