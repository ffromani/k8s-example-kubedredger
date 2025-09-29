package e2e

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golab.io/kubedredger/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = ginkgo.Describe("Configuration E2E", func() {
	var (
		ctx           context.Context
		configuration *v1alpha1.Configuration
		testNamespace = "k8s-example-kubedredger-system"
	)

	ginkgo.BeforeEach(func() {
		ctx = context.Background()

		configuration = &v1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: testNamespace,
			},
			Spec: v1alpha1.ConfigurationSpec{
				Content: "test content for e2e",
				Create:  true,
			},
		}
	})

	ginkgo.AfterEach(func() {
		if configuration != nil {
			_ = cl.Delete(ctx, configuration)

			ginkgo.By("ensuring configuration is removed")
			Eventually(func() bool {
				err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
				return err != nil
			}, time.Minute, time.Second).Should(BeTrue())
		}
	})

	ginkgo.It("should create configuration and verify status and file creation", func() {
		ginkgo.By("creating the configuration")
		Expect(cl.Create(ctx, configuration)).To(Succeed())

		ginkgo.By("waiting for the configuration to be processed")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.LastUpdated.Time.After(time.Time{})
		}, time.Minute, time.Second).Should(BeTrue())

		ginkgo.By("verifying the configuration status")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.FileExists
		}, time.Minute, time.Second).Should(BeTrue())

		Eventually(func() string {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return ""
			}
			return configuration.Status.Content
		}, time.Minute, time.Second).Should(Equal("test content for e2e"))

		ginkgo.By("verifying the file in the kind container is created")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(configuration), configuration)
			if err != nil {
				return false
			}
			return configuration.Status.FileExists
		}, time.Minute, time.Second).Should(BeTrue())

		ginkgo.By("verifying the file content in the kind container using docker")
		Eventually(func() string {
			cmd := exec.Command("docker", "exec", "k8s-example-kubedredger-kind-control-plane", "test", "-f", "/tmp/config")
			if cmd.Run() != nil {
				return "not found"
			}

			cmd = exec.Command("docker", "exec", "k8s-example-kubedredger-kind-control-plane", "cat", "/tmp/config")
			output, err := cmd.Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(output))
		}, time.Minute, time.Second).Should(Equal("test content for e2e"))
	})
})
