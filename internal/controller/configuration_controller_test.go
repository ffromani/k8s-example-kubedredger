/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
)

const (
	confSnippet = "answer=42\n"
)

func NewFakeConfigurationReconciler() (*ConfigurationReconciler, string, func() error, error) {
	dir, err := os.MkdirTemp("", "kubedredger-ctrl-test")
	if err != nil {
		return nil, "", func() error { return nil }, err
	}
	GinkgoLogr.Info("created temporary directory", "path", dir)
	cleanup := func() error {
		return os.RemoveAll(dir)
	}
	rec := ConfigurationReconciler{
		Client:  k8sClient,
		Scheme:  scheme.Scheme,
		ConfMgr: configfile.NewManager(dir),
	}
	return &rec, dir, cleanup, nil
}

var _ = Describe("Configuration Controller", func() {
	var testNamespace *v1.Namespace

	Context("When reconciling a resource", func() {
		var cleanup func() error
		var reconciler *ConfigurationReconciler
		var configRoot string

		BeforeEach(func() {
			var err error
			reconciler, configRoot, cleanup, err = NewFakeConfigurationReconciler()
			Expect(err).ToNot(HaveOccurred())

			ctx := context.Background()
			Expect(err).ToNot(HaveOccurred())

			// see: https://book.kubebuilder.io/reference/envtest.html?highlight=envtest#namespace-usage-limitation
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "workshop-",
				},
			}
			Expect(reconciler.Client.Create(ctx, ns)).To(Succeed())
			testNamespace = ns
		})

		AfterEach(func() {
			// intentionally not try to delete namespaces.
			// see: https://book.kubebuilder.io/reference/envtest.html?highlight=envtest#namespace-usage-limitation
			Expect(cleanup()).To(Succeed())
		})

		When("handling the configuration", func() {
			It("creates the configuration from scratch", func(ctx context.Context) {

				conf := &workshopv1alpha1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace.Name,
						Name:      "test-create",
					},
					Spec: workshopv1alpha1.ConfigurationSpec{
						Filename: "foo.conf",
						Content:  "foo=bar\nbaz=42\n",
						Create:   true,
					},
				}
				Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
				DeferCleanup(func() {
					Expect(reconciler.Client.Delete(context.Background(), conf)).To(Succeed())
				})

				key := client.ObjectKeyFromObject(conf)
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				configPath := filepath.Join(configRoot, conf.Spec.Filename)
				_, err = os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

				data, err := os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				updatedConf := &workshopv1alpha1.Configuration{}
				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())
			})

			It("updates the configuration once created", func(ctx context.Context) {
				conf := &workshopv1alpha1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace.Name,
						Name:      "test-create",
					},
					Spec: workshopv1alpha1.ConfigurationSpec{
						Filename:   "bar2.conf",
						Content:    "foo=bar\n",
						Create:     true,
						Permission: ptr.To[uint32](0600),
					},
				}
				Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
				DeferCleanup(func() {
					Expect(reconciler.Client.Delete(context.Background(), conf)).To(Succeed())
				})

				key := client.ObjectKeyFromObject(conf)
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				configPath := filepath.Join(configRoot, conf.Spec.Filename)
				finfo, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)

				data, err := os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				updatedConf := &workshopv1alpha1.Configuration{}
				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())

				Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(conf), conf)).To(Succeed())
				conf.Spec.Create = false
				conf.Spec.Permission = nil
				conf.Spec.Content = confSnippet
				Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				finfo2, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo2.Mode())).To(Equal(uint32(0644)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0644)

				data, err = os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())
			})

			It("does not create the same configuration file twice", func(ctx context.Context) {
				origContent := "foo=bar\n"
				conf := &workshopv1alpha1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace.Name,
						Name:      "test-create",
					},
					Spec: workshopv1alpha1.ConfigurationSpec{
						Filename:   "foo5.conf",
						Content:    origContent,
						Create:     true,
						Permission: ptr.To[uint32](0600),
					},
				}
				Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
				DeferCleanup(func() {
					Expect(reconciler.Client.Delete(context.Background(), conf)).To(Succeed())
				})

				key := client.ObjectKeyFromObject(conf)
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				configPath := filepath.Join(configRoot, conf.Spec.Filename)
				finfo, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)

				data, err := os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				updatedConf := &workshopv1alpha1.Configuration{}
				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())

				Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(conf), conf)).To(Succeed())
				conf.Spec.Content = confSnippet
				Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				finfo2, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo2.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0600)

				data, err = os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(confSnippet), "configuration content doesn't match")

				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())
			})
			It("updates the configuration once created multiple times", func(ctx context.Context) {
				conf := &workshopv1alpha1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace.Name,
						Name:      "test-create",
					},
					Spec: workshopv1alpha1.ConfigurationSpec{
						Filename:   "quux.conf",
						Content:    "foo=bar\n",
						Create:     true,
						Permission: ptr.To[uint32](0600),
					},
				}

				Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
				DeferCleanup(func() {
					Expect(reconciler.Client.Delete(context.Background(), conf)).To(Succeed())
				})

				key := client.ObjectKeyFromObject(conf)
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				configPath := filepath.Join(configRoot, conf.Spec.Filename)
				finfo, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)

				data, err := os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				updatedConf := &workshopv1alpha1.Configuration{}
				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())

				Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(conf), conf)).To(Succeed())
				conf.Spec.Create = false
				conf.Spec.Permission = nil
				conf.Spec.Content = confSnippet
				Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				finfo2, err := os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo2.Mode())).To(Equal(uint32(0644)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0644)

				data, err = os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())

				Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(conf), conf)).To(Succeed())
				conf.Spec.Create = false
				conf.Spec.Permission = nil
				conf.Spec.Content = "#answer=42\nattempts=2\nverify=always\n"
				Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())

				finfo2, err = os.Stat(configPath)
				Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
				Expect(uint32(finfo2.Mode())).To(Equal(uint32(0644)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0644)

				data, err = os.ReadFile(configPath)
				Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
				Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

				Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
				Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())
			})
		})
	})
})

func verifyAvailableStatus(confStatus *workshopv1alpha1.ConfigurationStatus) error {
	if !confStatus.FileExists {
		return fmt.Errorf("cannot be available without file created")
	}
	if !isConditionEqual(confStatus.Conditions, ConditionAvailable, metav1.ConditionTrue) ||
		!isConditionEqual(confStatus.Conditions, ConditionProgressing, metav1.ConditionFalse) ||
		!isConditionEqual(confStatus.Conditions, ConditionDegraded, metav1.ConditionFalse) {
		return fmt.Errorf("unexpected status conditions: %#v", confStatus.Conditions)
	}
	return nil
}

func isConditionEqual(conds []metav1.Condition, condType string, condStatus metav1.ConditionStatus) bool {
	for _, cond := range conds {
		if cond.Type == condType {
			return cond.Status == condStatus
		}
	}
	return false
}
