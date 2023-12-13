#!/usr/bin/env bash


while [[ $(kubectl get pods -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}' -n $NAMESPACE) == *"False"* ]]; do
    echo "waiting for all pods to be in ready state" && sleep 1;
done
echo "All pods are ready"
sleep 5

