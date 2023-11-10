package resource

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"sort"
)

// ToNamespaceFailures iterates through failed namespaces and returns a list with failure reason
func ToNamespaceFailures(failedNamespaces map[string]error) []jfrogv1alpha1.SecretNamespaceFailure {
	namespaceFailures := make([]jfrogv1alpha1.SecretNamespaceFailure, len(failedNamespaces))

	i := 0
	for namespace, err := range failedNamespaces {
		namespaceFailures[i] = jfrogv1alpha1.SecretNamespaceFailure{
			Namespace: namespace,
			Reason:    err.Error(),
		}
		i++
	}
	sort.Slice(namespaceFailures, func(i, j int) bool { return namespaceFailures[i].Namespace < namespaceFailures[j].Namespace })
	return namespaceFailures
}
