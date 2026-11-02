#Requires -Version 7.0

# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$KubeContext,
    [Parameter(Mandatory)][uri]$AppConfigEndpoint,
    [Parameter(Mandatory)][string]$AcrName,
    [Parameter(Mandatory)][string]$ServiceAccount,
    [string]$TestNamespace = "default",
    [ValidatePattern('^[a-z0-9]+(?:[._/-][a-z0-9]+)*$')]
    [string]$ImageRepository = "kubernetes-provider-e2e"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false

function Invoke-Tool {
    param([string]$Name, [string[]]$Arguments)
    & $Name @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Name $($Arguments -join ' ') failed with exit code $LASTEXITCODE."
    }
}

function Get-ClusterObject {
    param([string]$Kind, [string]$Name)
    $result = Invoke-Tool kubectl ($script:kubeArgs + @(
        "get", $Kind, $Name, "--ignore-not-found", "--output", "json"
    )) | Out-String
    if (-not [string]::IsNullOrWhiteSpace($result)) {
        return $result | ConvertFrom-Json
    }
}

function Save-Diagnostic {
    param([string]$FileName, [string[]]$Arguments)
    try {
        Invoke-Tool kubectl ($script:kubeArgs + $Arguments) |
            Set-Content -Path (Join-Path $script:outputDirectory $FileName) -Encoding utf8NoBOM
    }
    catch {
        Write-Warning "Could not capture ${FileName}: $_"
    }
}

if ($AppConfigEndpoint.Scheme -ne "https" -or $AppConfigEndpoint.UserInfo -or
    $AppConfigEndpoint.Query -or $AppConfigEndpoint.Fragment -or $AppConfigEndpoint.AbsolutePath -ne "/") {
    throw "AppConfigEndpoint must be an HTTPS store endpoint without credentials, query, or path."
}
$endpoint = $AppConfigEndpoint.GetLeftPart([System.UriPartial]::Authority)
foreach ($tool in @("az", "docker", "helm", "kubectl", "go")) {
    Get-Command $tool -ErrorAction Stop | Out-Null
}

$repoRoot = Split-Path $PSScriptRoot -Parent
$runID = [guid]::NewGuid().ToString("N")
$release = "acp-e2e-$runID"
$controllerNamespace = $release
$kubeArgs = @("--context", $KubeContext, "--request-timeout=30s")
$outputDirectory = Join-Path $repoRoot "artifacts" "e2e" $runID
$chartDirectory = Join-Path $outputDirectory "chart"
$version = "0.0.0-e2e.$runID"
$installAttempted = $false
$namespaceCreated = $false
$failure = $null
$cleanupFailures = [System.Collections.Generic.List[string]]::new()
$environmentNames = @("E2E_APPCONFIG_ENDPOINT", "E2E_KUBE_CONTEXT", "E2E_NAMESPACE", "E2E_SERVICE_ACCOUNT", "E2E_RUN_ID")
$savedEnvironment = @{}
foreach ($name in $environmentNames) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

