---
name: patch-skill
description: Automatically update Go dependencies with security patches for Azure App Configuration Kubernetes Provider. Use when the user mentions dependency updates, security patches, vulnerability fixes, go.mod updates, or automated dependency management for this project.
metadata:
  author: azure-app-configuration
  version: "1.0"
---

# Auto Vulnerability Patch

This skill automates dependency vulnerability patching for the Azure App Configuration Kubernetes Provider Go project.

## Step-by-step Instructions

### Step 1 — Determine the next patch version

Read `version.json` on the `main` branch to get the current version. Compute the next patch version by incrementing the patch number by 1.

**Example:** if `version.json` contains `1.2.3`, the next version is `1.2.4`.

### Step 2 — Create branches

1. Create (or reuse) a release branch named `release/v<major>.<minor>.<patch>` using the next version computed above. If the branch already exists, skip creation.
2. From that release branch, create a feature branch named `auto-vuln-patch-<timestamp>` (e.g., `auto-vuln-patch-20240915-1230`).
3. Switch to the feature branch for all subsequent steps.

### Step 3 — Update dependencies

Run in the repository root:

```bash
go get -u ./...
go mod tidy
```

**Edge case — Go version bump:** If `go get` reports that a package requires a newer Go version, update the `go` directive in `go.mod` to the required version. Also update the Go version in any pipeline YAML files (e.g., CI/CD workflow files) to match.

### Step 4 — Handle breaking changes (if any)

After updating dependencies, run `go build ./...` to check for compilation errors. If a dependency update introduces breaking changes (e.g., removed or renamed APIs, changed function signatures, modified types), resolve them as follows:

1. **Identify the breaking package and versions.** Check the `go.mod` diff to find which package was upgraded and from which version to which version.
2. **Consult the package's release notes or changelog.** Look up the release notes, changelog, or migration guide for the new version on the package's repository (e.g., GitHub releases page). Identify what changed and what the recommended migration path is.
3. **Apply the migration.** Update the project's source code to conform to the new API according to the package's official guidance. Common breaking changes include:
   - Renamed or removed functions/methods — replace calls with the new names or alternatives documented in the release notes.
   - Changed function signatures (added/removed/reordered parameters) — update all call sites to match the new signature.
   - Changed types or interfaces — update type assertions, interface implementations, and struct usages accordingly.
   - Removed or replaced packages — switch imports and usages to the successor package as indicated in the migration guide.
4. **Verify the fix.** Run `go build ./...` again to confirm the code compiles. Then run `go test ./...` to confirm no regressions.
5. **Document the change.** Note any breaking-change migrations in the PR description so reviewers understand why source code was modified beyond `go.mod` / `go.sum`.

**Example:** Package `example.com/foo` upgraded from `v2.3.0` to `v3.0.0`. The release notes state that `foo.DoWork(ctx, input)` was renamed to `foo.Execute(ctx, input, opts)` with a new required `opts` parameter. Update every call site from `foo.DoWork(ctx, input)` to `foo.Execute(ctx, input, foo.DefaultOptions())`.

### Step 5 — Bump `version.json`

Update `version.json` to the next patch version determined in Step 1.

### Step 6 — Commit, push, and open a pull request

1. Commit all changes (at minimum `go.mod`, `go.sum`, and `version.json`).
2. Push the feature branch.
3. Open a pull request from the feature branch into the release branch.

### Step 7 — Review the pull request

Each PR should include:
- Complete dependency change diff
- Test results
- Linter output
- Security considerations checklist

**Review Checklist:**
- [ ] Verify `go.mod` / `go.sum` changes are expected
- [ ] Confirm no breaking API changes
- [ ] Check all tests pass (`go test ./...`)
- [ ] Review any security advisories for updated packages
- [ ] Validate linter results

## Common Edge Cases

- **Breaking changes in dependencies:** If a dependency update introduces breaking API changes that cannot be resolved automatically, document the error and the relevant release notes in the PR description and flag the PR for manual maintainer intervention.
- **No updates available:** If `go get -u ./...` produces no changes, report that all dependencies are already up to date and stop.
- **Test failures after update:** If `go test ./...` fails after updating, note the failing packages in the PR description so a maintainer can investigate.
- **Merge conflicts on the release branch:** If the release branch has diverged, rebase the feature branch onto the latest release branch before opening the PR.
