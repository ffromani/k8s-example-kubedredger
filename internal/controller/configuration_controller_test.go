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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
)

func NewFakeConfigurationReconciler(initObjects ...runtime.Object) (*ConfigurationReconciler, func() error, error) {
	dir, err := os.MkdirTemp("", "kubedredger-ctrl-test")
	if err != nil {
		return nil, func() error { return nil }, err
	}
	GinkgoLogr.Info("created temporary directory", "path", dir)
	cleanup := func() error {
		return os.RemoveAll(dir)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(&workshopv1alpha1.Configuration{}).WithRuntimeObjects(initObjects...).Build()
	rec := ConfigurationReconciler{
		Client:  fakeClient,
		Scheme:  scheme.Scheme,
		ConfMgr: configfile.NewManager(dir),
	}
	return &rec, cleanup, nil
}

var _ = Describe("Configuration Controller", func() {
	Context("When reconciling a resource", func() {
		var cleanup func() error
		var reconciler *ConfigurationReconciler

		BeforeEach(func() {
			var err error
			reconciler, cleanup, err = NewFakeConfigurationReconciler()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(cleanup()).To(Succeed())
		})

		It("creates the configuration from scratch", func(ctx context.Context) {
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Path:       "foobar.conf",
					Content:    "foo=bar\nbaz=42\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			fullPath := filepath.Join(reconciler.ConfMgr.GetConfigurationRoot(), conf.Spec.Path)
			finfo, err := os.Stat(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")

			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)
			data, err := os.ReadFile(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")
		})

		It("updates the configuration once created", func(ctx context.Context) {
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Path:       "foobar.conf",
					Content:    "foo=bar\n",
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			fullPath := filepath.Join(reconciler.ConfMgr.GetConfigurationRoot(), conf.Spec.Path)

			finfo, err := os.Stat(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)

			data, err := os.ReadFile(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

			conf.Spec.Create = false
			conf.Spec.Permission = nil
			conf.Spec.Content = "answer=42\n"
			Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			finfo2, err := os.Stat(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
			Expect(uint32(finfo2.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0600)

			data, err = os.ReadFile(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")
		})

		It("does not create the same configuration file twice", func(ctx context.Context) {
			origContent := "foo=bar\n"
			conf := &workshopv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-create",
				},
				Spec: workshopv1alpha1.ConfigurationSpec{
					Path:       "foobar.conf",
					Content:    origContent,
					Create:     true,
					Permission: ptr.To[uint32](0600),
				},
			}
			key := client.ObjectKeyFromObject(conf)
			Expect(reconciler.Client.Create(ctx, conf)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			fullPath := filepath.Join(reconciler.ConfMgr.GetConfigurationRoot(), conf.Spec.Path)

			finfo, err := os.Stat(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
			Expect(uint32(finfo.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo.Mode(), 0600)

			data, err := os.ReadFile(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(conf.Spec.Content), "configuration content doesn't match")

			conf.Spec.Content = "answer=42\n"
			Expect(reconciler.Client.Update(ctx, conf)).To(Succeed())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).To(HaveOccurred())

			finfo2, err := os.Stat(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error Stat()ing configuration file")
			Expect(uint32(finfo2.Mode())).To(Equal(uint32(0600)), "error checking permissions, got %o expected %o", finfo2.Mode(), 0600)

			data, err = os.ReadFile(fullPath)
			Expect(err).NotTo(HaveOccurred(), "error reading configuration file content")
			Expect(string(data)).To(Equal(origContent), "configuration content doesn't match")
		})
	})
})
