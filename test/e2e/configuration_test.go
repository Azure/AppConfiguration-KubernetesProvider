//go:build e2e

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package e2e

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type settingID struct {
	key   string
	label string
}

type fixture struct {
	name     string
	prefix   string
	settings []settingID
	provider *acpv1.AzureAppConfigurationProvider
}

var _ = Describe("Live configuration projection", Serial, func() {
	var f *fixture

	BeforeEach(func() {
		name := "acp-e2e-" + uuid.NewString()
		f = &fixture{
			name:   name,
			prefix: "e2e/" + config.runID + "/" + name + "/",
		}
		fmt.Fprintf(GinkgoWriter, "Fixture: provider=%s/%s prefix=%s\n", config.namespace, f.name, f.prefix)
		DeferCleanup(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				f.logDiagnostics(ctx)
			}
			Expect(f.cleanup(ctx)).To(Succeed(), "cleanup must remove only this fixture's resources")
		}, NodeTimeout(2*time.Minute))
	})

	It("creates a ConfigMap with exactly the selected initial values", func(ctx SpecContext) {
		f.addSetting(ctx, "Message", "baseline", "v1")
		f.addSetting(ctx, "Color", "baseline", "blue")
		f.addSetting(ctx, "Message", "other", "not-selected")
		f.createProvider(ctx, "baseline", false)

		f.waitForData(ctx, map[string]string{"Message": "v1", "Color": "blue"})
	}, SpecTimeout(4*time.Minute))

	It("refreshes the same ConfigMap after an Azure value changes", func(ctx SpecContext) {
		f.addSetting(ctx, "Message", "baseline", "v1")
		f.createProvider(ctx, "baseline", true)
		initial := f.waitForData(ctx, map[string]string{"Message": "v1"})
		generation := f.provider.Generation

		current, err := azureClient.GetSetting(ctx, f.prefix+"Message", &azappconfig.GetSettingOptions{
			Label: to.Ptr("baseline"),
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = azureClient.SetSetting(ctx, f.prefix+"Message", to.Ptr("v2"), &azappconfig.SetSettingOptions{
			Label:           to.Ptr("baseline"),
			Tags:            current.Tags,
			OnlyIfUnchanged: current.ETag,
		})
		Expect(err).NotTo(HaveOccurred())

		updated := f.waitForData(ctx, map[string]string{"Message": "v2"})
		Expect(updated.UID).To(Equal(initial.UID), "refresh should update rather than replace the ConfigMap")
		Expect(kubeClient.Get(ctx, client.ObjectKeyFromObject(f.provider), f.provider)).To(Succeed())
		Expect(f.provider.Generation).To(Equal(generation), "refresh must not require a provider spec update")
	}, SpecTimeout(4*time.Minute))

	It("reconciles a selector change and removes values from the previous selection", func(ctx SpecContext) {
		f.addSetting(ctx, "Message", "blue", "blue-value")
		f.addSetting(ctx, "BlueOnly", "blue", "blue-only")
		f.addSetting(ctx, "Message", "green", "green-value")
		f.addSetting(ctx, "GreenOnly", "green", "green-only")
		f.createProvider(ctx, "blue", false)
		initial := f.waitForData(ctx, map[string]string{"Message": "blue-value", "BlueOnly": "blue-only"})

		original := f.provider.DeepCopy()
		f.provider.Spec.Configuration.Selectors[0].LabelFilter = to.Ptr("green")
		Expect(kubeClient.Patch(ctx, f.provider, client.MergeFrom(original))).To(Succeed())

		updated := f.waitForData(ctx, map[string]string{"Message": "green-value", "GreenOnly": "green-only"})
		Expect(updated.UID).To(Equal(initial.UID))
	}, SpecTimeout(4*time.Minute))
})

func (f *fixture) addSetting(ctx context.Context, key, label, value string) {
	id := settingID{key: f.prefix + key, label: label}
	// Record intent before the request so cleanup also covers a lost success response.
	f.settings = append(f.settings, id)
	_, err := azureClient.AddSetting(ctx, id.key, &value, &azappconfig.AddSettingOptions{
		Label: &label,
		Tags:  map[string]*string{"e2e-run": &config.runID, "e2e-fixture": &f.name},
	})
	Expect(err).NotTo(HaveOccurred())
}

