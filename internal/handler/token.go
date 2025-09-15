package handler

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	k8sClientSet "artifactory-secrets-rotator/internal/client"
	operations "artifactory-secrets-rotator/internal/operations"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/smithy-go/ptr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const tokenEndpoint = "/access/api/v1/aws/token"

// HandlingToken Get JFrog access token
func HandlingToken(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, recorder record.EventRecorder, k8sClient client.Client) error {
	logger := log.FromContext(ctx)

	if tokenDetails.Token != "" {
		logger.Info("Token already defined. skipping artifactory token creation")
		return nil
	}

	// Check if the service account name already exists
	if secretRotator.Spec.ServiceAccount.Name == "" {
		secretRotator.Spec.ServiceAccount.Name = tokenDetails.DefaultServiceAccountName
	}

	// Check if the service account namespace already exists
	if secretRotator.Spec.ServiceAccount.Namespace == "" {
		secretRotator.Spec.ServiceAccount.Namespace = tokenDetails.DefaultServiceAccountNamespace
	}

	// get the k8s client this is needed to get the k8s client
	clientset, err := k8sClientSet.GetK8sClient()
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
			fmt.Sprintf("failed to initialise k8s client, error: %s", err.Error()))
		return err
	}

	// Get Service Account details, further we will use the service account to create a token request
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(secretRotator.Spec.ServiceAccount.Namespace).Get(ctx, secretRotator.Spec.ServiceAccount.Name, metav1.GetOptions{})
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
			fmt.Sprintf("failed to get service account %s from %s namespace, error: %s", secretRotator.Spec.ServiceAccount.Name, secretRotator.Spec.ServiceAccount.Namespace, err.Error()))
		return err
	}

	var request *http.Request

	// Check if Pod Identity is enabled - if so, use the clean Pod Identity flow
	if DetectPodIdentity() {
		logger.Info("Pod Identity detected - using simplified Pod Identity flow")

		// For Pod Identity, no role ARN annotation or web identity token needed
		// Just get the signed request directly
		request, err = GetSignedRequestForPodIdentity(ctx, tokenDetails)
		if err != nil {
			recorder.Eventf(secretRotator, "Warning", "PodIdentityError",
				fmt.Sprintf("Error in Pod Identity flow: %s", err.Error()))
			return err
		}
	} else {
		logger.Info("IRSA detected - using traditional IRSA flow")

		// Check if the role arn annotation exists or not, if not, it will be set for reconciliation
		roleARN := serviceAccount.Annotations[operations.RoleARNKey]
		if roleARN == "" {
			logger.Error(err, "Error getting the role ARN from the service account's annotations")
			return &operations.ReconcileError{Message: "role ARN annotation is empty", RetryIn: 1 * time.Minute}
		}

		// Create token request for the target service account
		tokenRequest, err := clientset.CoreV1().ServiceAccounts(secretRotator.Spec.ServiceAccount.Namespace).CreateToken(
			ctx,
			secretRotator.Spec.ServiceAccount.Name,
			&authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{Audiences: []string{operations.AmazonAwsSts}, ExpirationSeconds: ptr.Int64(operations.ServiceAccountExpirationSeconds)}},
			metav1.CreateOptions{},
		)
		if err != nil {
			recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
				fmt.Sprintf("failed to create token for user/service account %s from %s namespace, error: %s", secretRotator.Spec.ServiceAccount.Name, secretRotator.Spec.ServiceAccount.Namespace, err.Error()))
			return err
		}

		// getting signed request headers for AWS STS GetCallerIdentity call and check role max session duration
		// this is needed to get the max session duration for the role ARN
		request, err = GetSignedRequestAndHandleRoleMaxSession(ctx, roleARN, tokenRequest.Status.Token, secretRotator.Spec.ServiceAccount.Name, secretRotator.Spec.ServiceAccount.Namespace, tokenDetails)
		if err != nil {
			recorder.Eventf(secretRotator, "Warning", "TokenGenerationFailure",
				fmt.Sprintf("Error getting signed AWS credentials, error was %s", err.Error()))
			return err
		}
	}

	//getting max aws role session time to be used as artiactory token expiration time
	maxTTL := tokenDetails.RoleMaxSessionDuration
	if secretRotator.Spec.RefreshInterval == nil {
		logger.Info("JFrog access token TTL will use AWS role Max Session Duration", "roleMaxSession", maxTTL)
	}

	// if the maxTTL is not set we will use the default value of 3 hours
	tokenDetails.TTLInSeconds = float64(*maxTTL)
	if secretRotator.Spec.RefreshInterval != nil && tokenDetails.TTLInSeconds < secretRotator.Spec.RefreshInterval.Seconds() {
		// if the token is set to expire before reconciliation runs we will always get into token expire events
		err = errors.New("the token TTL taken from Role max session value, is shorter then reconciliation duration set through operator refreshTime, which is a misconfiguration causing token expire events")
		logger.Error(err, "CRITICAL MISS CONFIGURATION")
		//reflect this mis misconfiguration through the operator events
		recorder.Eventf(secretRotator, "Warning", "TokenGenerationFailure",
			fmt.Sprintf("The token TTL taken from Role max session value (%d), is shorter then reconciliation duration set through operator refreshTime (%s), which is a misconfiguration causing token expire events",
				*maxTTL,
				secretRotator.Spec.RefreshInterval))
	}

	logger.Info("Generating artifactory token")
	tokenDetails.Username, tokenDetails.Token, err = createArtifactoryToken(ctx, request, tokenDetails.ArtifactoryUrl, maxTTL, &secretRotator.Spec.Security, secretRotator.Name)
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
			fmt.Sprintf("could not get artifactory Token, notice we might ran into expired tokens if this persists, error was %s", err.Error()))
		return err
	}
	return nil
}

