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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	"golab.io/kubedredger/internal/nodelabel"
)

const (
	testNodeName = "test-host"

	confSnippet = "answer=42\n"
)

func NewFakeConfigurationReconciler(initObjects ...runtime.Object) (*ConfigurationReconciler, string, func() error, error) {
	dir, err := os.MkdirTemp("", "kubedredger-ctrl-test")
	if err != nil {
		return nil, "", func() error { return nil }, err
	}
	GinkgoLogr.Info("created temporary directory", "path", dir)
	configFile := filepath.Join(dir, "config.conf")
	cleanup := func() error {
		return os.RemoveAll(dir)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(&workshopv1alpha1.Configuration{}).WithRuntimeObjects(initObjects...).Build()
	rec := ConfigurationReconciler{
		Client:   fakeClient,
		Scheme:   scheme.Scheme,
		ConfMgr:  configfile.NewManager(configFile),
		Labeller: nodelabel.NewManager(testNodeName, fakeClient),
	}
	return &rec, configFile, cleanup, nil
}

var _ = Describe("Configuration Controller", func() {
	var testNode *v1.Node

	Context("When reconciling a resource", func() {
		var cleanup func() error
		var reconciler *ConfigurationReconciler
		var configPath string

		BeforeEach(func() {
			var err error
			reconciler, configPath, cleanup, err = NewFakeConfigurationReconciler()
			Expect(err).ToNot(HaveOccurred())

			testNode = &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNodeName,
				},
			}
		})

		AfterEach(func() {
			Expect(cleanup()).To(Succeed())
		})

		It("creates the configuration from scratch", func(ctx context.Context) {
			Expect(reconciler.Client.Create(ctx, testNode)).To(Succeed())

			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    "foo=bar\nbaz=42\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			key := client.ObjectKeyFromObject(conf)
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			finfo, err := os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)
			data, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

			updatedConf := &workshopv1alpha1.Configuration{}
			Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
			Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())
		})

		It("creates the configuration from scratch, but stays in progress if can't update the k8s node object", func(ctx context.Context) {
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    "foo=bar\nbaz=42\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			key := client.ObjectKeyFromObject(conf)
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).To(HaveOccurred())

			updatedConf := &workshopv1alpha1.Configuration{}
			Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
			Expect(verifyProgressingStatus(&updatedConf.Status)).To(Succeed())

			// still should have created the file!
			finfo, err := os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)
			data, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")
		})

		It("creates the configuration from scratch, but stays in progress if can't update the k8s node object, then succeeds", func(ctx context.Context) {
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    "foo=bar\nbaz=42\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			key := client.ObjectKeyFromObject(conf)
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).To(HaveOccurred())

			updatedConf := &workshopv1alpha1.Configuration{}
			Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
			Expect(verifyProgressingStatus(&updatedConf.Status)).To(Succeed())

			// still should have created the file!
			finfo, err := os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)
			data, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

			Expect(reconciler.Client.Create(ctx, testNode)).To(Succeed())

			var updatedNode v1.Node
			Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(testNode), &updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).ToNot(HaveKey(nodelabel.ContentHash))

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).ToNot(HaveOccurred())
			finfo, err = os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)
			data, err = os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

			Expect(reconciler.Client.Get(ctx, key, updatedConf)).To(Succeed())
			Expect(verifyAvailableStatus(&updatedConf.Status)).To(Succeed())

			Expect(verifyNodeIsLabelled(ctx, reconciler.Client, testNode.Name)).To(Succeed())
		})

		It("updates the configuration once created", func(ctx context.Context) {
			Expect(reconciler.Client.Create(ctx, testNode)).To(Succeed())

			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    "foo=bar\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

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
			Expect(verifyNodeIsLabelled(ctx, reconciler.Client, testNode.Name)).To(Succeed())
		})

		It("does not create the same configuration file twice", func(ctx context.Context) {
			Expect(reconciler.Client.Create(ctx, testNode)).To(Succeed())

			origContent := "foo=bar\n"
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    origContent,
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

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
			Expect(reconciler.Client.Create(ctx, testNode)).To(Succeed())

			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Content:    "foo=bar\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

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
			Expect(verifyNodeIsLabelled(ctx, reconciler.Client, testNode.Name)).To(Succeed())

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
			Expect(verifyNodeIsLabelled(ctx, reconciler.Client, testNode.Name)).To(Succeed())
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

func verifyProgressingStatus(confStatus *workshopv1alpha1.ConfigurationStatus) error {
	if !isConditionEqual(confStatus.Conditions, ConditionAvailable, metav1.ConditionFalse) ||
		!isConditionEqual(confStatus.Conditions, ConditionProgressing, metav1.ConditionTrue) ||
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
func verifyNodeIsLabelled(ctx context.Context, cli client.Client, nodeName string) error {
	var updatedNode v1.Node
	if err := cli.Get(ctx, client.ObjectKey{Name: nodeName}, &updatedNode); err != nil {
		return err
	}
	val, ok := updatedNode.Labels[nodelabel.ContentHash]
	if !ok {
		return errors.New("missing contenthash key")
	}
	if val == "" {
		return errors.New("empty contenthash value")
	}
	return nil
}
