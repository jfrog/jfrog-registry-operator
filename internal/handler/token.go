package handler

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	operations "artifactory-secrets-rotator/internal/operations"
	types "artifactory-secrets-rotator/internal/types"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go/aws"
)

const tokenEndpoint = "/access/api/v1/aws/token"

func CreateArtifacotryToken(ctx context.Context, request *http.Request, artifactoryUrl string, secretTTL *int32) (string, string, error) {
	//secretTTL := secretRotator.Spec.SecretTTL
	logger := log.FromContext(ctx)
	url := fmt.Sprintf("%s%s%s", "https://", artifactoryUrl, tokenEndpoint)
	logger.Info("prepare rt call", "url", url)

	requestBody := fmt.Sprintf("%s%d%s", "{\"expires_in\": ", *secretTTL, "}")
	body := []byte(requestBody)

	logger.Info("rt request body", "requestBody", requestBody)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		logger.Error(err, "Error sending POST request")
		return "", "", err
	}
	// Set headers if needed
	req.Header.Set("Content-Type", "application/json")
	for k, v := range request.Header {
		logger.Info("req.Header", "header", k, "value", v)
		req.Header.Add(k, v[0])
	}

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err, "Error sending artifactory create token request")
		return "", "", err
	}
	defer resp.Body.Close()
	logger.Info("returned StatusCode from Artifactory is ", "statusCode", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMessage := fmt.Sprintf("%s%s%s%d%s", "Error getting artifactory token request, token creation to ", url, " returned ", resp.StatusCode, " response")
		err := errors.New(errMessage)
		logger.Error(err, "Error calling tokens url")
		return "", "", err
	}
	// Read and process the response
	myResponse := &types.AccessResponse{}
	derr := json.NewDecoder(resp.Body).Decode(myResponse)
	if derr != nil {
		logger.Error(derr, "Error reading artifactory response")
		return "", "", derr
	}
	logger.Info("read RT response", "AccessToken=", myResponse.AccessToken)
	return myResponse.Username, myResponse.AccessToken, err
}

func HandlingToken(ctx context.Context, tokenDetails *types.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, recorder record.EventRecorder) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// if this is the first secret we are updating this reconciliation, lets get a new token
	if tokenDetails.Token == "" {
		logger.Info("getting AWS token and role name for generating signed AWS temporary credentials")
		awsRoleName := os.Getenv("AWS_ROLE_ARN")
		awsTokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
		//getting AWS_ROLE_ARN from env
		if awsRoleName == "" {
			err := errors.New("missing aws Role Name")
			logger.Error(err, "AWS AWS_ROLE_ARN was empty")
			recorder.Event(secretRotator, "Warning", "Misconfiguration",
				fmt.Sprintf("missing aws Role Name, error was %s", err.Error()))
			return operations.RetryFailedReconciliation()
		} else {
			logger.Info("AWS Role Name", "roleName", awsRoleName)
		}
		//getting AWS_WEB_IDENTITY_TOKEN_FILE from env
		if awsTokenFile == "" {
			err := errors.New("missing aws identity token file location")
			logger.Error(err, "AWS AWS_WEB_IDENTITY_TOKEN_FILE env variable was empty, this might mean the service account is not annotated with Assumed role, or some other misconfiguration, the Artifactory token will not be rotated")
			recorder.Event(secretRotator, "Warning", "Misconfiguration",
				fmt.Sprintf("missing aws identity token file location, error was %s", err.Error()))
			return operations.RetryFailedReconciliation()
		} else {
			logger.Info("AWS Token identity file is found")
		}
		//getting signed request headers for AWS STS GetCallerIdentity call
		request, err := GetSignedRequest(awsRoleName, awsTokenFile)
		if err != nil {
			logger.Error(err, "Error getting signed AWS credentials")
			recorder.Event(secretRotator, "Warning", "TokenGenerationFailure",
				fmt.Sprintf("Error getting signed AWS credentials, error was %s", err.Error()))
			//@todo faster iteration
			return operations.RetryFailedReconciliation()
		}
		//getting max aws role session time to be used as artiactory token expiration time
		maxTTL, err := GetMaxSession(awsRoleName)
		if err != nil {
			logger.Error(err, "Error getting role max session time, we will use default artifactory token expiration: 3 hours")
			maxTTL = aws.Int32(10800)
		} else {
			logger.Info("JFrog access token TTL will use AWS role Max Session Duration", "Role ARN", awsRoleName, "roleMaxSession", maxTTL)
		}
		tokenDetails.TTLInSeconds = float64(*maxTTL)
		if tokenDetails.TTLInSeconds < tokenDetails.RefreshInt.Seconds() {
			// if the token is set to expire before reconciliation runs we will always get into token expire events
			err = errors.New("The token TTL taken from Role max session value, is shorter then reconciliation duration set through operator refreshTime, which is a misconfiguration causing token expire events")
			logger.Error(err, "CRITICAL MIS CONFIGURATION")
			//reflect this mis misconfiguration through the operator events
			recorder.Event(secretRotator, "Warning", "TokenGenerationFailure",
				fmt.Sprintf("The token TTL taken from Role max session value (%d), is shorter then reconciliation duration set through operator refreshTime (%s), which is a misconfiguration causing token expire events",
					*maxTTL,
					tokenDetails.RefreshInt))
		}
		logger.Info("Generating artifactory token")
		tokenDetails.Username, tokenDetails.Token, err = CreateArtifacotryToken(ctx, request, tokenDetails.ArtifactoryUrl, maxTTL)
		//token, err = r.createArtifacotryToken(ctx, secretRotator)
		if err != nil {
			logger.Error(err, "could not get artifactory Token, notice we might ran into expired tokens if this persists")
			recorder.Event(secretRotator, "Warning", "Misconfiguration",
				fmt.Sprintf("could not get artifactory Token, notice we might ran into expired tokens if this persists, error was %s", err.Error()))

			return operations.RetryFailedReconciliation()
		}
		// @toDo this section is only for debug and should be removed
		// logger.Info("Artifactory token response", "token", tokenDetails.Token)
	}
	return ctrl.Result{}, nil
}
