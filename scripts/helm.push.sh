#!/usr/bin/env bash
IMG="${1}"
HELM_OUTPUT="${2}"
HELM_VERSION="${3}"
segments=(${IMG//// })

host=${segments[0]}

helm push ${HELM_OUTPUT}/kubernetes-provider-$HELM_VERSION.tgz oci://$host/helm