// createArtifactoryToken triggers a call against to retrieve JFrog access token
func createArtifactoryToken(ctx context.Context, request *http.Request, artifactoryUrl string, secretTTL *int32, securityDetails *jfrogv1alpha1.SecurityDetails, secretRotatorName string) (string, string, error) {
	logger := log.FromContext(ctx)
	url := fmt.Sprintf("%s%s%s", "https://", artifactoryUrl, tokenEndpoint)
	requestBody := fmt.Sprintf("%s%d%s", "{\"expires_in\": ", *secretTTL, "}")
	body := []byte(requestBody)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", "", &operations.ReconcileError{Message: "Error constructing artifactory request", Cause: err, RetryIn: 1 * time.Minute}
	}

	// Set headers if needed
	req.Header.Set("Content-Type", "application/json")
	for k, v := range request.Header {
		req.Header.Add(k, v[0])
	}

	// Create a custom HTTP client with TLS configuration
	client, err := createCustomHTTPClient(securityDetails, secretRotatorName)
	if err != nil {
		return "", "", &operations.ReconcileError{Message: "Error in intialising custom HTTP client with TLS configuration", Cause: err, RetryIn: 1 * time.Minute}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", &operations.ReconcileError{Message: "Error sending artifactory create token request", Cause: err, RetryIn: 1 * time.Minute}
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Error(err, "Could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		errMessage := fmt.Sprintf("%s%s%s%d%s", "Error getting artifactory token request, token creation to ", url, " returned ", resp.StatusCode, " response")
		return "", "", &operations.ReconcileError{Message: errMessage, RetryIn: 1 * time.Minute}
	}
	// Read and process the response
	myResponse := &operations.AccessResponse{}
	err = json.NewDecoder(resp.Body).Decode(myResponse)
	if err != nil {
		return "", "", &operations.ReconcileError{Message: "Error reading artifactory response", RetryIn: 1 * time.Minute}
	}
	return myResponse.Username, myResponse.AccessToken, nil
}

// Create a custom HTTP client with TLS configuration
func createCustomHTTPClient(securityDetails *jfrogv1alpha1.SecurityDetails, secretRotatorName string) (*http.Client, error) {

	// Initialising http transport,  enable HTTP/2 support for simple configurations
	tr := &http.Transport{TLSClientConfig: &tls.Config{}}

	// Security is disabled
	if !securityDetails.Enabled {
		return &http.Client{}, nil
	}

	// Check if InsecureSkipVerify is enable or not
	if securityDetails.InsecureSkipVerify {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}, nil
	}

	dirPath := jfrogv1alpha1.CustomCertificatePath + secretRotatorName
	if operations.FileExists(dirPath+jfrogv1alpha1.CertPem) && operations.FileExists(dirPath+jfrogv1alpha1.KeyPem) || operations.FileExists(dirPath+jfrogv1alpha1.TlsCrt) && operations.FileExists(dirPath+jfrogv1alpha1.TlsKey) {
		certPath := jfrogv1alpha1.CertPem
		keyPath := jfrogv1alpha1.KeyPem
		if operations.FileExists(dirPath+jfrogv1alpha1.TlsCrt) && operations.FileExists(dirPath+jfrogv1alpha1.TlsKey) {
			certPath = jfrogv1alpha1.TlsCrt
			keyPath = jfrogv1alpha1.TlsKey
		}
		// Load server certs
		cert, err := tls.LoadX509KeyPair(dirPath+certPath, dirPath+keyPath)
		if err != nil {
			return nil, err
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	if operations.FileExists(dirPath+jfrogv1alpha1.CaPem) || operations.FileExists(dirPath+jfrogv1alpha1.TlsCa) {
		caPath := jfrogv1alpha1.CaPem
		if operations.FileExists(dirPath + jfrogv1alpha1.TlsCa) {
			caPath = jfrogv1alpha1.TlsCa
		}
		// Load CA cert
		caCert, err := os.ReadFile(dirPath + caPath)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tr.TLSClientConfig.RootCAs = caCertPool
	}

	// Setup HTTPS client with cert
	client := &http.Client{
		Transport: tr,
	}

	return client, nil
}
