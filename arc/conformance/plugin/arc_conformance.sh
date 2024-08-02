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

# setup kubeconfig for conformance test
setupKubeConfig() {
  KUBECTL_CONTEXT=azure-arc-appconfig-test
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
validateParameters() {
  if [ -z "${TENANT_ID}" ]; then
    echo "ERROR: parameter TENANT_ID is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [ -z "${SUBSCRIPTION_ID}" ]; then
    echo "ERROR: parameter SUBSCRIPTION_ID is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [ -z "${CLUSTER_NAME}" ]; then
    echo "ERROR: parameter CLUSTER_NAME is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [ -z "${CLUSTER_RG}" ]; then
    echo "ERROR: parameter CLUSTER_RG is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  # OBJECT_ID is an id of the Service Principal created in conformance test subscription.
  if [ -z "${OBJECT_ID}" ]; then
    echo "ERROR: parameter OBJECT_ID is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [[ -z "${WORKLOAD_CLIENT_ID}" ]]; then
    if [ -z "${AZURE_CLIENT_ID}" ]; then
      echo "ERROR: parameter AZURE_CLIENT_ID is required." > "${results_dir}"/error
      python3 /arc/setup_failure_handler.py
    fi

    if [ -z "${AZURE_CLIENT_SECRET}" ]; then
      echo "ERROR: parameter AZURE_CLIENT_SECRET is required." > "${results_dir}"/error
      python3 /arc/setup_failure_handler.py
    fi
   fi
}

login_to_azure() {
   if [[ -z $WORKLOAD_CLIENT_ID ]]; then
      echo "logging in using service principal '${AZURE_CLIENT_ID}'"
      az login --service-principal \
         -u ${AZURE_CLIENT_ID} \
         -p ${AZURE_CLIENT_SECRET} \
         --tenant ${TENANT_ID} 2> ${results_dir}/error || python3 setup_failure_handler.py
   else
      echo "logging in using managed identity '${WORKLOAD_CLIENT_ID}'"
      az login --identity \
         -u ${WORKLOAD_CLIENT_ID} 2> ${results_dir}/error || python3 setup_failure_handler.py
   fi

	echo "setting subscription: ${SUBSCRIPTION_ID} as default subscription"
	az account set -s $SUBSCRIPTION_ID
}

validateParameters

# add az cli extensions 
az extension add --name aks-preview
az extension add --name k8s-extension

login_to_azure

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

# register the KubernetesConfiguration resource provider
az provider register --namespace Microsoft.KubernetesConfiguration

echo "INFO: Creating extension"
az k8s-extension create \
      --name appconfigurationkubernetesprovider \
      --extension-type Microsoft.AppConfiguration \
      --cluster-name "${CLUSTER_NAME}" \
      --resource-group "${CLUSTER_RG}" \
      --cluster-type managedClusters \
      --release-train preview 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py

# wait for provider pods
kubectl wait pod -n azappconfig-system --for=condition=Ready -l app.kubernetes.io/instance=azureappconfiguration.kubernetesprovider --timeout=5m

# clean up test resources
echo "INFO: cleaning up test resources" 
az k8s-extension delete \
  --name appconfigurationkubernetesprovider \
  --resource-group "${CLUSTER_RG}" \
  --cluster-type managedClusters \
  --cluster-name "${CLUSTER_NAME}" \
  --force \
  --yes \
  --no-wait