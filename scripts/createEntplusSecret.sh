#!/usr/bin/env bash
set -e

create_docker_registry_secret="$(cat <<EOF
apiVersion: v1
kind: Secret
type: kubernetes.io/dockerconfigjson
metadata:
  name: entplus-secret
stringData:
  .dockerconfigjson: '{"auths":{"${docker_registry}":{"username":"${docker_username}","password":"${docker_password}","email":"k8s@jfrog.com","auth":"$(echo -n "${docker_username}:${docker_password}" | base64 | tr -d \\n)"}}}'
EOF
)"

echo "$create_docker_registry_secret" | kubectl apply -n "${NAMESPACE}" -f -