
# JFrog Registry Operator

## Two ways to install an operator using manual deployment and a Helm chart on Kubernetes

### Install operator using helm chart

```bash
# Get the latest [Helm release](https://github.com/helm/helm#install) Note: (only V3 is supported)
# before installing JFrog helm charts, you need to add the [JFrog helm repository](https://charts.jfrog.io) to your helm client.
helm repo add jfrog https://charts.jfrog.io

# update the helm repo
helm repo update

# decide on the namespace and kubernetes service account name you will want to create
export SERVICE_ACCOUNT_NAME="<service account name>"
export ANNOTATIONS="<Role annotation for service account>" # Example: eks.amazonaws.com/role-arn: arn:aws:iam::000000000000:role/jfrog-operator-role
export NAMESPACE="jfrog-operator"

# install JFrog secret rotator operator
helm upgrade --install secretrotator jfrog/jfrog-registry-operator --set "serviceAccount.name=${SERVICE_ACCOUNT_NAME}" --set serviceAccount.annotations=${ANNOTATIONS}  -n ${NAMESPACE}
```

Once operator is in running state, configure artifactoryUrl, refreshTime, namespaceSelector and secretMetadata in [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/main/charts/jfrog-registry-operator/examples/secretrotator.yaml)

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
kubectl delete -f [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/main/charts/jfrog-registry-operator/examples/secretrotator.yaml) -n ${NAMESPACE}

# remove CRD from cluster
kubectl delete crd secretrotators.apps.jfrog.com
```

### Install JFrog secret rotator operator manually

```sh
# deploy the crd:
kubectl apply -f https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/main/config/crd/bases/apps.jfrog.com_secretrotators.yaml

# install operator
kubectl apply -f https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/main/config/deploy/operator.yaml

# create secretrotator object
Ref: https://github.com/jfrog/jfrog-registry-operator/blob/main/charts/jfrog-registry-operator/examples/secretrotator.yaml
kubectl apply -f [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/main/charts/jfrog-registry-operator/examples/secretrotator.yaml) -n ${NAMESPACE}
```

#### Uninstall operator

```sh
# delete secretrotator object
Ref: https://github.com/jfrog/jfrog-registry-operator/blob/main/charts/jfrog-registry-operator/examples/secretrotator.yaml
kubectl delete -f secretrotator.yaml -n ${NAMESPACE}

# delete the operator:
kubectl delete -f https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/main/config/deploy/operator.yaml

### delete CRD
kubectl delete -f https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/main/config/crd/bases/apps.jfrog.com_secretrotators.yaml
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

## Monitoring operator

Follow [monitoring setup docs](./config/monitoring/).

## How to contribute
1. Fork and clone the repo
2. Add your changes, make sure you update new specs by running `make generate` and `make manifests`
3. Run locally : `make install`
4. Apply CR(`kubectl apply -f config/samples/jfrog_v1alpha1_secretrotator.yaml`) and check application is working as expected
4. Once changes are tested raise a PR, [Ref](https://docs.github.com/en/desktop/working-with-your-remote-repository-on-github-or-github-enterprise/creating-an-issue-or-pull-request-from-github-desktop)