Push-Location $repoRoot
try {
    $actualContext = (Invoke-Tool kubectl @("config", "get-contexts", $KubeContext, "--output", "name") | Out-String).Trim()
    if ($actualContext -ne $KubeContext) {
        throw "Kubernetes context '$KubeContext' does not exist. The current context is never used implicitly."
    }
    foreach ($resource in @(
        @("crd", "azureappconfigurationproviders.azconfig.io"),
        @("clusterrole", "appconfig-manager-role"),
        @("clusterrolebinding", "appconfig-manager-rolebinding")
    )) {
        if ($null -ne (Get-ClusterObject $resource[0] $resource[1])) {
            throw "$($resource -join '/') already exists. This workflow only installs on a cluster without an existing provider; it will not upgrade or replace one."
        }
    }
    $account = Invoke-Tool kubectl ($kubeArgs + @(
        "get", "serviceaccount", $ServiceAccount, "--namespace", $TestNamespace, "--output", "json"
    )) | ConvertFrom-Json
    foreach ($annotation in @("azure.workload.identity/client-id", "azure.workload.identity/tenant-id")) {
        $property = $account.metadata.annotations.PSObject.Properties[$annotation]
        if ($null -eq $property -or [string]::IsNullOrWhiteSpace($property.Value)) {
            throw "Service account $TestNamespace/$ServiceAccount is missing $annotation."
        }
    }

    Invoke-Tool go @("test", "-tags=e2e", "./test/e2e", "-run", "^TestLiveConfiguration$", "-count=1")
    $dockerOS = (Invoke-Tool docker @("info", "--format", "{{.OSType}}") | Out-String).Trim()
    if ($dockerOS -ne "linux") {
        throw "A Docker engine running Linux containers is required."
    }
    $registry = (Invoke-Tool az @("acr", "show", "--name", $AcrName, "--query", "loginServer", "--output", "tsv") | Out-String).Trim()
    if (-not $registry) {
        throw "ACR did not return a login server."
    }
    $image = "${registry}/${ImageRepository}:e2e-$runID"
    New-Item -ItemType Directory -Path $chartDirectory | Out-Null
    Copy-Item (Join-Path $repoRoot "deploy" "Chart.yaml") $chartDirectory
    Copy-Item (Join-Path $repoRoot "deploy" ".helmignore") $chartDirectory
    Copy-Item (Join-Path $repoRoot "deploy" "templates") $chartDirectory -Recurse
    Copy-Item (Join-Path $repoRoot "config" "crd" "bases" "*.yaml") (Join-Path $chartDirectory "templates")
    $values = Get-Content (Join-Path $repoRoot "deploy" "parameter" "helm-values.yaml") -Raw
    $values.Replace("[[dockerImageRepoName]]", "$registry/$ImageRepository").
        Replace("[[dockerImageTag]]", "e2e-$runID") |
        Set-Content (Join-Path $chartDirectory "values.yaml") -Encoding utf8NoBOM
    Invoke-Tool helm @("lint", $chartDirectory, "--strict")
    Invoke-Tool helm @("package", $chartDirectory, "--destination", $outputDirectory, "--version", $version)
    $chartPackage = Join-Path $outputDirectory "kubernetes-provider-$version.tgz"

    Write-Host "Building candidate $image"
    Invoke-Tool docker @("build", "--platform", "linux/amd64", "--build-arg", "MODULE_VERSION=$version", "--tag", $image, $repoRoot)
    Invoke-Tool az @("acr", "login", "--name", $AcrName)
    Invoke-Tool docker @("push", $image)
    $image | Set-Content (Join-Path $outputDirectory "image.txt") -Encoding utf8NoBOM

    Invoke-Tool kubectl ($kubeArgs + @("create", "namespace", $controllerNamespace))
    $namespaceCreated = $true
    $installAttempted = $true
    Invoke-Tool helm @(
        "install", $release, $chartPackage, "--kube-context", $KubeContext,
        "--namespace", $controllerNamespace, "--wait", "--timeout", "5m"
    )
    $pods = Invoke-Tool kubectl ($kubeArgs + @(
        "get", "pods", "--namespace", $controllerNamespace,
        "--selector", "app.kubernetes.io/instance=$release", "--output", "json"
    )) | ConvertFrom-Json
    if (@($pods.items).Count -ne 1 -or $pods.items[0].spec.containers[0].image -ne $image) {
        throw "The deployed pod does not match the candidate image."
    }
    $pods.items[0].status.containerStatuses |
        Select-Object name, image, imageID, ready, restartCount |
        ConvertTo-Json | Set-Content (Join-Path $outputDirectory "running-image.json") -Encoding utf8NoBOM

    $env:E2E_APPCONFIG_ENDPOINT = $endpoint
    $env:E2E_KUBE_CONTEXT = $KubeContext
    $env:E2E_NAMESPACE = $TestNamespace
    $env:E2E_SERVICE_ACCOUNT = $ServiceAccount
    $env:E2E_RUN_ID = $runID
    Invoke-Tool go @(
        "test", "-tags=e2e", "./test/e2e", "-run", "^TestE2E$", "-count=1", "-timeout=15m", "-v",
        "-ginkgo.v", "-ginkgo.no-color",
        "-ginkgo.junit-report=$(Join-Path $outputDirectory 'junit.xml')"
    ) | Tee-Object -FilePath (Join-Path $outputDirectory "tests.log")

    $remainingSettings = Invoke-Tool az @(
        "appconfig", "kv", "list", "--endpoint", $endpoint, "--auth-mode", "login",
        "--key", "e2e/$runID/*", "--label", "*", "--fields", "key", "label", "--output", "json", "--only-show-errors"
    ) | ConvertFrom-Json
    if (@($remainingSettings).Count -ne 0) {
        throw "Azure test settings remain after cleanup. Inspect the run prefix e2e/$runID/."
    }
}
catch {
    $failure = $_
}
finally {
    if ($installAttempted) {
        Save-Diagnostic "provider.log" @(
            "logs", "deployment/az-appconfig-k8s-provider", "--namespace", $controllerNamespace, "--tail=300"
        )
        Save-Diagnostic "pods.json" @("get", "pods", "--namespace", $controllerNamespace, "--output", "json")
        Save-Diagnostic "events.json" @("get", "events", "--namespace", $controllerNamespace, "--output", "json")
        try {
            $crd = Get-ClusterObject "crd" "azureappconfigurationproviders.azconfig.io"
            if ($null -ne $crd) {
                $providers = Invoke-Tool kubectl ($kubeArgs + @(
                    "get", "azureappconfigurationproviders", "--all-namespaces", "--output", "json"
                )) | ConvertFrom-Json
                if (@($providers.items).Count -ne 0) {
                    throw "Provider resources remain. Refusing to uninstall the CRD and delete their data; inspect release $release in $controllerNamespace."
                }
            }
            $releases = Invoke-Tool helm @(
                "list", "--kube-context", $KubeContext, "--namespace", $controllerNamespace,
                "--all", "--filter", "^$release$", "--output", "json"
            ) | ConvertFrom-Json
            if (@($releases).Count -ne 0) {
                Invoke-Tool helm @(
                    "uninstall", $release, "--kube-context", $KubeContext,
                    "--namespace", $controllerNamespace, "--wait", "--timeout", "2m"
                )
            }
            $installAttempted = $false
        }
        catch {
            $cleanupFailures.Add("Provider cleanup failed: $_")
        }
    }
    if ($namespaceCreated -and -not $installAttempted) {
        try {
            Invoke-Tool kubectl ($kubeArgs + @("delete", "namespace", $controllerNamespace, "--wait=true", "--timeout=2m"))
        }
        catch {
            $cleanupFailures.Add("Namespace cleanup failed: $_")
        }
    }
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], "Process")
    }
    Pop-Location
}

foreach ($cleanupFailure in $cleanupFailures) {
    Write-Warning $cleanupFailure
}
if ($null -ne $failure) {
    throw $failure
}
if ($cleanupFailures.Count -ne 0) {
    throw "Live tests completed, but cleanup failed. See the warnings above."
}
Write-Host "Live E2E tests passed. Results: $outputDirectory"
Write-Host "Candidate image retained for reproduction: $image"
