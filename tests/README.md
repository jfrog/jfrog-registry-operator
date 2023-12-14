# E2E testing for operators using kuttl framework

There are two ways to run the integration tests locally.
If you are writing the tests make sure to test it locally using this approach.

## 1) Using Vcluster

### Prerequisites
* Access to an existing kubernetes cluster with kubectl installed [Refer](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/)
  `brew install kubectl`
* Install Vcluster [Refer](https://www.vcluster.com/docs/getting-started/setup)
  `brew install vcluster`
* Install helm [Refer](https://helm.sh/docs/intro/install/)
  `brew install helm`
* Install kuttl [Refer](https://kuttl.dev/docs/cli.html#setup-the-kuttl-kubectl-plugin)
  `brew tap kudobuilder/tap`
  `brew install kuttl-cli`

### To run the tests
```shell
# Run tests from the operator folder
./kuttl.sh
```
To run specific tests
```shell
./kuttl.sh "--test install"
```
Note: the folder name is the test name.
You can also pass extra kuttl args using this approach.
