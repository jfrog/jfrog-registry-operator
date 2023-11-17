
# Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

## 2 Ways to install an operator using manual deployment and a Helm chart on Kubernetes

## 1. Install operator using helm chart
### Install JFrog secret rotator operator
```shell
helm upgrade --install secretrotator jfrog/jfrog-registry-operator --set "serviceAccount.name=${SERVICE_ACCOUNT_NAME}" --set serviceAccount.annotations=${ANNOTATIONS}  -n ${NAMESPACE}
```

Once operator is in running state, configure artifactoryUrl, refreshTime, namespaceSelector and secretMetadata in /charts/jfrog-registry-operator/examples/secretrotator.yaml

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
```
Run: kubectl apply -f /charts/jfrog-registry-operator/examples/secretrotator.yaml -n ${NAMESPACE}
```

### UnInstall JFrog secret rotator operator
```shell
helm uninstall secretrotator -n ${NAMESPACE}
kubectl delete -f /charts/jfrog-registry-operator/examples/secretrotator.yaml -n ${NAMESPACE}
```

### [Note]: helm uninstall will not delete CRDs, to Uninstall CRDs run:
```sh
kubectl delete crd secretrotators.apps.jfrog.com
```

## 2. Install JFrog secret rotator operator manually
1. Install Custom Resources Defination:
```sh
kubectl apply -f https://git.jfrog.info/users/repos/jfrog-registry-operator/raw/config/crd/bases/apps.jfrog.com_secretrotators.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```
2. Deploy the operator:
```sh
kubectl apply -f https://git.jfrog.info/users/repos/jfrog-registry-operator/raw/config/deploy/operator.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
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
### Undeploy operator
Uninstall the operator:
```sh
kubectl delete -f https://git.jfrog.info/users/repos/jfrog-registry-operator/raw/config/deploy/operator.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```
### Uninstall CRDs
To delete the CRDs from the cluster:
```sh
kubectl delete -f https://git.jfrog.info/users/repos/jfrog-registry-operator/raw/config/crd/bases/apps.jfrog.com_secretrotators.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```

## Monitoring operator
Follow [monitoring setup docs](./config/monitoring/).

## How to contribute
1. Fork and clone the repo
2. Add your changes, make sure you update new specs by running `make generate` and `make manifests`
3. Run locally : `make install`
4. Apply CR(`kubectl apply -f config/samples/jfrog_v1alpha1_secretrotator.yaml`) and check application is working as expected
4. Once changes are tested raise a PR, [Ref](https://docs.github.com/en/desktop/working-with-your-remote-repository-on-github-or-github-enterprise/creating-an-issue-or-pull-request-from-github-desktop)
