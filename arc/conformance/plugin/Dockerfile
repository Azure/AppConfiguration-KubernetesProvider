FROM mcr.microsoft.com/azure-cli:latest
ARG KUBE_VERSION=v1.25.3
ARG TARGETARCH

RUN pip install --trusted-host pypi.org --trusted-host files.pythonhosted.org pytest pytest-xdist filelock junit_xml kubernetes==24.2.0 azure.identity msrestazure azure-mgmt-hybridkubernetes azure-mgmt-kubernetesconfiguration==2.0.0

# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin

COPY arc/conformance/plugin/arc_conformance.sh /arc/arc_conformance.sh
COPY arc/conformance/plugin/setup_failure_handler.py /arc/setup_failure_handler.py

RUN ["chmod", "+x", "/arc/arc_conformance.sh"]
ENTRYPOINT ["/arc/arc_conformance.sh"]