# Live functional E2E tests

These opt-in tests exercise a deployed provider image against real App
Configuration and Kubernetes services. They cover:

- Initial ConfigMap creation, including label selection, prefix trimming, and
  the provider reaching `Complete` status.
- Refresh from `v1` to `v2` without changing the provider resource.
- A selector change with refresh disabled, including removal of previously
  selected keys.

Assertions poll for the exact ConfigMap data and initial `Complete` status, with
a two-minute deadline per wait. The refresh interval is ten seconds. These are
test settings, not product latency guarantees.

## Prerequisites

- Go matching [go.mod](../../go.mod), PowerShell 7, Docker with Linux containers,
  Azure CLI, Helm 3, and kubectl.
- An explicitly named kubeconfig context for a non-production AKS test cluster.
- An existing namespace and service account with
  `azure.workload.identity/client-id` and
  `azure.workload.identity/tenant-id` annotations.
- A federated credential matching the cluster issuer and the exact subject
  `system:serviceaccount:<namespace>:<service-account>`, with audience
  `api://AzureADTokenExchange`.
- The workload identity has **App Configuration Data Reader** on the test store.
- The local Azure CLI identity, authenticated using `az login`, has **App
  Configuration Data Owner** on the test store, ACR push access, and Kubernetes
  access to install the chart and create/delete test resources.
- The AKS kubelet identity has appropriate ACR pull access. This is separate from
  the provider's workload identity.

The test runner uses `AzureCLICredential`; the deployed provider uses workload
identity. No connection string, client secret, or local OIDC issuer is needed.
Never run untrusted PR code with these permissions or against shared production
resources.

## Azure Pipelines

[azure-pipelines.yml](../../azure-pipelines.yml) runs offline checks on pushes
and pull requests. During initial CI validation, the live stage runs only for
manually queued builds. Select the feature branch explicitly when testing
pipeline changes; the pipeline's default branch can still be `main`.

The live stage uses the `K8s Provider GitHub PR Integration Test` ARM service
connection with workload identity federation. The authenticated `AzureCLI@2`
task obtains a temporary, non-admin kubeconfig and configures kubelogin to use
the same Azure CLI session. Docker credentials and kubeconfig are isolated
under the agent's temporary directory and removed afterward. They are never
published as artifacts.

The dedicated environment is:

| Setting | Value |
| --- | --- |
| Resource group | `k8sprovider-integrationtest` |
| AKS/context | `k8sprovider-integrationtest-aks` |
| App Configuration | `https://appconfig-k8sprovider-integrationtest.azconfig.io` |
| ACR | `appconfige2etest` |
| Image repository | `kubernetes-provider-e2e` |
| Test namespace/service account | `e2e/k8s-provider-sa` |

The registry uses ABAC repository permissions. The kubelet's Repository Reader
assignment is restricted to `kubernetes-provider-e2e`; do not change the
repository or substitute `AcrPull`. The pipeline uses Repository Writer access
and separate management-plane access for registry metadata.

Before authorizing live runs, configure server-side checks on the service
connection: pipeline-specific authorization, trusted-branch control, and an
exclusive lock. The live stage uses `lockBehavior: sequential` so the lock is
held through installation, testing, cleanup, and reporting. The YAML condition
is not a security boundary; do not enable privileged fork builds or weaken
existing organization protections.

For initial validation, the service-connection branch allowlist is limited to
`refs/heads/linglingye/add-integration-tests`. Review the server-side allowlist
and branch-protection requirements before moving live execution to the main
branch or enabling a trusted pull-request workflow.

JUnit and diagnostic artifacts are published even when live tests fail.
JUnit includes the suite setup case as well as the three functional specs.
Cleanup failures fail the run, and cancelled jobs have a five-minute cleanup
grace period. Forced termination can still leave resources behind; inspect the
run-owned resource names before attempting recovery.

## Build, deploy, test, and clean up

From the repository root, in PowerShell:

```powershell
.\scripts\test-e2e.ps1 `
    -KubeContext "<existing-test-context>" `
    -AppConfigEndpoint "https://<test-store>.azconfig.io" `
    -AcrName "<test-registry>" `
    -TestNamespace "default" `
    -ServiceAccount "k8s-provider-sa"
```

The script:

1. Refuses to proceed if the provider CRD or its fixed cluster RBAC already
   exists. It never replaces an existing provider installation.
2. Checks configuration, reuses the existing service account, and stages the
   current chart and CRDs outside the source directories.
3. Builds a Linux/amd64 candidate image using the repository Dockerfile, then
   pushes it to a unique tag in the `kubernetes-provider-e2e` ACR repository.
4. Installs the chart in a unique temporary controller namespace and verifies
   the running pod uses that candidate.
5. Runs the three scenarios in the supplied test namespace, using unique
   provider/ConfigMap names and `e2e/<run-id>/...` Azure key prefixes.
6. Collects logs, events, the running image ID, and JUnit results beneath
   `artifacts/e2e/<run-id>`.
7. Removes test-owned Azure settings and Kubernetes resources, then uninstalls
   the temporary chart and deletes its controller namespace.

The existing test namespace, workload service account, federated credential,
and Azure role assignments are never changed. The candidate image and local
diagnostic artifacts are retained for reproduction; manage old candidate images
through the test registry's retention process.

Run this workflow serially on a cluster reserved for testing. A namespace alone
does not isolate multiple controller versions or cluster-scoped CRDs/RBAC. If
provider resources remain at teardown, the script deliberately refuses to
uninstall the CRD. Cleanup failures cause the run to fail.

Azure cleanup checks ownership tags and deletes individual key/label pairs
conditionally using their ETags. It never deletes a shared prefix or namespace.
If the process is forcibly terminated, inspect the logged run prefix and
resource names before manually cleaning up leftovers. Diagnostics do not dump
Kubernetes Secrets or service-account tokens.

## Run only the suite against an existing candidate

After deploying the intended image yourself:

```powershell
$env:E2E_APPCONFIG_ENDPOINT = "https://<test-store>.azconfig.io"
$env:E2E_KUBE_CONTEXT = "<existing-test-context>"
$env:E2E_NAMESPACE = "default"
$env:E2E_SERVICE_ACCOUNT = "k8s-provider-sa"
go test -tags=e2e ./test/e2e -count=1 -timeout=15m -v
```

`E2E_RUN_ID` is optional; the suite generates a UUID when it is omitted.
`KUBECONFIG` is honored, but the context must always be supplied explicitly.
Missing required configuration fails instead of skipping live tests. The
standalone suite does not install or uninstall the provider.

Ordinary `go test ./...` excludes this suite through the `e2e` build tag.
Use `-count=1` to prevent cached results. To validate configuration handling
without accessing Azure or Kubernetes:

```powershell
go test -tags=e2e ./test/e2e -run "^TestLiveConfiguration$" -count=1
```
