package handler

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	operations "artifactory-secrets-rotator/internal/operations"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const tokenEndpoint = "/access/api/v1/aws/token"

// HandlingToken Get JFrog access token
func HandlingToken(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, recorder record.EventRecorder) error {
	logger := log.FromContext(ctx)

	if tokenDetails.Token != "" {
		logger.Info("Token already defined. skipping artifactory token creation")
	}
	awsRoleName := os.Getenv("AWS_ROLE_ARN")
	awsTokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	//getting AWS_ROLE_ARN from env
	if awsRoleName == "" {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration", "missing aws Role Name")
		return &operations.ReconcileError{Message: "AWS_ROLE_ARN is empty", RetryIn: 1 * time.Minute}
	}
	//getting AWS_WEB_IDENTITY_TOKEN_FILE from env
	if awsTokenFile == "" {
		recorder.Event(secretRotator, "Warning", "Misconfiguration", "missing aws identity token file location")
		return &operations.ReconcileError{Message: "AWS_WEB_IDENTITY_TOKEN_FILE env variable was empty, this might mean the service account is not annotated with Assumed role, or some other misconfiguration, the Artifactory token will not be rotated", RetryIn: 1 * time.Minute}
	}
	//getting signed request headers for AWS STS GetCallerIdentity call
	request, err := GetSignedRequest(ctx, awsRoleName, awsTokenFile)
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "TokenGenerationFailure",
			fmt.Sprintf("Error getting signed AWS credentials, error was %s", err.Error()))
		return err
	}
	//getting max aws role session time to be used as artiactory token expiration time
	maxTTL, err := GetMaxSession(ctx, awsRoleName)
	if err != nil {
		logger.Error(err, "Error getting role max session time, we will use default artifactory token expiration: 3 hours")
		maxTTL = aws.Int32(14400)
	} else if secretRotator.Spec.RefreshInterval == nil {
		logger.Info("JFrog access token TTL will use AWS role Max Session Duration", "role", awsRoleName, "roleMaxSession", maxTTL)
	}
	tokenDetails.TTLInSeconds = float64(*maxTTL)
	if secretRotator.Spec.RefreshInterval != nil && tokenDetails.TTLInSeconds < secretRotator.Spec.RefreshInterval.Seconds() {
		// if the token is set to expire before reconciliation runs we will always get into token expire events
		err = errors.New("the token TTL taken from Role max session value, is shorter then reconciliation duration set through operator refreshTime, which is a misconfiguration causing token expire events")
		logger.Error(err, "CRITICAL MIS CONFIGURATION")
		//reflect this mis misconfiguration through the operator events
		recorder.Eventf(secretRotator, "Warning", "TokenGenerationFailure",
			fmt.Sprintf("The token TTL taken from Role max session value (%d), is shorter then reconciliation duration set through operator refreshTime (%s), which is a misconfiguration causing token expire events",
				*maxTTL,
				secretRotator.Spec.RefreshInterval))
	}
	logger.Info("Generating artifactory token")
	tokenDetails.Username, tokenDetails.Token, err = createArtifactoryToken(ctx, request, tokenDetails.ArtifactoryUrl, maxTTL)
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
			fmt.Sprintf("could not get artifactory Token, notice we might ran into expired tokens if this persists, error was %s", err.Error()))
		return err
	}
	return nil
}

// createArtifactoryToken triggers a call against to retrieve JFrog access token
func createArtifactoryToken(ctx context.Context, request *http.Request, artifactoryUrl string, secretTTL *int32) (string, string, error) {
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

	// Send the request
	client := &http.Client{}
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
