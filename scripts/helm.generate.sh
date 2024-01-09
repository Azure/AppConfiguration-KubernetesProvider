#!/usr/bin/env bash
cp -a ./config/crd/bases/. ./deploy/templates

IMG="${1}"
OUTPUT_PATH="${2}"
HEML_VERSION="${3}"
segments=(${IMG//:/ })

repository=${segments[0]}
tag=${segments[1]}

cat ./deploy/parameter/helm-values.yaml |
 sed -e "s|\[\[dockerImageRepoName\]\]|${repository}|g" |
 sed -e "s|\[\[dockerImageTag\]\]|${tag}|g" > ./deploy/values.yaml

(cd ./deploy && helm dependency update)

if [ -n "$3" ]; then 
  helm package ./deploy -d ${OUTPUT_PATH} --version ${HEML_VERSION}
else
  helm package ./deploy -d ${OUTPUT_PATH} 
fi

