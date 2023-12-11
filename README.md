
<div align="center">

# JFrog Registry Operator

[![JFrog Registry Operator](config/images/frogbot-intro.png)](#readme)

[![Scanned by JFrog Registry Operator](config/images/frogbot-badge.png)](https://github.com/jfrog/jfrog-registry-operator#readme)
[![Go Report Card](https://goreportcard.com/badge/github.com/jfrog/jfrog-registry-operator)](https://goreportcard.com/badge/github.com/jfrog/jfrog-registry-operator)
[![Build status](https://github.com/jfrog/jfrog-registry-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/jfrog/jfrog-registry-operator/actions/workflows/test.yml?branch=master)
[![GitHub issues](https://img.shields.io/github/issues/jfrog/jfrog-registry-operator)](https://github.com/jfrog/jfrog-registry-operator/issues)

</div>

### Install operator using helm chart

```bash
# Get the latest [Helm release](https://github.com/helm/helm#install) Note: (only V3 is supported)
# before installing JFrog helm charts, you need to add the [JFrog helm repository](https://charts.jfrog.io) to your helm client.
helm repo add jfrog https://charts.jfrog.io

# update the helm repo
helm repo update

# decide on the namespace and kubernetes service account name you will want to create
export SERVICE_ACCOUNT_NAME="<service account name>" # Default name is jfrog-operator-sa"
export ANNOTATIONS="<Role annotation for service account>" # Example: eks.amazonaws.com/role-arn: arn:aws:iam::000000000000:role/jfrog-operator-role
export NAMESPACE="jfrog-operator"

# install JFrog secret rotator operator
helm upgrade --install secretrotator jfrog/jfrog-registry-operator --set "serviceAccount.name=${SERVICE_ACCOUNT_NAME}" --set serviceAccount.annotations=${ANNOTATIONS}  -n ${NAMESPACE}
```

Once operator is in running state, configure artifactoryUrl, refreshTime, namespaceSelector and secretMetadata in [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/master/charts/jfrog-registry-operator/examples/secretrotator.yaml)

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
  secretName: token-secret
  artifactoryUrl: ""
  refreshTime: 30m
  secretMetadata:
    annotations:
      annotationKey: annotationValue
    labels:
      labelName: labelValue
```

Apply the secretrotator mainfest:

```
kubectl apply -f /charts/jfrog-registry-operator/examples/secretrotator.yaml -n ${NAMESPACE}
```

#### Uninstalling JFrog Secret Rotator operator

```shell
# uninstall secretrotator using the following command
helm uninstall secretrotator -n ${NAMESPACE}

# uninstall secretrotator object (path should be pointing to secretrotator CR yaml)
kubectl delete -f [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/master/charts/jfrog-registry-operator/examples/secretrotator.yaml) -n ${NAMESPACE}

# remove CRD from cluster
kubectl delete crd secretrotators.apps.jfrog.com
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
