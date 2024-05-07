#!/usr/bin/env bash

set -e

results_dir="${RESULTS_DIR:-/tmp/results}"

waitForArc() {
    ready=false
    max_retries=60
    sleep_seconds=20

    for i in $(seq 1 $max_retries)
    do
    status=$(helm ls -a -A -o json | jq '.[]|select(.name=="azure-arc").status' -r)
    if [ "$status" == "deployed" ]; then
        echo "helm release successful"
        ready=true
        break
    elif [ "$status" == "failed" ]; then
        echo "helm release failed"
        break
    else
        echo "waiting for helm release to be successful. Status - ${status}. Attempt# $i of $max_retries"
        sleep ${sleep_seconds}
    fi
    done

    echo "$ready"
}

saveResult() {
  # prepare the results for handoff to the Sonobuoy worker.
  cd "${results_dir}"
  # Sonobuoy worker expects a tar file.
  tar czf results.tar.gz ./*
  # Signal the worker by writing out the name of the results file into a "done" file.
  printf "%s/results.tar.gz" "${results_dir}" > "${results_dir}"/done
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResult EXIT

# initial environment variables for the plugin
setEnviornmentVariables() {
  export JUNIT_OUTPUT_FILEPATH=/tmp/results/
  export IS_ARC_TEST=true
  export CI_KIND_CLUSTER=true
}

# setup kubeconfig for conformance test
setupKubeConfig() {
  KUBECTL_CONTEXT=azure-arc-test
  APISERVER=https://kubernetes.default.svc/
  TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
  cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt > ca.crt

  kubectl config set-cluster ${KUBECTL_CONTEXT} \
    --embed-certs=true \
    --server=${APISERVER} \
    --certificate-authority=./ca.crt 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

  kubectl config set-credentials ${KUBECTL_CONTEXT} --token="${TOKEN}" 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

  # Delete previous rolebinding if exists. And ignore the error if not found.
  kubectl delete clusterrolebinding clusterconnect-binding --ignore-not-found
  kubectl create clusterrolebinding clusterconnect-binding --clusterrole=cluster-admin --user="${OBJECT_ID}" 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

  kubectl config set-context ${KUBECTL_CONTEXT} \
    --cluster=${KUBECTL_CONTEXT} \
    --user=${KUBECTL_CONTEXT} \
    --namespace=default 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

  kubectl config use-context ${KUBECTL_CONTEXT} 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py
  echo "INFO: KubeConfig setup complete"
}

# validate enviorment variables
if [ -z "${TENANT_ID}" ]; then
  echo "ERROR: parameter TENANT_ID is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

if [ -z "${SUBSCRIPTION_ID}" ]; then
  echo "ERROR: parameter SUBSCRIPTION_ID is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

if [ -z "${AZURE_CLIENT_ID}" ]; then
  echo "ERROR: parameter AZURE_CLIENT_ID is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

if [ -z "${AZURE_CLIENT_SECRET}" ]; then
  echo "ERROR: parameter AZURE_CLIENT_SECRET is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

if [ -z "${ARC_CLUSTER_NAME}" ]; then
  echo "ERROR: parameter ARC_CLUSTER_NAME is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

if [ -z "${ARC_CLUSTER_RG}" ]; then
  echo "ERROR: parameter ARC_CLUSTER_RG is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

# OBJECT_ID is an id of the Service Principal created in conformance test subscription.
if [ -z "${OBJECT_ID}" ]; then
  echo "ERROR: parameter OBJECT_ID is required." > "${results_dir}"/error
  python3 /arc/setup_failure_handler.py
fi

# add az cli extensions 
az extension add --name aks-preview
az extension add --name k8s-extension

# login with service principal
az login --service-principal \
  -u "${AZURE_CLIENT_ID}" \
  -p "${AZURE_CLIENT_SECRET}" \
  --tenant "${TENANT_ID}" 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

az account set --subscription "${SUBSCRIPTION_ID}" 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

# set environment variables
setEnviornmentVariables

# setup Kubeconfig
setupKubeConfig

# Wait for resources in ARC agents to come up
echo "INFO: Waiting for ConnectedCluster to come up"
waitSuccessArc="$(waitForArc)"
if [ "${waitSuccessArc}" == false ]; then
    echo "helm release azure-arc failed" > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
    exit 1
else
    echo "INFO: ConnectedCluster is available"
fi

echo "INFO: Creating extension"
az k8s-extension create \
      --name azureappconfig \
      --extension-type Microsoft.AppConfiguration \
      --scope cluster \
      --cluster-name "${ARC_CLUSTER_NAME}" \
      --resource-group "${ARC_CLUSTER_RG}" \
      --cluster-type managedClusters \
      --release-train preview \
      --release-namespace kube-system 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

# wait for secrets store csi driver and provider pods
kubectl wait pod -n kube-system --for=condition=Ready -l app=secrets-store-csi-driver --timeout=5m
kubectl wait pod -n kube-system --for=condition=Ready -l app=csi-secrets-store-provider-azure --timeout=5m

/arc/e2e -ginkgo.v -ginkgo.skip="${GINKGO_SKIP}" -ginkgo.focus="${GINKGO_FOCUS}"

# clean up test resources
echo "INFO: cleaning up test resources" 
az k8s-extension delete \
  --name azureappconfig \
  --resource-group "${ARC_CLUSTER_RG}" \
  --cluster-type managedClusters \
  --cluster-name "${ARC_CLUSTER_NAME}" \
  --force \
  --yes \
  --no-wait