package e2e

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golab.io/kubedredger/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestE2E(t *testing.T) {
	if testing.Short() {
		return
	}
	RegisterFailHandler(ginkgo.Fail)

	ginkgo.RunSpecs(t, "E2E Suite")
}

var cl client.Client

var _ = ginkgo.BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))
	var err error
	cl, err = newClient()
	Expect(err).NotTo(HaveOccurred())
})

var _ = ginkgo.AfterSuite(func() {

})

func newClient() (client.Client, error) {
	myScheme := runtime.NewScheme()

	if err := v1alpha1.AddToScheme(myScheme); err != nil {
		return nil, err
	}

	config := ctrl.GetConfigOrDie()
	cl, err := client.New(config, client.Options{
		Scheme: myScheme,
	})
	if err != nil {
		return nil, err
	}
	return cl, nil
}
