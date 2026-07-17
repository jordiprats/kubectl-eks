package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetServiceAccountsWithIRSA(ctx context.Context, namespace string) ([]corev1.ServiceAccount, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var serviceAccounts []corev1.ServiceAccount

	if namespace == "" {
		// Get service accounts from all namespaces
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaceList.Items {
			saList, err := clientset.CoreV1().ServiceAccounts(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, sa := range saList.Items {
				if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
					serviceAccounts = append(serviceAccounts, sa)
				}
			}
		}
	} else {
		// Get service accounts from specific namespace
		saList, err := clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, sa := range saList.Items {
			if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
				serviceAccounts = append(serviceAccounts, sa)
			}
		}
	}

	return serviceAccounts, nil
}

func GetServiceAccountsWithIRSAWithConfig(ctx context.Context, restConfig *rest.Config, namespace string) ([]corev1.ServiceAccount, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var serviceAccounts []corev1.ServiceAccount

	if namespace == "" {
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaceList.Items {
			saList, err := clientset.CoreV1().ServiceAccounts(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, sa := range saList.Items {
				if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
					serviceAccounts = append(serviceAccounts, sa)
				}
			}
		}
	} else {
		saList, err := clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, sa := range saList.Items {
			if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
				serviceAccounts = append(serviceAccounts, sa)
			}
		}
	}

	return serviceAccounts, nil
}

func GetPodsWithKube2IAM(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var pods []corev1.Pod

	if namespace == "" {
		// Get pods from all namespaces
		podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, pod := range podList.Items {
			if role, ok := pod.Annotations["iam.amazonaws.com/role"]; ok && role != "" {
				pods = append(pods, pod)
			}
		}
	} else {
		// Get pods from specific namespace
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, pod := range podList.Items {
			if role, ok := pod.Annotations["iam.amazonaws.com/role"]; ok && role != "" {
				pods = append(pods, pod)
			}
		}
	}

	return pods, nil
}

func GetPodsWithKube2IAMWithConfig(ctx context.Context, restConfig *rest.Config, namespace string) ([]corev1.Pod, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var pods []corev1.Pod

	if namespace == "" {
		// Get pods from all namespaces
		podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, pod := range podList.Items {
			if role, ok := pod.Annotations["iam.amazonaws.com/role"]; ok && role != "" {
				pods = append(pods, pod)
			}
		}
	} else {
		// Get pods from specific namespace
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, pod := range podList.Items {
			if role, ok := pod.Annotations["iam.amazonaws.com/role"]; ok && role != "" {
				pods = append(pods, pod)
			}
		}
	}

	return pods, nil
}

func GetServiceAccountsWithPodIdentity(ctx context.Context, namespace string) ([]corev1.ServiceAccount, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var serviceAccounts []corev1.ServiceAccount

	if namespace == "" {
		// Get service accounts from all namespaces
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaceList.Items {
			saList, err := clientset.CoreV1().ServiceAccounts(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, sa := range saList.Items {
				// Check for any IAM-related annotations/labels
				hasIAM := false

				// IRSA annotation
				if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
					hasIAM = true
				}
				// EKS Pod Identity annotations
				if _, ok := sa.Annotations["eks.amazonaws.com/service-account-role-arn"]; ok {
					hasIAM = true
				}
				if _, ok := sa.Annotations["eks.amazonaws.com/pod-identity-association"]; ok {
					hasIAM = true
				}
				// EKS Pod Identity labels
				if _, ok := sa.Labels["eks.amazonaws.com/pod-identity-association"]; ok {
					hasIAM = true
				}
				// Legacy IAM annotation
				if _, ok := sa.Annotations["iam.amazonaws.com/role"]; ok {
					hasIAM = true
				}

				if hasIAM {
					serviceAccounts = append(serviceAccounts, sa)
				}
			}
		}
	} else {
		// Get service accounts from specific namespace
		saList, err := clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, sa := range saList.Items {
			// Check for any IAM-related annotations/labels
			hasIAM := false

			// IRSA annotation
			if _, ok := sa.Annotations["eks.amazonaws.com/role-arn"]; ok {
				hasIAM = true
			}
			// EKS Pod Identity annotations
			if _, ok := sa.Annotations["eks.amazonaws.com/service-account-role-arn"]; ok {
				hasIAM = true
			}
			if _, ok := sa.Annotations["eks.amazonaws.com/pod-identity-association"]; ok {
				hasIAM = true
			}
			// EKS Pod Identity labels
			if _, ok := sa.Labels["eks.amazonaws.com/pod-identity-association"]; ok {
				hasIAM = true
			}
			// Legacy IAM annotation
			if _, ok := sa.Annotations["iam.amazonaws.com/role"]; ok {
				hasIAM = true
			}

			if hasIAM {
				serviceAccounts = append(serviceAccounts, sa)
			}
		}
	}

	return serviceAccounts, nil
}
