---
name: auto-vulnerability-patch
description: Automatically update Go dependencies with security patches for Azure App Configuration Kubernetes Provider. Use when user mentions dependency updates, security patches, vulnerability fixes, or automated dependency management for this project. 
---

# Auto Vulnerability Patch

This skill automates dependency vulnerability patching for the Azure App Configuration Kubernetes Provider project with GitHub source code. This skill describes how to create PullRequest, follow the exact steps to create branches, and select the right source and target branches while creating the PullRequest.

## When to Use This Skill

Use this skill when you need to:
- Update Go dependencies with security patches
- Manage vulnerability fixes in go.mod
- Create automated dependency update workflows
- Review and manage dependency update pull requests

## Branching Strategy and Naming Conventions

Create a new release branch for the further release, the branch name should be in the format `release/v<major>.<minor>.<patch>`. 

Read the current version from the `version.json` file in `main` branch, the branch name should correspond to the next patch version. For example, if the current version is `1.2.3`, the branch name should be `release/v1.2.4`. If this branch is already existing, no need to create a new.
 
Then need to create a feature branch from this release branch, the feature branch name should be in the format `auto-vuln-patch-<timestamp>`, e.g., `auto-vuln-patch-20240915-1230`. Switch to this new feature branch for the following steps.


The PullRequest should be created from the feature branch `auto-vuln-patch-<timestamp>` to the release branch `release/v<major>.<minor>.<patch>`. 

## Quick Start

### Trigger a Dependency Update

Run the following command on the feature branch to update all Go dependencies to their latest versions, including security patches:

```bash
go get -u ./...
go mod tidy
```

If the `go get` command returns any package requiring newer go versions, update to the required Go version in `go.mod` file. Also update the Go version in the pipeline YAML file if necessary.

### Update version.json

Update the `version.json` file to reflect the new version after the dependency updates. Increment the patch version by 1. For example, if the current version is `1.2.3`, update it to `1.2.4`.

### Commit and Push Changes

Commit and push the changes to the new feature branch `auto-vuln-patch-<timestamp>` created earlier. 


### Review the Pull Request

Each PR includes:
- Complete dependency change diff
- Test results
- Linter output
- Security considerations checklist

**Review Checklist:**
- [ ] Verify go.mod changes are expected
- [ ] Confirm no breaking changes
- [ ] Check all tests pass
- [ ] Review any security advisories
- [ ] Validate linter results
