package master_failure_test

import (
	. "tests/test_helpers"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/cppforlife/turbulence/incident"
	"github.com/cppforlife/turbulence/incident/selector"
	"github.com/cppforlife/turbulence/tasks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = MasterFailureDescribe("A single master and etcd failure", func() {

	var (
		deployment                    director.Deployment
		kubectl                       *KubectlRunner
		nginxSpec                     = PathFromRoot("specs/nginx.yml")
		countRunningApiServerOnMaster func() int
	)

	BeforeEach(func() {
		var err error
		director := NewDirector(testconfig.Bosh)
		deployment, err = director.FindDeployment(testconfig.Bosh.Deployment)
		Expect(err).NotTo(HaveOccurred())
		countRunningApiServerOnMaster = CountProcessesOnVmsOfType(deployment, MasterVmType, "kube-apiserver", VmRunningState)

		if testconfig.TurbulenceTests.IsMultiAZ {
			Expect(countRunningApiServerOnMaster()).To(Equal(3))
		} else {
			Expect(countRunningApiServerOnMaster()).To(Equal(1))
		}

		kubectl = NewKubectlRunner(testconfig.Kubernetes.PathToKubeConfig)
		kubectl.CreateNamespace()
	})

	AfterEach(func() {
		kubectl.RunKubectlCommand("delete", "-f", nginxSpec)
		kubectl.RunKubectlCommand("delete", "namespace", kubectl.Namespace())
	})

	Specify("The cluster is healthy after master is resurrected", func() {
		By("Deploying a workload on the k8s cluster")
		Eventually(kubectl.RunKubectlCommand("create", "-f", nginxSpec), "30s", "5s").Should(gexec.Exit(0))
		Eventually(kubectl.RunKubectlCommand("rollout", "status", "deployment/nginx", "-w"), "120s").Should(gexec.Exit(0))

		By("Deleting the Master VM")
		hellRaiser := TurbulenceClient(testconfig.Turbulence)
		killOneMaster := incident.Request{
			Selector: selector.Request{
				Deployment: &selector.NameRequest{
					Name: testconfig.Bosh.Deployment,
				},
				Group: &selector.NameRequest{
					Name: MasterVmType,
				},
				ID: &selector.IDRequest{
					Limit: selector.MustNewLimitFromString("1"),
				},
			},
			Tasks: tasks.OptionsSlice{
				tasks.KillOptions{},
			},
		}
		incident := hellRaiser.CreateIncident(killOneMaster)
		incident.Wait()

		if testconfig.TurbulenceTests.IsMultiAZ {
			Expect(countRunningApiServerOnMaster()).To(Equal(2))
		} else {
			Expect(countRunningApiServerOnMaster()).To(Equal(0))
		}

		By("Waiting for resurrection")
		Eventually(func() bool { return AllComponentsAreHealthy(kubectl) }, "600s", "20s").Should(BeTrue())

		By("Checking that all nodes are available")
		Expect(AllBoshWorkersHaveJoinedK8s(deployment, kubectl)).To(BeTrue())

		By("Checking for the workload on the k8s cluster")
		session := kubectl.RunKubectlCommand("get", "deployment", "nginx")
		Eventually(session, "120s").Should(gexec.Exit(0))
	})
})
