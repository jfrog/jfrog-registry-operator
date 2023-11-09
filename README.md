
## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

## 2 Ways to install an operator using manual deployment and a Helm chart on Kubernetes
### Prepare env for installation, [Note]: We will remove this once we make it open source
```shell
# set the required env variabless
export NAMESPACE=jfrog-operator
# entplus docker registry credentials
export docker_username=<username>
export docker_password=<password>
export docker_registry=<registry>
kubectl create ns ${NAMESPACE}
RUN: ./scripts/createEntplusSecret.sh
```

## 1. Install JFrog secret rotator operator manually
1. Install Custom Resources Defination:
```sh
kubectl apply -f https://git.jfrog.info/users/carmith/repos/artifactory-secrets-rotator/raw/config/crd/bases/apps.jfrog.com_secretrotators.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```
2. Deploy the operator:
```sh
kubectl apply -f https://git.jfrog.info/users/carmith/repos/artifactory-secrets-rotator/raw/config/deploy/operator.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
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
kubectl delete -f https://git.jfrog.info/users/carmith/repos/artifactory-secrets-rotator/raw/config/deploy/operator.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```
### Uninstall CRDs
To delete the CRDs from the cluster:
```sh
kubectl delete -f https://git.jfrog.info/users/carmith/repos/artifactory-secrets-rotator/raw/config/crd/bases/apps.jfrog.com_secretrotators.yaml?at=refs%2Fheads%2Ffeature%2FINST-7020-1
```

## 2. Install operator using helm chart
### Install JFrog secret rotator operator
```shell
helm upgrade --install secretrotator ./charts/jfrog-registry-operator -f ./charts/jfrog-registry-operator/values.yaml --set "global.imagePullSecrets[0]=entplus-secret" -n ${NAMESPACE}
```
### UnInstall JFrog secret rotator operator
```shell
helm uninstall secretrotator -n ${NAMESPACE}
```
### [Note]: helm uninstall will not delete CRDs, to Uninstall CRDs run:
```sh
kubectl delete crd secretrotators.apps.jfrog.com
```

## Monitoring operator
Follow [monitoring setup docs](./config/monitoring/).

## How to contribute
1. Fork and clone the repo
2. Add your changes, make sure you update new specs by running `make generate` and `make manifests`
3. Run locally : `make install`
4. Apply CR(`kubectl apply -f config/samples/jfrog_v1alpha1_secretrotator.yaml`) and check application is working as expected
4. Once changes are tested raise a PR, [Ref](https://docs.github.com/en/desktop/working-with-your-remote-repository-on-github-or-github-enterprise/creating-an-issue-or-pull-request-from-github-desktop)

## Slack Channel
Raise a issue in slack channel
