#!/usr/bin/env bash

set -e

results_dir="${RESULTS_DIR:-/tmp/results}"

cluster_type_connected="managedClusters"

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

  if [ -z $CLUSTER_TYPE ]; then
		echo "CLUSTER_TYPE is undefined, defaulting to managedClusters" 
		CLUSTER_TYPE=$cluster_type_connected
	fi

  if [ -z "${CLUSTER_NAME}" ]; then
    echo "ERROR: parameter CLUSTER_NAME is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [ -z "${CLUSTER_RG}" ]; then
    echo "ERROR: parameter CLUSTER_RG is required." > "${results_dir}"/error
    python3 /arc/setup_failure_handler.py
  fi

  if [[ -z "${WORKLOAD_CLIENT_ID}" ]]; then
      echo "ERROR: parameter AZURE_CLIENT_ID is required." > "${results_dir}"/error
      python3 /arc/setup_failure_handler.py
  fi
}

login_to_azure() {
	# Login with managed identity
  echo "login to azure using managed identity: '${WORKLOAD_CLIENT_ID}'"
	az login --identity \
	--username ${WORKLOAD_CLIENT_ID} 2> ${results_dir}/error || python3 setup_failure_handler.py || exit 1

	echo "setting subscription: ${SUBSCRIPTION_ID} as default subscription"
	az account set -s $SUBSCRIPTION_ID
}

addK8sExtension() {
  echo "adding k8s-extension extension"
  az extension add --name k8s-extension

  # register the KubernetesConfiguration resource provider
  az provider register --namespace Microsoft.KubernetesConfiguration
}

createK8sProviderExtension() {
	echo "INFO: Creating Microsoft.AppConfiguration extension"
  az k8s-extension create \
    --name appconfigurationkubernetesprovider \
    --extension-type Microsoft.AppConfiguration \
    --cluster-name "${CLUSTER_NAME}" \
    --resource-group "${CLUSTER_RG}" \
    --cluster-type "${$CLUSTER_TYPE}" \
    --release-train preview 2> "${results_dir}"/error || python3 /arc/setup_failure_handler.py
}

waitForK8sProviderExtensionInstalled() {
    installedState=false
    max_retries=40
    sleep_seconds=10
    for i in $(seq 1 $max_retries)
    do
      echo "iteration: ${i}, clustername: ${CLUSTER_NAME}, resourcegroup: ${CLUSTER_RG}"
      provisioningState=$(az k8s-extension show  --cluster-name $CLUSTER_NAME --resource-group $CLUSTER_RG --cluster-type $CLUSTER_TYPE --name appconfigurationkubernetesprovider --query provisioningState -o json)
      provisioningState=$(echo $provisioningState | tr -d '"' | tr -d '"\r\n')
      echo "extension provisioning state: ${provisioningState}"
      if [ ! -z "$provisioningState" ]; then
         if [ "${provisioningState}" == "Succeeded" ]; then
            break
         fi
      fi
      sleep ${sleep_seconds}
    done
    echo "Microsoft.AppConfiguration extension installed"
}

deleteK8sProviderExtension() {
  az k8s-extension delete \
    --name appconfigurationkubernetesprovider \
    --resource-group "${CLUSTER_RG}" \
    --cluster-type "${$CLUSTER_TYPE}" \
    --cluster-name "${CLUSTER_NAME}" \
    --force \
    --yes
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

# validate parameters
validateParameters

# login to azure
login_to_azure

# add k8s-extension extension
addK8sExtension

# create k8s provider extension
createK8sProviderExtension

# wait for k8s provider extension to be installed
waitForK8sProviderExtensionInstalled

# delete k8s provider extension
deleteK8sProviderExtension