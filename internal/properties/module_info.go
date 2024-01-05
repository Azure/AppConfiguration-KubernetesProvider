// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package properties

var (
	// Just set the default value here, if running or debugging the project locally, it will use this value.
	// It would be overwritten by the version value in version.json using the '-ldflags' option at build time.
	// See Dockerfile for detail to know how it be overwritten.
	ModuleVersion string = "develop"
	ModuleName    string = "azcfg-k8s"
)
