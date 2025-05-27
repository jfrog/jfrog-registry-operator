package client

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Clientset contains the clientSets
type Clientset struct {
	KubeClient *kubernetes.Clientset
}

// GetClientSet will generation both ClientSets (k8s, and jfrog)
func GetK8sClient() (*kubernetes.Clientset, error) {
	clientSets := &Clientset{KubeClient: nil}
	config := ctrl.GetConfigOrDie()

	k8sClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Errorf("unable to create kubernetes clientSet, error: %v: ", err)
	}

	clientSets.KubeClient = k8sClientSet

	return clientSets.KubeClient, nil
}
