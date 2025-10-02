package e2e

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"golab.io/kubedredger/api/v1alpha1"
)

var _ = ginkgo.Describe("Configuration E2E", func() {
	var (
		configuration *v1alpha1.Configuration
		testNamespace string
		confRoot      string
		confName      string
	)

	ginkgo.BeforeEach(func() {
		testNamespace = "golab-kubedredger"
		confRoot = "/tmp/config.d"
		confName = "kubedredger.conf"

		configuration = &v1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: testNamespace,
			},
			Spec: v1alpha1.ConfigurationSpec{
				Filename: confName,
				Content:  "test content for e2e",
				Create:   true,
			},
		}
	})

	ginkgo.AfterEach(func() {
		ctx := context.Background()

		if configuration == nil {
			return // nothing to do
		}
		_ = cl.Delete(ctx, configuration)

		ginkgo.By("ensuring configuration is removed")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			return apierrors.IsNotFound(err)
		}, time.Minute, time.Second).Should(BeTrue())
	})

	ginkgo.It("should create configuration and verify status and file creation", func() {
		ctx := context.Background()

		ginkgo.By("creating the configuration")
		Expect(cl.Create(ctx, configuration)).To(Succeed())

		ginkgo.By("waiting for the configuration to be processed")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.LastUpdated.Time.After(time.Time{})
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeTrue())

		ginkgo.By("verifying the configuration status")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.FileExists
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeTrue())

		Eventually(func() string {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return ""
			}
			return configuration.Status.Content
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Equal("test content for e2e"))

		ginkgo.By("verifying the file in the kind container is created")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.FileExists
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeTrue())

		confPath := filepath.Join(confRoot, confName)
		ginkgo.By("verifying the file content in the kind container using docker: " + confPath)
		Eventually(func() string {
			cmd := exec.Command("docker", "exec", "kubedredger-kind-control-plane", "cat", confPath)
			output, err := cmd.Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(output))
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Equal("test content for e2e"))
	})
})
