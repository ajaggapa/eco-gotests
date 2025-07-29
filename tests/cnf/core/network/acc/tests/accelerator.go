package tests

import (
	"fmt"
	"github.com/openshift-kni/eco-goinfra/pkg/nodes"
	"github.com/openshift-kni/eco-goinfra/pkg/schemes/fec/fectypes"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netenv"
	v1 "k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eco-goinfra/pkg/deployment"
	"github.com/openshift-kni/eco-goinfra/pkg/pod"
	sriovfec "github.com/openshift-kni/eco-goinfra/pkg/sriov-fec"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/acc/internal/tsparams"
	. "github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netinittools"
)

var _ = Describe("Intel Accelerator 100", Ordered, Label(tsparams.LabelSuite), ContinueOnFailure, func() {

	BeforeAll(func() {
		By("Checking if operator is installed and has required resources")
		fecDeploy, err := deployment.Pull(APIClient, "sriov-fec-controller-manager", tsparams.OperatorNamespace)
		if err != nil && err.Error() == "no matches for kind \"SriovFecNodeConfig\" in version \"sriovfec.intel.com/v1\"" {
			Skip("Cluster does not have operator installed")
		}
		Expect(err).ToNot(HaveOccurred(), "Failed to pull SriovFecOperator")
		Expect(fecDeploy.IsReady(2*time.Minute)).To(BeTrue(), "SriovFecOperator is not ready")

		By("Checking if node config exists")
		sfncList, err := sriovfec.List(APIClient, tsparams.OperatorNamespace)
		Expect(err).ToNot(HaveOccurred(), "Failed to list SriovFecNodeConfig")
		if len(sfncList) == 0 {
			Skip("No SriovFecNodeConfig found")
		}

		By("Checking if all damensets are present and ready")
		for _, dsName := range tsparams.DaemonsetNames {
			ds, err := deployment.Pull(APIClient, dsName, tsparams.OperatorNamespace)
			Expect(err).ToNot(HaveOccurred(), "Failed to pull %s", dsName)
			Expect(ds.IsReady(2*time.Minute)).To(BeTrue(), "%s is not ready", dsName)
		}

		secureBoot := isSecureBootEnabled()

		By("Deploying PerformanceProfile if it's not installed")
		err = netenv.DeployPerformanceProfile(
			APIClient,
			NetConfig,
			"performance-profile-dpdk",
			"1,3,5,7,9,11,13,15,17,19,21,23,25",
			"0,2,4,6,8,10,12,14,16,18,20",
			24)
		Expect(err).ToNot(HaveOccurred(), "Fail to deploy PerformanceProfile")

		By("Checking if the cluster has ACC100 cards")
		sfnc, accelerator, err := getNodeConfigWithAccCard(tsparams.Acc100DeviceID)
		if err != nil {
			Skip(fmt.Sprintf("Cluster does not have ACC100 cards: %s", err.Error()))
		}

		By("Creating SriovFecClusterConfig")
		defineFecClusterConfig(accelerator.PCIAddress, sfnc.Object.Name, secureBoot)

	})

	AfterEach(func() {
		By("Cleaning up test resources")
		// TODO: Add cleanup logic for test resources
	})
})

func isSecureBootEnabled() bool {
	workernodeList, err := nodes.List(APIClient, v2.ListOptions{LabelSelector: NetConfig.WorkerLabel})
	Expect(err).ToNot(HaveOccurred(), "Failed to list worker nodes")
	Expect(len(workernodeList)).To(BeNumerically(">=", 1), "Worker node list length must be > 1")

	for _, workerNode := range workernodeList {
		testPod, err := pod.NewBuilder(APIClient, "testpod1", tsparams.TestNamespaceName, NetConfig.CnfNetTestContainer).
			WithPrivilegedFlag().
			WithVolume(v1.Volume{Name: "host", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/"}}}).
			WithLocalVolume("host", "/host").
			DefineOnNode(workerNode.Object.Name).
			CreateAndWaitUntilRunning(2 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Failed to create test pod")

		output, err := testPod.ExecCommand([]string{"cat", "/host/sys/kernel/security/lockdown"})
		Expect(err).ToNot(HaveOccurred(), "Failed to get /host/sys/kernel/security/lockdown")
		if strings.Contains(output.String(), "No such file or directory") || err != nil {
			return false
		}

		return strings.Contains(output.String(), "[integrity]") || strings.Contains(output.String(),
			"[confidentiality]")
	}

	return false
}

func getNodeConfigWithAccCard(devID string) (*sriovfec.NodeConfigBuilder, *fectypes.SriovAccelerator, error) {
	sfncList, err := sriovfec.List(APIClient, tsparams.OperatorNamespace)
	Expect(err).ToNot(HaveOccurred(), "Failed to list SriovFecNodeConfig")

	for _, sfnc := range sfncList {
		for _, accelerator := range sfnc.Object.Status.Inventory.SriovAccelerators {
			if accelerator.DeviceID == devID {
				return sfnc, &accelerator, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("cluster doesn`t have sriovfecnodeconfig with accelerator id %s", devID)
}

func defineFecClusterConfig(pciAddress, nodeName string, secureBoot bool) fectypes.SriovFecClusterConfig {

	queueGroupConfig := fectypes.QueueGroupConfig{
		AqDepthLog2:     4,
		NumAqsPerGroups: 16,
		NumQueueGroups:  2,
	}

	pfDriverType := "pci-pf-stub"

	if secureBoot {
		pfDriverType = "vfio-pci"
	}

	sfcc := fectypes.SriovFecClusterConfig{
		ObjectMeta: v2.ObjectMeta{
			Name:      "config",
			Namespace: tsparams.OperatorNamespace,
		},
		Spec: fectypes.SriovFecClusterConfigSpec{
			Priority: 1,
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": nodeName,
			},
			AcceleratorSelector: fectypes.AcceleratorSelector{
				PCIAddress: pciAddress,
			},
			PhysicalFunction: fectypes.PhysicalFunctionConfig{
				PFDriver: pfDriverType,
				VFAmount: 2,
				VFDriver: "vfio-pci",
				BBDevConfig: fectypes.BBDevConfig{
					ACC100: &fectypes.ACC100BBDevConfig{
						Downlink4G:   queueGroupConfig,
						Downlink5G:   queueGroupConfig,
						Uplink4G:     queueGroupConfig,
						Uplink5G:     queueGroupConfig,
						PFMode:       false,
						MaxQueueSize: 1024,
						NumVfBundles: 2,
					},
				},
			},
		},
	}

	return sfcc
}

func waitForNodeConfigToSucceed() {

}
