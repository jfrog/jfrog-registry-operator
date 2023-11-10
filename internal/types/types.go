package types

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// AccessResponse
type AccessResponse struct {
	TokenId     string `json:"token_id"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Username    string `json:"username"`
}

// TokenDetails holding resorce object token details
type TokenDetails struct {
	Username              string
	Token                 string
	ArtifactoryUrl        string
	SecretName            string
	NamespaceSelector     labels.Selector
	RequeueInterval       time.Duration
	NamespaceList         v1.NamespaceList
	RefreshInt            time.Duration
	FailedNamespaces      map[string]error
	ProvisionedNamespaces []string
	TTLInSeconds          float64
}

const SecretRotatorFinalizer = "apps.jfrog.com/finalizer"
const (
	// typeAvailableMemcached represents the status of the Deployment reconciliation
	TypeAvailableSecretRotator = "Available"
	// typeDegradedMemcached represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	TypeDegradedSecretRotator = "Degraded"
)
