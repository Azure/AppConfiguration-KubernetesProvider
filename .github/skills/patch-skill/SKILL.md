---
name: auto-vulnerability-patch
description: Automatically update Go dependencies with security patches for Azure App Configuration Kubernetes Provider. Use when user mentions dependency updates, security patches, vulnerability fixes, or automated dependency management for this project. 
---

# Auto Vulnerability Patch

This skill automates dependency vulnerability patching for the Azure App Configuration Kubernetes Provider project with GitHub source code.

## When to Use This Skill

Use this skill when you need to:
- Update Go dependencies with security patches
- Manage vulnerability fixes in go.mod
- Create automated dependency update workflows
- Review and manage dependency update pull requests

## Quick Start

### Create new branch

Create and switch to a new branch for the dependency update, the branch name should be in the format `release/v<major>.<minor>.<patch>`.

Read the current version from the `version.json` file, the branch name should correspond to the next patch version. For example, if the current version is `1.2.3`, the branch name should be `release/v1.2.4`.

### Trigger a Dependency Update

Run the following command in your local repository:

```bash
go get -u ./...
go mod tidy
```

If the `go get` command returns any package requiring newer go versions, update to the required Go version in `go.mod` file. Also update the Go version in the pipeline YAML file if necessary.

### Commit and Push Changes

Commit and push the changes to the new branch created earlier

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
