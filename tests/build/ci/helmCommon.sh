#!/usr/bin/env bash

#set -o errexit
#set -o nounset
#set -o pipefail
set -e

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"

IMAGE_TAG=${CHART_TESTING_TAG:-"0.1.0"}
IMAGE_REPOSITORY=${CHART_TESTING_IMAGE:-"releases-docker.jfrog.io/charts-ci"}
chart_name="jfrog-registry-operator"
ctYamlPath=charts/test/ct.yaml
chartDir=charts/
chartPath=charts/jfrog-registry-operator


docker_exec() {
    docker exec --interactive ct "$@"
}

cleanup() {
    echo 'Removing ct container...'
    docker kill ct > /dev/null 2>&1
    echo 'Done!'
}

install_helm() {
    echo 'Installing helm...'
    if [[ "${LOCAL_RUN}" = "true" ]]; then
        echo "Local run, not downloading helm cli..."
    else
        echo "Install Helm ${HELM_VERSION} cli"
        curl -s -O https://get.helm.sh/helm-"${HELM_VERSION}"-linux-amd64.tar.gz
        tar -zxvf helm-"${HELM_VERSION}"-linux-amd64.tar.gz && mv linux-amd64/helm /usr/local/bin/helm
        echo
    fi
}

install_kubeconform() {
    echo 'Installing kubeconform...'

    if [[ "${LOCAL_RUN}" = "true" ]]
    then
        echo "Local run, not downloading kubeconform cli..."
    else
        echo "CI run, downloading kubeconform cli..."
        curl -sSLo tmp/kubeconform.tar.gz "https://github.com/yannh/kubeconform/releases/download/$KUBECONFORM_VERSION/kubeconform-linux-amd64.tar.gz"
        tar xf tmp/kubeconform.tar.gz -C tmp && chmod +x tmp/kubeconform
        sudo mv tmp/kubeconform /usr/local/bin/kubeconform
    fi
}

validate_manifests() {
    echo "------------------------------------------------------------------------------------------------------------------------"
    echo " Validating Manifests!"
    echo " Charts to be processed: ${chart_name}"
    echo "------------------------------------------------------------------------------------------------------------------------"
    cd tmp
    echo "Validating chart ${chart_name}"
    rm -rf charts
    mkdir charts
    pwd
    helm template "${chartPath}" --output-dir charts > /dev/null 2>&1
    TEMPLATE_FILES="${chartPath}/templates"
    if [ -d "${TEMPLATE_FILES}" ] 
    then
        echo "------------------------------------------------------------------------------------------------------------------------"
        echo "==> Processing with default values..."
        echo "------------------------------------------------------------------------------------------------------------------------"
        # shellcheck disable=SC2086
        kubeconform ${TEMPLATE_FILES}
        if [ -d "${chartPath}/ci" ]
        then
            FILES="${chartPath}/ci/*"
            for file in $FILES
            do
                echo "------------------------------------------------------------------------------------------------------------------------"
                echo "==> Processing with $file..."
                echo "------------------------------------------------------------------------------------------------------------------------"
                rm -rf stable
                mkdir stable
                helm template "${chartPath}" -f "$file" --output-dir charts > /dev/null 2>&1
                TEMPLATE_FILES="${chartPath}/templates/*" 
                # shellcheck disable=SC2086
                kubeconform ${TEMPLATE_FILES}
            done
        fi
    fi
    echo "------------------------------------------------------------------------------------------------------------------------"
    echo "Done Manifests validating!"
    echo
}

lintCharts(){
    mkdir -p tmp
    # install_kubeconform
    # install_helm
    sed -i "s/ART_USER/${int_entplus_deployer_user}/g" ${PROJECT_ROOT_DIR}/charts/test/ct.yaml
    sed -i "s/ART_PASSWORD/${int_entplus_deployer_apikey}/g" ${PROJECT_ROOT_DIR}/charts/test/ct.yaml
    echo "chart ::: $chart_name"
    echo "Charts Linting using helm !"

    # shellcheck disable=SC2086
    docker run --rm -v "${PROJECT_ROOT_DIR}:/workdir" --workdir /workdir "$IMAGE_REPOSITORY:$IMAGE_TAG" ct lint ${CHART_TESTING_ARGS} --config ${ctYamlPath} --chart-dirs ${chartDir} --validate-maintainers=false --check-version-increment=false | tee tmp/lint.log
    echo "Done Charts Linting!"
    echo
    if [[ -z "${CHART_TESTING_ARGS}" ]]; then
        # Validate Kubernetes manifests
        validate_manifests | tee -a tmp/lint.log
    fi
}

