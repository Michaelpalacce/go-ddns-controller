/*
Copyright 2024.

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

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Michaelpalacce/go-ddns-controller/test/utils"
)

const (
	namespace      = "go-ddns-controller-system"
	controllerName = "go-ddns-controller"
)

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			var err error

			// projectimage stores the name of the image used in the example
			projectRepository := "ghcr.io/michaelpalacce/go-ddns-controller"
			projectVersion := "v0.0.1"
			projectimage := fmt.Sprintf("%s:%s", projectRepository, projectVersion)

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the manager(Operator) image to the kind cluster")
			cmd = exec.Command("make", "kind-deploy", fmt.Sprintf("IMG=%s", projectimage), fmt.Sprintf("REPLICAS=%d", 1))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the go-ddns-controller pod is running as expected")
			EventuallyWithOffset(1, verifyControllerUp(1), time.Minute, time.Second).Should(Succeed())

			By("scaling the manager(Operator) deployment to 3 replicas and enabling leader election")
			cmd = exec.Command(
				"make",
				"kind-deploy",
				fmt.Sprintf("IMG=%s", projectimage),
				fmt.Sprintf("REPLICAS=%d", 3),
				fmt.Sprintf("ARGS=%s", "--leader-elect,--health-probe-bind-address=:8081"),
			)

			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			EventuallyWithOffset(1, verifyControllerUp(3), time.Minute, time.Second).Should(Succeed())
		})
	})
})

func verifyControllerUp(podCount int) func() error {
	return func() error {
		// Get pod name
		cmd := exec.Command("kubectl", "get",
			"pods", "-l", "control-plane=controller-manager",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", namespace,
		)

		podOutput, err := utils.Run(cmd)
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		podNames := utils.GetNonEmptyLines(string(podOutput))
		if len(podNames) != podCount {
			return fmt.Errorf("expect %d controller pods running, but got %d", podCount, len(podNames))
		}

		for _, podName := range podNames {
			ExpectWithOffset(2, podName).Should(ContainSubstring("go-ddns-controller"))

			// Validate pod status
			cmd = exec.Command("kubectl", "get",
				"pods", podName, "-o", "jsonpath=\"{.status.containerStatuses[*].ready}\"",
				"-n", namespace,
			)
			ready, err := utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			fmt.Println(string(ready))
			if string(ready) != "\"true\"" {
				return fmt.Errorf("controller pod in %s ready", ready)
			}
		}

		return nil
	}
}