func (f *fixture) createProvider(ctx context.Context, label string, refresh bool) {
	f.provider = &acpv1.AzureAppConfigurationProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.name,
			Namespace: config.namespace,
			Labels:    map[string]string{runLabel: config.runID},
		},
		Spec: acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: to.Ptr(config.endpoint),
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: f.name,
			},
			Auth: &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ServiceAccountName: to.Ptr(config.serviceAccount),
				},
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{{
					KeyFilter:   to.Ptr(f.prefix + "*"),
					LabelFilter: &label,
				}},
				TrimKeyPrefixes: []string{f.prefix},
			},
		},
	}
	if refresh {
		f.provider.Spec.Configuration.Refresh = &acpv1.DynamicConfigurationRefreshParameters{
			Enabled:  true,
			Interval: "10s",
		}
	}
	Expect(kubeClient.Create(ctx, f.provider)).To(Succeed())
}

func (f *fixture) waitForData(ctx context.Context, expected map[string]string) corev1.ConfigMap {
	var result corev1.ConfigMap
	Eventually(func() (map[string]string, error) {
		err := kubeClient.Get(ctx, client.ObjectKey{Namespace: config.namespace, Name: f.name}, &result)
		return result.Data, err
	}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Equal(expected))
	Expect(metav1.IsControlledBy(&result, f.provider)).To(BeTrue(), "the tested provider must own the output")
	return result
}

func (f *fixture) logDiagnostics(ctx context.Context) {
	provider := &acpv1.AzureAppConfigurationProvider{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: config.namespace, Name: f.name}, provider); err != nil {
		fmt.Fprintf(GinkgoWriter, "Cannot read provider %s status: %v\n", f.name, err)
	} else {
		fmt.Fprintf(GinkgoWriter, "Provider %s status: %+v\n", f.name, provider.Status)
	}
	events := &corev1.EventList{}
	if err := kubeClient.List(ctx, events, client.InNamespace(config.namespace),
		client.MatchingFields{"involvedObject.name": f.name}); err != nil {
		fmt.Fprintf(GinkgoWriter, "Cannot read fixture events: %v\n", err)
	} else {
		for _, event := range events.Items {
			fmt.Fprintf(GinkgoWriter, "Event %s: %s\n", event.Reason, event.Message)
		}
	}
}

func (f *fixture) cleanup(ctx context.Context) error {
	var failures []error
	for _, object := range []client.Object{
		&acpv1.AzureAppConfigurationProvider{ObjectMeta: metav1.ObjectMeta{Name: f.name, Namespace: config.namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: f.name, Namespace: config.namespace}},
	} {
		if err := deleteOwnedObject(ctx, object); err != nil {
			failures = append(failures, err)
		}
	}
	for _, id := range f.settings {
		current, err := azureClient.GetSetting(ctx, id.key, &azappconfig.GetSettingOptions{Label: &id.label})
		if isAzureNotFound(err) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("read setting %s [%s] for cleanup: %w", id.key, id.label, err))
			continue
		}
		runTag, fixtureTag := current.Tags["e2e-run"], current.Tags["e2e-fixture"]
		if runTag == nil || *runTag != config.runID || fixtureTag == nil || *fixtureTag != f.name {
			failures = append(failures, fmt.Errorf("refusing to delete setting %s [%s]: ownership tags do not match", id.key, id.label))
			continue
		}
		_, err = azureClient.DeleteSetting(ctx, id.key, &azappconfig.DeleteSettingOptions{
			Label: &id.label, OnlyIfUnchanged: current.ETag,
		})
		if err != nil && !isAzureNotFound(err) {
			failures = append(failures, fmt.Errorf("delete setting %s [%s]: %w", id.key, id.label, err))
		}
	}
	return errors.Join(failures...)
}

func deleteOwnedObject(ctx context.Context, object client.Object) error {
	key := client.ObjectKeyFromObject(object)
	if err := kubeClient.Get(ctx, key, object); err != nil {
		return client.IgnoreNotFound(err)
	}
	if object.GetLabels()[runLabel] != config.runID {
		return fmt.Errorf("refusing to delete %T %s: run label does not match", object, key)
	}
	uid := object.GetUID()
	if err := kubeClient.Delete(ctx, object, &client.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid},
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %T %s: %w", object, key, err)
	}
	return wait.PollUntilContextTimeout(ctx, time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		err := kubeClient.Get(ctx, key, object)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if object.GetUID() != uid {
			return false, fmt.Errorf("%T %s was replaced during cleanup", object, key)
		}
		return false, nil
	})
}

func isAzureNotFound(err error) bool {
	var responseError *azcore.ResponseError
	return errors.As(err, &responseError) && responseError.StatusCode == http.StatusNotFound
}
