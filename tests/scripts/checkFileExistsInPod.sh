#!/usr/bin/env bash

type=$1
name=$2
containerName=$3
mountPath=$4

assert=false
while [[ "$assert" == "false" ]]; do
    podName=$(kubectl get pods -l app.kubernetes.io/name="$name" -o=jsonpath='{.items[0].metadata.name}' -n $NAMESPACE 2> /dev/null)
    fileExists=$(kubectl exec -it $podName -c "$containerName" -n $NAMESPACE -- bash -c "if [[ -f $mountPath ]] ; then echo true; else echo false ; fi" 2> /dev/null)
    echo "Expected $name $mountPath to be present"
    if [[ $fileExists ]]; then
        assert=true
        echo "$name container $mountPath check is successfull"
        break
    fi
    sleep 10
done
kubectl rollout status "$type"/"$name" -n $NAMESPACE

