# 📖 Guidelines

- Pull requests should be created on the master branch.

# ⚒️ Building and Testing the Sources

## Build JFrog Registry Operator

Make sure Go is installed by running:

```
go version
```

Clone the sources and CD to the root directory of the project:

```
git clone https://github.com/jfrog/jfrog-registry-operator.git
cd jfrog-registry-operator
```

Make your changes and Build the sources as follows:

Before running the application, generate mocks by running the following command from within the root directory of the project:

```sh
make generate
make manifests
```

To run the code, follow these steps:

```sh
kubectl apply -f config/crd/bases
```

```sh
go run main.go
```

Once operator is in running state, configure artifactoryUrl, refreshTime, namespaceSelector and secretMetadata in [secretrotator.yaml](https://github.com/jfrog/jfrog-registry-operator/blob/master/charts/jfrog-registry-operator/examples/secretrotator.yaml) diff terminal

```sh
kubectl apply -f config/samples/jfrog_v1alpha1_secretrotator.yaml`
```
