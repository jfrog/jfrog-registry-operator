
<div align="center">

# JFrog Registry Operator

[![JFrog Registry Operator](config/images/frogbot-intro.png)](#readme)

[![Scanned by JFrog Registry Operator](config/images/frogbot-badge.png)](https://github.com/jfrog/jfrog-registry-operator#readme)
[![Go Report Card](https://goreportcard.com/badge/github.com/jfrog/jfrog-registry-operator)](https://goreportcard.com/report/github.com/jfrog/jfrog-registry-operator)
[![Build status](https://github.com/jfrog/jfrog-registry-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/jfrog/jfrog-registry-operator/actions/workflows/test.yml?branch=master)
[![GitHub issues](https://img.shields.io/github/issues/jfrog/jfrog-registry-operator)](https://github.com/jfrog/jfrog-registry-operator/issues)

</div>

## Setting up JFrog’s AssumeRole Capabilities in AWS

Follow the [official documentation](https://jfrog.com/help/r/jfrog-installation-setup-documentation/passwordless-access-for-amazon-eks) for detailed instructions on detailed information and AWS configuration required to run the JFrog Registry Operator.

The integration of AWS Assume Role and JFrog Access presents a powerful solution that enables AWS Identity and Access Management  (IAM) users to temporarily assume permissions to perform actions in a secure and controlled manner. The solution enhances Kubernetes Secrets Management by automating token rotation, enhancing access controls, and seamlessly integrating JFrog Artifactory into the AWS environment

### AssumeRole JFrog Architecture & Deployment

The following diagram shows the basic architecture of how AssumeRole integrates with JFrog Access to provide enhanced access control:

![image](./config/images/secretrotator.png)

If you are interested in making the move from vulnerable manual secret handling to secure automated secret management, then your journey towards a more secure and seamless containerized future begins here. See how quickly this powerful capability can be deployed by checking out our [step-by-step installation and configuration guide](https://jfrog.com/help/r/jfrog-installation-setup-documentation/passwordless-access-for-amazon-eks).

## Install operator using helm chart - Ignore if you already installed using Setting up JFrog’s AssumeRole Capabilities in AWS

```bash
# Get the latest [Helm release](https://github.com/helm/helm#install) Note: (only V3 is supported)
# before installing JFrog helm charts, you need to add the [JFrog helm repository](https://charts.jfrog.io) to your helm client.
helm repo add jfrog https://charts.jfrog.io

# update the helm repo
helm repo update

# decide on the namespace and kubernetes service account name you will want to create
export SERVICE_ACCOUNT_NAME="<service account name>"

# Support for external service accounts has also been added. Users can now utilize an external service account; for this, follow the multi-user installation details relevant to external service accounts.
# Setting SERVICE_ACCOUNT_NAME and ANNOTATIONS is optional for multi-user installations, available from release version 2.1.x.
export ANNOTATIONS="<Role annotation for service account>" # Example: eks.amazonaws.com/role-arn: arn:aws:iam::000000000000:role/jfrog-operator-role
export NAMESPACE="jfrog-operator"

# install JFrog secret rotator operator
helm upgrade --install secretrotator jfrog/jfrog-registry-operator --set "serviceAccount.name=${SERVICE_ACCOUNT_NAME}" --set serviceAccount.annotations=${ANNOTATIONS}  --namespace  ${NAMESPACE} --create-namespace
```

### For multi-user installations, if multiple service accounts need to be created:
```
# In a multi-user scenario, please create all service accounts using the role ARN as an annotation via the Helm chart. This will also update the ClusterRole to grant the necessary permissions to each specific service account.

# Create a custom-values.yaml file with service account details and then install operator.
exchangedServiceAccounts:
 - name: "sample-service-account"
   namespace: "<NAMESPACE>"
   annotations:
      eks.amazonaws.com/role-arn: < role arn >

helm upgrade --install secretrotator jfrog/jfrog-registry-operator --create-namespace -f custom-values.yaml -n ${NAMESPACE}

Important Note: After this, you can use the service account name and namespace in custom resources. You may install multiple custom resources with different service account details.

Example:
serviceAccount:
  name: "sample-service-account"
  namespace: "<NAMESPACE>"
```

Once operator is in running state, configure `artifactoryUrl`, `refreshTime`, `namespaceSelector`, `serviceAccount`, `generatedSecrets`, and `secretMetadata` in [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/master/charts/jfrog-registry-operator/examples/secretrotator.yaml)

Sample Manifest:

```
apiVersion: apps.jfrog.com/v1alpha1
kind: SecretRotator
metadata:
  labels:
    app.kubernetes.io/name: secretrotators.apps.jfrog.com
    app.kubernetes.io/instance: secretrotator
    app.kubernetes.io/created-by: artifactory-secrets-rotator
  name: secretrotator
spec:
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: jfrog-operator
  generatedSecrets:
    - secretName: token-imagepull-secret
      secretType: docker
    # - secretName: token-generic-secret
    #   secretType: generic
  artifactoryUrl: "artifactory.example.com"
  refreshTime: 30m
  # serviceAccount: # The default name and namespace will be the operator’s service account name and namespace
  #   name: ""
  #   namespace: ""
  secretMetadata:
    annotations:
      annotationKey: annotationValue
    labels:
      labelName: labelValue
  security:
    enabled: false
    secretNamespace:
    ## NOTE: You can provide either a ca.pem or ca.crt. But make sure that key needs to same as ca.crt or ca.pem in secret
    certificateSecretName:
    insecureSkipVerify: false
```
Note: Currently spec.secretName is supported but going forward this will be deprecated soon.

Apply the secretrotator mainfest:

```
kubectl apply -f /charts/jfrog-registry-operator/examples/secretrotator.yaml -n ${NAMESPACE}
```

### Uninstalling JFrog Secret Rotator operator

```shell
# Uninstall the secretrotator using the following command
helm uninstall secretrotator -n ${NAMESPACE}

# Uninstall the secretrotator object (path should be pointing to the secretrotator.yaml)
kubectl delete -f secretrotator.yaml -n ${NAMESPACE}

# Remove the CRD from the cluster
kubectl delete crd secretrotators.apps.jfrog.com
```

### Upgrading JFrog Secret Rotator operator

```shell
# update the helm repo
helm repo update

# To upgrade the Custom Resource Definition (CRD), run the following command:
kubectl apply -f https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/refs/heads/master/config/crd/bases/apps.jfrog.com_secretrotators.yaml

# Uninstall the secretrotator using the following command
helm upgrade --install secretrotator jfrog/jfrog-registry-operator --set "serviceAccount.name=${SERVICE_ACCOUNT_NAME}" --set serviceAccount.annotations=${ANNOTATIONS}  --namespace  ${NAMESPACE} --create-namespace
```

### Check Resources in your cluster

```shell
# For secrets in your namespace
kubectl get secrets -n ${NAMESPACE}

# For operator pod in your namespace
kubectl get po -n ${NAMESPACE}

# For SecretRotator
kubectl get SecretRotator
```

## 🤖 Monitoring operator

Follow [monitoring setup docs](./config/monitoring/).

## 🔥 Reporting issues

Please help us improve Frogbot by [reporting issues](https://github.com/jfrog/jfrog-registry-operator/issues/new/choose) you encounter.

<div id="contributions"></div>

## 💻 Contributions

We welcome pull requests from the community. To help us improve this project, please read our [Contribution](./CONTRIBUTING.md#-guidelines) guide.
