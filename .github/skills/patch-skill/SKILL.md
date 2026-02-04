---
name: auto-vulnerability-patch
description: Automatically update Go dependencies with security patches for Azure App Configuration Kubernetes Provider. Use when user mentions dependency updates, security patches, vulnerability fixes, or automated dependency management for this project. Works with GitHub source code and Azure DevOps build pipelines.
---

# Auto Vulnerability Patch

This skill automates dependency vulnerability patching for the Azure App Configuration Kubernetes Provider project with GitHub source code and Azure DevOps (ADO) build/release pipelines.

## When to Use This Skill

Use this skill when you need to:
- Update Go dependencies with security patches
- Manage vulnerability fixes in go.mod
- Create automated dependency update workflows
- Trigger Azure DevOps builds from GitHub
- Review and manage dependency update pull requests

## Quick Start

### Trigger a Dependency Update

1. Navigate to GitHub Actions in your repository
2. Select the "Update Dependencies" workflow
3. Click "Run workflow"
4. Choose update type:
   - **patch**: Security and bug fixes only (recommended for production)
   - **minor**: Patch and minor version updates
   - **all**: All updates including major versions (requires careful testing)

The workflow will:
- Update dependencies in go.mod and go.sum
- Run automated tests and linters
- Create a pull request for manual review

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

## Configuration Setup

### Required GitHub Secrets

Add these secrets in: `Settings → Secrets and variables → Actions`

```bash
ADO_ORGANIZATION    # Your Azure DevOps organization name
ADO_PROJECT         # Azure DevOps project name
ADO_PIPELINE_ID     # Pipeline ID (numeric)
ADO_PAT             # Personal Access Token
```

### Get Pipeline ID

Find your ADO Pipeline ID:

```bash
# Navigate to your pipeline in ADO
# URL format: https://dev.azure.com/{org}/{project}/_build?definitionId={ID}
# The ID is the number in the URL
```

### Create ADO Personal Access Token

Required permissions:
- Build (Read & Execute)
- Code (Read)

Steps:
1. Log into Azure DevOps
2. User Settings → Personal Access Tokens
3. New Token → Select required scopes
4. Copy token and add to GitHub Secrets

## Workflow Architecture

```
Manual Trigger
      ↓
Update Dependencies (GitHub Actions)
      ↓
Run Tests & Linters
      ↓
Create Pull Request (requires human review)
      ↓
Code Review & Approval
      ↓
Merge to Main Branch
      ↓
Trigger ADO Pipeline (automatic)
      ↓
Build & Release
```

## Files Included

This skill created:
- `.github/workflows/update-dependencies.yml` - Main dependency update workflow
- `.github/workflows/trigger-ado-build.yml` - ADO pipeline trigger automation
- `.github/dependabot.yml` - Dependabot configuration for automatic security alerts

## Best Practices

1. **Regular Updates**: Run dependency updates weekly or monthly
2. **Security First**: Prioritize security patches using "patch" update mode
3. **Incremental Changes**: Use patch mode for safer, smaller updates
4. **Test Thoroughly**: Always verify test results before merging
5. **Document Changes**: Add notes to PRs about significant version changes
6. **Monitor Releases**: Watch ADO build after merging

## Troubleshooting

### Pull Request Creation Fails

Check:
- GitHub Actions permissions enabled
- `contents: write` permission granted
- `pull-requests: write` permission granted
- No branch protection conflicts

### ADO Pipeline Not Triggering

Verify:
- ADO_PAT is valid and not expired
- ADO_PIPELINE_ID is correct (numeric ID)
- PAT has Build (Read & Execute) permissions
- Organization and Project names are exact matches

### Tests Fail After Update

Actions:
- Review test logs in Actions tab
- Check for breaking changes in dependency changelogs
- Consider updating dependencies incrementally
- May require code changes to fix incompatibilities

### Network or Permission Errors

Check:
- GitHub token permissions in workflow
- ADO PAT expiration date
- Organization access restrictions
- Firewall or network policies

## Advanced Usage

### Custom Update Patterns

Modify `.github/workflows/update-dependencies.yml` to customize:
- Test commands
- Linter configurations
- PR templates
- Notification recipients

### Scheduled Updates

Add to workflow (uncomment in `update-dependencies.yml`):

```yaml
on:
  schedule:
    - cron: '0 9 * * 1'  # Weekly on Monday 9 AM
  workflow_dispatch:
```

### Multiple Update Strategies

Create separate workflows for:
- Security-only updates (automatic merge)
- Minor updates (requires review)
- Major updates (requires thorough testing)

## Examples

### Example 1: Security Patch Update

```bash
# Trigger: Manual workflow dispatch with "patch" mode
# Result: Updates Azure SDK from 1.20.0 to 1.20.1
# Action: PR created, tests pass, ready for review
```

### Example 2: Minor Version Update

```bash
# Trigger: Manual workflow dispatch with "minor" mode
# Result: Updates k8s.io/client-go from 0.34.3 to 0.35.0
# Action: PR created with breaking changes warning
```

### Example 3: Post-Merge ADO Trigger

```bash
# Trigger: PR merged to main branch
# Result: ADO pipeline receives webhook
# Action: Build starts automatically with updated dependencies
```

## Security Considerations

- Only use trusted dependency sources
- Review all dependency changes manually
- Verify checksums in go.sum
- Monitor security advisories
- Keep PAT secure and rotate regularly
- Use minimal required PAT permissions
- Enable branch protection on main

## Related Resources

- [Go Modules Documentation](https://go.dev/ref/mod)
- [GitHub Actions Workflow Syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
- [Azure DevOps REST API](https://learn.microsoft.com/en-us/rest/api/azure/devops/)
- [Dependabot Configuration](https://docs.github.com/en/code-security/dependabot)

## Support

For issues or questions:
- Create a GitHub Issue in the repository
- Contact the project maintenance team
- Review GitHub Actions logs for detailed error messages
