# JFrog Secret Rotator Operator Chart Changelog
All changes to this chart will be documented in this file.

## [2.1.1] - June 11, 2025
* Added subdomain support in registry operator `spec.artifactorySubdomains` [GH-34](https://github.com/jfrog/jfrog-registry-operator/pull/34)

## [2.1.0] - May 27, 2025
* Added support for `exchangedServiceAccounts`. Using this, multiple service accounts can be created, which can later be used in `serviceAccount.name` and `serviceAccount.namespace` in the custom resource
* Added permissions for `serviceaccounts` and `serviceaccounts/token` for the target service accounts.
* Removed support for operator-specific service account annotations support. Users can now create custom service accounts or use `exchangedServiceAccounts`.
* The operator's service account requires an optional ARN annotation. If the user does not configure any service account, they will need to update the annotation using `serviceAccount.annotations`
* Removed default labels from the deployment. Customers can now pass the required labels to avoid any duplication with Kustomize. [GH-32](https://github.com/jfrog/jfrog-registry-operator/issues/32)

## [2.0.0] - May 15, 2025
*** Important Changes ***
* In the custom resource, the introduced `spec.generatedSecrets` configuration typically involves specifying: `secretName` – the name of the Secret to be generated, and `secretType` – the type of Secret to generate (e.g., Docker, Generic)
* Scope: Scope can be anything (Optional)
* Note: Currently spec.secretName is supported but going forward this will be deprecated soon.

## [1.4.2] - Mar 26, 2025
* Release of jfrog-registry-operator `1.4.2`
* Added support for providing containerSecurityContext

## [1.4.1] - Sept 03, 2024
* Release of jfrog-registry-operator `1.4.1`

## [1.3.0] - Jul 17, 2024
* Release of jfrog-registry-operator `1.3.0`

## [1.2.0] - Jul 15, 2024
* Release of jfrog-registry-operator `1.2.0`

## [1.1.0] - Feb 1, 2024
* Updated README.md to create a namespace using `--create-namespace` as part of helm install

## [1.0.0] - Dec 12, 2023
* First release of jfrog-registry-operator `1.0.0`

## [1.0.1] - Dec 20, 2023
* Adding serviceMonitor to jfrog-registry-operator