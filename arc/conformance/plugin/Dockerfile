FROM bitnami/kubectl:1.20.9 as kubectl

FROM mcr.microsoft.com/azure-cli:latest
ARG TARGETARCH

RUN pip install --trusted-host pypi.org --trusted-host files.pythonhosted.org pytest pytest-xdist filelock junit_xml kubernetes==24.2.0 azure.identity msrestazure azure-mgmt-hybridkubernetes azure-mgmt-kubernetesconfiguration==2.0.0

COPY arc/conformance/plugin/arc_conformance.sh /arc/arc_conformance.sh
COPY arc/conformance/plugin/setup_failure_handler.py /arc/setup_failure_handler.py

COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/

RUN ["chmod", "+x", "/arc/arc_conformance.sh"]
ENTRYPOINT ["/arc/arc_conformance.sh"]