packageCharts() {
    local dockerImageTag=${1}
    local releaseVersionFlag=${2}
    local ubiminimalVersion=${3}
    local artiDockerRepo=${4}
    local imageRegistry=
    local imageRepository=
    cd ../..
    echo "------------------------------------------------------------------------------------------------------------------------"
    echo " Start packaging and uploading Charts ... "
    echo "------------------------------------------------------------------------------------------------------------------------"
    echo " Charts to be packaged are : ${chart_name}"
    echo "------------------------------------------------------------------------------------------------------------------------"
        echo "<<<========|$chartPath|==========>>>"
        chartNameModified=$(echo $chart_name | tr '-' '_' )
        echo -e "\n"
        echo "==> Packaging Chart : ${chart_name}"
        echo "------------------------------------------------------------------------------------------------------------------------"
        chart_version=$(grep "version:" "${chartPath}/Chart.yaml" | cut -d" " -f 2)
        app_version=$(grep "appVersion:" "${chartPath}/Chart.yaml" | cut -d" " -f 2)
        echo "==> Chart Version : ${chart_version}"
        
        if [[ "${releaseVersionFlag}" != "true" ]]; then
            final_chart_name="${chart_name}-${chart_version}-x-${run_number}.tgz"
            BUMP_CHART_VERSION="${chart_version}-x-${run_number}"
            imageRegistry=entplus.jfrog.io
            imageRepository=${artiDockerRepo}/jfrog/jfrog-registry-operator
        elif [[ ${dockerImageTag} == *m0* ]]; then
            final_chart_name="${chart_name}-${dockerImageTag}.tgz"
            BUMP_CHART_VERSION="${dockerImageTag}"
            imageRegistry=entplus.jfrog.io
            imageRepository=dev-releases-docker-virtual/jfrog/jfrog-registry-operator
        else
            final_chart_name="${chart_name}-${chart_version}.tgz"
            BUMP_CHART_VERSION="${chart_version}"
        fi

        echo "Updating Chart version in Charts.yaml to ${BUMP_CHART_VERSION} ..."
        sed -i "s/${chart_version}/${BUMP_CHART_VERSION}/g" "${chartPath}/Chart.yaml"
        sed -i "s/^appVersion: .*$/appVersion: ${dockerImageTag}/g" "${chartPath}/Chart.yaml"

        # Updating chart version in CHANGELOG.md file
        echo "Updating Chart version in CHANGELOG.md to ${BUMP_CHART_VERSION} ..."
        sed -i "s/${chart_version}/${BUMP_CHART_VERSION}/g" "${chartPath}/CHANGELOG.md"

        echo "==> Packaging Chart as : ${final_chart_name}"
        docker rm -f ct || true
        echo "11"
        # shellcheck disable=SC2154
        docker run --rm --interactive --detach --name ct \
            -v ${PROJECT_ROOT_DIR}:/workdir \
            --workdir /workdir \
            "$IMAGE_REPOSITORY:$IMAGE_TAG" \
            cat

        if [[ ! -z "${imageRegistry}" ]]; then
            docker_exec yq w -i ${chartDir}/${chart_name}/values.yaml "image.registry" "${imageRegistry}"
        fi
        if [[ ! -z "${imageRepository}" ]]; then
            docker_exec yq w -i ${chartDir}/${chart_name}/values.yaml "image.repository" "${imageRepository}"
        fi
        if [[ ! -z "${dockerImageTag}" ]]; then
            docker_exec yq w -i ${chartDir}/${chart_name}/values.yaml "image.tag" "${dockerImageTag}"
        fi
        docker_exec mkdir -p packages
        docker_exec helm repo add jfrog https://charts.jfrog.io
        docker_exec helm repo update
        docker_exec helm dep update --debug "${chartDir}/${chart_name}"
        docker_exec rm -f /tmp/package.log || true
        docker_exec helm package "${chartDir}/${chart_name}" | tee /tmp/package.log
        pkg_location=$(awk '{ print $8 }' < /tmp/package.log)
        docker_exec mv "${pkg_location}" "packages/${final_chart_name}"
}

uploadCharts(){
    local artiHelmRepo=${1}
    local buildName=${2}
    local buildNumber=${3}

    echo "Uploading Chart to Helm Dev repo :"
        jfrog rt u \
        ${PROJECT_ROOT_DIR}/packages/${final_chart_name} \
        ${artiHelmRepo}/${final_chart_name} \
        --user=${int_entplus_deployer_user} --password=${int_entplus_deployer_apikey} \
        --url=${int_entplus_deployer_url} --build-name=${buildName} --build-number=${buildNumber} \
        || errorExit "Failed to upload $final_chart_name chart package to dev repo"

    docker_exec rm -rf packages
    echo -e "\n\n------------------------------------------------------------------------------------------------------------------------"
    echo "Done packaging and uploading Charts to Dev Repo !!!"
    echo "------------------------------------------------------------------------------------------------------------------------"
}