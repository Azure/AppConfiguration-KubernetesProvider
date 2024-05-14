FROM mcr.microsoft.com/azure-cli:latest
FROM bitnami/kubectl:1.20.9 as kubectl
ARG KUBE_VERSION=v1.25.3
ARG TARGETARCH

RUN pip install --trusted-host pypi.org --trusted-host files.pythonhosted.org pytest pytest-xdist filelock junit_xml kubernetes==24.2.0 azure.identity msrestazure azure-mgmt-hybridkubernetes azure-mgmt-kubernetesconfiguration==2.0.0

# install kubectl
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/

COPY arc/conformance/plugin/arc_conformance.sh /arc/arc_conformance.sh
COPY arc/conformance/plugin/setup_failure_handler.py /arc/setup_failure_handler.py

RUN ["chmod", "+x", "/arc/arc_conformance.sh"]
ENTRYPOINT ["/arc/arc_conformance.sh"]