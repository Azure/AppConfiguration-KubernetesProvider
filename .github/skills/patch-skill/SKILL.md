---
name: auto-vulnerability-patch
description: Guide for updating Go dependencies with security patches for Azure App Configuration Kubernetes Provider. Use when user mentions dependency updates, security patches, vulnerability fixes, or dependency management for this project. 
---

# Dependency Vulnerability Patching

This skill provides guidance for dependency vulnerability patching for the Azure App Configuration Kubernetes Provider project.

## When to Use This Skill

Use this skill when you need to:
- Update Go dependencies with security patches
- Manage vulnerability fixes in go.mod
- Review and manage dependency update pull requests

## Quick Start

### Update Dependencies Locally

Run the following commands in your local repository:

```bash
go get -u ./...
go mod tidy
```

If the `go get` command returns any package requiring newer go versions, update to the required Go version in `go.mod` file. Also update the Go version in the pipeline YAML file if necessary.

### Commit and Push Changes

Commit and push the changes

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

### Merge and Deploy

After approval and merge to main:
1. ADO pipeline triggers automatically
2. Full build executes
3. New version releases
