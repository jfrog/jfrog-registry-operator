#!/usr/bin/env bash

if [[ $START_KIND != "true" ]]; then
    # Prepare Vcluster
    time=$(date +%Y%m%d%H%M%S)
    kubectl create ns e2e-${time} || true
    echo "Using namespace: e2e-${time}"
    export NAMESPACE=e2e-${time}
    echo "namespace: $NAMESPACE"

    if [[ $2 == "ci" ]]; then
        helm upgrade --install e2e-${time} entplus/vcluster --version ${VCLUSTER_VERSION} --namespace e2e-${time} -f ../vcluster-values.yaml
        vcluster connect e2e-${time} &
        vClusterPid=$!
    else
        vcluster create e2e-${time} -n e2e-${time} -f ../vcluster-values.yaml &
        vClusterPid=$!
    fi

    # Wait for Vcluster to come up
    ready=false
    wait_period=0
    while [[ $ready = false ]]
    do
        wait_period=$(($wait_period+1))
        sleep 5
        if [[ $wait_period -gt 20 ]];then
            echo "Timed out waiting for vcluster to come up"
            exit
        fi
        echo "Waiting for vcluster to be ready"
        clusters=$(vcluster list --output json)
        for cluster in $(echo "${clusters}" | jq -r -c '.[]'); do
            vClusterName=$(echo "$cluster" | jq -r '.Name')
            vClusterStatus=$(echo "$cluster" | jq -r '.Status')
            currentContext=$(kubectl config current-context)
              if [[ "$vClusterName" == "e2e-${time}" ]] && [[ "$vClusterStatus" == "Running" ]] && [[ "$currentContext" == *"e2e-$time"* ]];then
                ready=true
              fi
        done
    done

    sleep 10

    # Run integration tests using kuttl
    echo "Running integrations tests"
    if [[ $DEBUG_TESTS == "true" ]]; then
      kubectl kuttl test --report xml --skip-delete $1
    else
      kubectl kuttl test --report xml $1
    fi

    if [[ $DEBUG_TESTS != "true" ]]; then
        echo "Deleting Vcluster.."
        vcluster delete e2e-${time}
        kill -9 $vClusterPid
        kubectl delete ns e2e-${time}
    fi

else
    kubectl kuttl test --start-kind --parallel 3 --report xml $1
fi

# Kuttl reports
echo "******************TEST RESULT*******************"
cat kuttl-report.xml
echo "*********************END************************"