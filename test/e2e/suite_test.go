//go:build e2e

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package e2e

import (
	acpv1 "azappconfig/provider/api/v1"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const runLabel = "azconfig.io/e2e-run"

type liveConfiguration struct {
	endpoint       string
	kubeContext    string
	namespace      string
	serviceAccount string
	runID          string
}

var (
	config      liveConfiguration
	kubeClient  client.Client
	azureClient *azappconfig.Client
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Live App Configuration Suite")
}

func loadConfiguration() (liveConfiguration, error) {
	var result liveConfiguration
	for _, field := range []struct {
		name   string
		target *string
	}{
		{"E2E_APPCONFIG_ENDPOINT", &result.endpoint},
		{"E2E_KUBE_CONTEXT", &result.kubeContext},
		{"E2E_NAMESPACE", &result.namespace},
		{"E2E_SERVICE_ACCOUNT", &result.serviceAccount},
	} {
		*field.target = strings.TrimSpace(os.Getenv(field.name))
		if *field.target == "" {
			return result, fmt.Errorf("%s is required for live tests", field.name)
		}
	}

	endpoint, err := url.Parse(result.endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		(endpoint.Path != "" && endpoint.Path != "/") {
		return result, fmt.Errorf("E2E_APPCONFIG_ENDPOINT must be an HTTPS store endpoint without credentials, query, or path")
	}
	result.endpoint = strings.TrimSuffix(result.endpoint, "/")
	if problems := validation.IsDNS1123Label(result.namespace); len(problems) != 0 {
		return result, fmt.Errorf("invalid E2E_NAMESPACE: %s", strings.Join(problems, "; "))
	}
	if problems := validation.IsDNS1123Subdomain(result.serviceAccount); len(problems) != 0 {
		return result, fmt.Errorf("invalid E2E_SERVICE_ACCOUNT: %s", strings.Join(problems, "; "))
	}
	result.runID = os.Getenv("E2E_RUN_ID")
	if result.runID == "" {
		result.runID = uuid.NewString()
	}
	if problems := validation.IsValidLabelValue(result.runID); len(problems) != 0 {
		return result, fmt.Errorf("invalid E2E_RUN_ID: %s", strings.Join(problems, "; "))
	}
	return result, nil
}

var _ = BeforeSuite(func(ctx SpecContext) {
	var err error
	config, err = loadConfiguration()
	Expect(err).NotTo(HaveOccurred())

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: config.kubeContext},
	)
	restConfig, err := loader.ClientConfig()
	Expect(err).NotTo(HaveOccurred())
	restConfig.Timeout = 30 * time.Second

	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(acpv1.AddToScheme(scheme)).To(Succeed())
	kubeClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	account := &corev1.ServiceAccount{}
	Expect(kubeClient.Get(ctx, client.ObjectKey{
		Namespace: config.namespace,
		Name:      config.serviceAccount,
	}, account)).To(Succeed(), "the tests reuse, and never create or modify, this service account")
	for _, annotation := range []string{
		"azure.workload.identity/client-id",
		"azure.workload.identity/tenant-id",
	} {
		_, err := uuid.Parse(account.Annotations[annotation])
		Expect(err).NotTo(HaveOccurred(), "service account must have a valid %s annotation", annotation)
	}

	credential, err := azidentity.NewAzureCLICredential(nil)
	Expect(err).NotTo(HaveOccurred())
	azureClient, err = azappconfig.NewClient(config.endpoint, credential, nil)
	Expect(err).NotTo(HaveOccurred())
	fmt.Fprintf(GinkgoWriter, "Run %s: context=%s namespace=%s serviceAccount=%s endpoint=%s\n",
		config.runID, config.kubeContext, config.namespace, config.serviceAccount, config.endpoint)
}, NodeTimeout(time.Minute))

func TestLiveConfiguration(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{"missing endpoint", "E2E_APPCONFIG_ENDPOINT", ""},
		{"missing context", "E2E_KUBE_CONTEXT", ""},
		{"missing namespace", "E2E_NAMESPACE", ""},
		{"missing service account", "E2E_SERVICE_ACCOUNT", ""},
		{"HTTP endpoint", "E2E_APPCONFIG_ENDPOINT", "http://example.azconfig.io"},
		{"endpoint credentials", "E2E_APPCONFIG_ENDPOINT", "https://user:password@example.azconfig.io"},
		{"endpoint path", "E2E_APPCONFIG_ENDPOINT", "https://example.azconfig.io/kv"},
		{"invalid namespace", "E2E_NAMESPACE", "not/a/namespace"},
		{"invalid service account", "E2E_SERVICE_ACCOUNT", "not/an/account"},
		{"invalid run label", "E2E_RUN_ID", "not/a/label"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(test.field, test.value)
			if _, err := loadConfiguration(); err == nil {
				t.Fatal("expected invalid live-test configuration to fail")
			}
		})
	}
	t.Run("valid configuration generates a run ID", func(t *testing.T) {
		setValidEnvironment(t)
		cfg, err := loadConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := uuid.Parse(cfg.runID); err != nil {
			t.Fatalf("expected a unique run ID: %v", err)
		}
	})
	t.Run("trailing endpoint slash is normalized", func(t *testing.T) {
		setValidEnvironment(t)
		t.Setenv("E2E_APPCONFIG_ENDPOINT", "https://example.azconfig.io/")
		cfg, err := loadConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.endpoint != "https://example.azconfig.io" {
			t.Fatalf("unexpected normalized endpoint: %s", cfg.endpoint)
		}
	})
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("E2E_APPCONFIG_ENDPOINT", "https://example.azconfig.io")
	t.Setenv("E2E_KUBE_CONTEXT", "explicit-test-context")
	t.Setenv("E2E_NAMESPACE", "default")
	t.Setenv("E2E_SERVICE_ACCOUNT", "k8s-provider-sa")
	t.Setenv("E2E_RUN_ID", "")
}
