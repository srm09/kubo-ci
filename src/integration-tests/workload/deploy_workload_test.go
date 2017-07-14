package workload_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Deploy workload", func() {

	It("exposes routes via GCP LBs", func() {

		appUrl := fmt.Sprintf("http://%s:%s", workerAddress, nodePort)

		timeout := time.Duration(5 * time.Second)
		httpClient := http.Client{
			Timeout: timeout,
		}

		_, err := httpClient.Get(appUrl)
		Expect(err).To(HaveOccurred())

		deployNginx := runner.RunKubectlCommand("create", "-f", nginxSpec)
		Eventually(deployNginx, "60s").Should(gexec.Exit(0))
		rolloutWatch := runner.RunKubectlCommand("rollout", "status", "deployment/nginx", "-w")
		Eventually(rolloutWatch, "120s").Should(gexec.Exit(0))

		result, err := httpClient.Get(appUrl)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.StatusCode).Should(Equal(200))
	})

	AfterEach(func() {
		session := runner.RunKubectlCommand("delete", "-f", nginxSpec)
		session.Wait("30s")
	})

})