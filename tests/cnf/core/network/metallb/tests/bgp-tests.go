package tests

import (
	"fmt"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eco-goinfra/pkg/configmap"
	"github.com/openshift-kni/eco-goinfra/pkg/metallb"
	"github.com/openshift-kni/eco-goinfra/pkg/nad"
	"github.com/openshift-kni/eco-goinfra/pkg/namespace"
	"github.com/openshift-kni/eco-goinfra/pkg/pod"
	"github.com/openshift-kni/eco-goinfra/pkg/reportxml"
	"github.com/openshift-kni/eco-goinfra/pkg/service"
	netcmd "github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/cmd"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/define"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/frrconfig"
	. "github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netinittools"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netparam"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/metallb/internal/frr"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/metallb/internal/metallbenv"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/metallb/internal/tsparams"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("BGP", Ordered, Label(tsparams.LabelBGPTestCases), ContinueOnFailure, func() {
	BeforeAll(func() {
		validateEnvVarAndGetNodeList()

		By("Creating a new instance of MetalLB Speakers on workers")
		err := metallbenv.CreateNewMetalLbDaemonSetAndWaitUntilItsRunning(tsparams.DefaultTimeout, workerLabelMap)
		Expect(err).ToNot(HaveOccurred(), "Failed to recreate metalLb daemonset")
	})

	AfterAll(func() {
		if len(cnfWorkerNodeList) > 2 {
			By("Remove custom metallb test label from nodes")
			removeNodeLabel(workerNodeList, metalLbTestsLabel)
		}

		resetOperatorAndTestNS()
	})

	AfterEach(func() {
		By("Cleaning MetalLb operator namespace")
		metalLbNs, err := namespace.Pull(APIClient, NetConfig.MlbOperatorNamespace)
		Expect(err).ToNot(HaveOccurred(), "Failed to pull metalLb operator namespace")
		err = metalLbNs.CleanObjects(
			tsparams.DefaultTimeout,
			metallb.GetBGPPeerGVR(),
			metallb.GetBFDProfileGVR(),
			metallb.GetBGPAdvertisementGVR(),
			metallb.GetIPAddressPoolGVR())
		Expect(err).ToNot(HaveOccurred(), "Failed to remove object's from operator namespace")

		By("Cleaning test namespace")
		err = namespace.NewBuilder(APIClient, tsparams.TestNamespaceName).CleanObjects(
			tsparams.DefaultTimeout,
			pod.GetGVR(),
			service.GetGVR(),
			configmap.GetGVR(),
			nad.GetGVR())
		Expect(err).ToNot(HaveOccurred(), "Failed to clean test namespace")
	})

	Context("functionality", func() {
		DescribeTable("Creating AddressPool with bgp-advertisement", reportxml.ID("47174"),
			func(ipStack string, prefixLen int) {

				if ipStack == netparam.IPV4Family && clusterSupportsIPv4() {
					_, extFrrPod, _ := setupTestEnv(ipStack, prefixLen, false)

					By("Validating BGP route prefix")
					validatePrefix(
						extFrrPod, ipStack, prefixLen, removePrefixFromIPList(nodeAddrList[ipStack]), tsparams.LBipRange1[ipStack])
				} else if ipStack == netparam.IPV6Family && clusterSupportsIPv6() {
					_, extFrrPod, _ := setupTestEnv(ipStack, prefixLen, false)

					By("Validating BGP route prefix")
					validatePrefix(
						extFrrPod, ipStack, prefixLen, removePrefixFromIPList(nodeAddrList[ipStack]), tsparams.LBipRange1[ipStack])
				}
			},

			Entry("", netparam.IPV4Family, 32,
				reportxml.SetProperty("IPStack", netparam.IPV4Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet32)),
			Entry("", netparam.IPV4Family, 28,
				reportxml.SetProperty("IPStack", netparam.IPV4Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet28)),
			Entry("", netparam.IPV6Family, 128,
				reportxml.SetProperty("IPStack", netparam.IPV6Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet128)),
			Entry("", netparam.IPV6Family, 64,
				reportxml.SetProperty("IPStack", netparam.IPV6Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet64)),
		)

		It("provides Prometheus BGP metrics", reportxml.ID("47202"), func() {
			frrk8sPods, _, _ := setupIPv4TestEnv(32, false)

			By("Label namespace")
			testNs, err := namespace.Pull(APIClient, NetConfig.MlbOperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			_, err = testNs.WithLabel(tsparams.PrometheusMonitoringLabel, "true").Update()
			Expect(err).ToNot(HaveOccurred())

			By("Listing prometheus pods")
			prometheusPods, err := pod.List(APIClient, NetConfig.PrometheusOperatorNamespace, metav1.ListOptions{
				LabelSelector: tsparams.PrometheusMonitoringPodLabel,
			})
			Expect(err).ToNot(HaveOccurred(), "Failed to list prometheus pods")

			verifyMetricPresentInPrometheus(
				frrk8sPods, prometheusPods[0], "frrk8s_bgp_", tsparams.MetalLbBgpMetrics)
		})

		FDescribeTable("Verify external FRR BGP Peer cannot propagate routes to Speaker",
			reportxml.ID("47203"),
			func(ipStack string) {

				var frrk8sPods []*pod.Builder
				var extFrrPod *pod.Builder

				if ipStack == netparam.IPV4Family && clusterSupportsIPv4() {
					frrk8sPods, extFrrPod, _ = setupTestEnv(ipStack, 32, true)
				} else if ipStack == netparam.IPV6Family && clusterSupportsIPv6() {
					frrk8sPods, extFrrPod, _ = setupTestEnv(ipStack, 128, true)
				}

				By("Verify external FRR is advertising prefixes")
				advRoutes, err := frr.GetBGPAdvertisedRoutes(extFrrPod, netcmd.RemovePrefixFromIPList(nodeAddrList[ipStack]))
				Expect(err).ToNot(HaveOccurred(), "Failed to get BGP Advertised routes")
				Expect(len(advRoutes)).To(BeNumerically(">", 0), "BGP Advertised routes should not be empty")

				By("Verify MetalLB FRR pod is not receiving routes from External FRR Pod")
				recRoutes, err := frr.VerifyBGPReceivedRoutesOnFrrNodes(frrk8sPods)
				Expect(err).ToNot(HaveOccurred(), "Failed to verify BGP routes")
				Expect(recRoutes).ShouldNot(SatisfyAny(
					ContainSubstring(tsparams.ExtFrrConnectedPools[ipStack][0]), ContainSubstring(tsparams.ExtFrrConnectedPools[ipStack][1])),
					"Received routes validation failed")
			},
			//Entry("", netparam.IPV4Family,
			//	reportxml.SetProperty("IPStack", netparam.IPV4Family)),
			Entry("", netparam.IPV6Family,
				reportxml.SetProperty("IPStack", netparam.IPV6Family)),
		)
	})

	Context("Log Level Feature", func() {
		It("Verify frrk8s pod default Info logs", reportxml.ID("49810"), func() {
			By("Fetch speaker pods from metallb-system namespace")
			speakerPods, err := pod.List(APIClient, NetConfig.MlbOperatorNamespace,
				metav1.ListOptions{LabelSelector: tsparams.SpeakerLabel})
			Expect(err).ToNot(HaveOccurred(), "Failed to list speaker pods")
			Expect(len(speakerPods)).Should(BeNumerically(">", 0), "Speaker Pods List should not be empty")

			By("Verify loglevel in speaker pod logs")
			for _, speakerPod := range speakerPods {
				podLogs, err := speakerPod.GetFullLog("speaker")
				Expect(err).ToNot(HaveOccurred(), "Failed to get speaker pod logs")
				Expect(podLogs).Should(SatisfyAll(ContainSubstring("info"), Not(ContainSubstring("debug"))),
					"Pods logs should contain info logs only")
			}
		})

		It("Verify frrk8s debug logs", reportxml.ID("49812"), func() {
			By("Creating a new instance of MetalLB with Log level set to debug")
			err := metallbenv.CreateNewMetalLbDaemonSetAndWaitUntilItsRunning(tsparams.DefaultTimeout, workerLabelMap, "debug")
			Expect(err).ToNot(HaveOccurred(), "Failed to create a new instance of MetalLB with Log level set to debug")

			By("Fetch speaker pods from metallb-system namespace")
			speakerPods, err := pod.List(APIClient, NetConfig.MlbOperatorNamespace,
				metav1.ListOptions{LabelSelector: tsparams.SpeakerLabel})
			Expect(err).ToNot(HaveOccurred(), "Failed to list speaker pods")
			Expect(len(speakerPods)).Should(BeNumerically(">", 0), "Speaker Pods List should not be empty")

			By("Verify loglevel in speaker pod logs")
			for _, speakerPod := range speakerPods {
				podLogs, err := speakerPod.GetFullLog("speaker")
				Expect(err).ToNot(HaveOccurred(), "Failed to get speaker pod logs")
				Expect(podLogs).Should(SatisfyAll(ContainSubstring("debug"), ContainSubstring("info")),
					"Pods logs should contain both info and debug logs")
			}
		})
	})

	Context("Updates", func() {
		DescribeTable("Verify bgp-advertisement updates", reportxml.ID("47178"),
			func(ipStack string, prefixLen int) {

				// To-Do: This should be removed once we have dual stack clusters for testing.
				// Also, the test procedure for IPv6 should be supported.
				if ipStack == netparam.IPV6Family {
					Skip("bgp test cases doesn't support ipv6 yet")
				}

				_, extFrrPod, bgpAdv := setupIPv4TestEnv(prefixLen, false)

				By("Validating BGP route prefix")
				validatePrefix(
					extFrrPod, ipStack, prefixLen, removePrefixFromIPList(ipv4NodeAddrList), tsparams.LBipv4Range)

				By("Validate BGP Community is received on the External FRR Pod")
				bgpStatus, err := frr.GetBGPCommunityStatus(extFrrPod, tsparams.NoAdvertiseCommunity, strings.ToLower(ipStack))
				Expect(err).ToNot(HaveOccurred(), "Failed to collect bgp community status")
				Expect(len(bgpStatus.Routes)).To(BeNumerically(">", 0),
					"Failed to fetch BGP routes with required Community")

				By("Validate BGP Local Preference received on External FRR Pod")
				bgpStatus, err = frr.GetBGPStatus(extFrrPod, strings.ToLower(ipStack))
				Expect(err).ToNot(HaveOccurred(), "Failed to collect bgp command output")
				for _, frrRoute := range bgpStatus.Routes {
					Expect(frrRoute[0].LocalPref).To(Equal(uint32(100)))
				}

				By("Update BGP Advertisements")
				_, err = bgpAdv.
					WithLocalPref(200).
					WithAggregationLength4(28).
					WithCommunities([]string{tsparams.CustomCommunity}).
					Update(false)
				Expect(err).ToNot(HaveOccurred(), "Failed to update BGPAdvertisement")

				By("Validating updated BGP route prefix")
				_, subnet, err := net.ParseCIDR(tsparams.LBipv4Range[0] + "/28")
				Expect(err).ToNot(HaveOccurred(), "Failed to parse CIDR")

				Eventually(func() (map[string][]frr.Route, error) {
					bgpStatus, err := frr.GetBGPStatus(extFrrPod, strings.ToLower(ipStack), "test")
					if err != nil {
						return nil, err
					}

					return bgpStatus.Routes, nil
				}, time.Minute, tsparams.DefaultRetryInterval).Should(HaveKey(subnet.String()))

				By("Validate BGP Community received on External FRR Pod")
				bgpStatus, err = frr.GetBGPCommunityStatus(extFrrPod, tsparams.CustomCommunity, strings.ToLower(ipStack))
				Expect(err).ToNot(HaveOccurred(), "Failed to collect bgp community status")
				Expect(len(bgpStatus.Routes)).To(BeNumerically(">", 0),
					"Failed to fetch BGP routes with required Community")

				By("Validate BGP Local Preference on External FRR Pod")
				bgpStatus, err = frr.GetBGPStatus(extFrrPod, strings.ToLower(ipStack))
				Expect(err).ToNot(HaveOccurred(), "Failed to collect bgp command output")
				for _, frrRoute := range bgpStatus.Routes {
					Expect(frrRoute[0].LocalPref).To(Equal(uint32(200)))
				}
			},
			Entry("", netparam.IPV4Family, 32,
				reportxml.SetProperty("IPStack", netparam.IPV4Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet32)),
			Entry("", netparam.IPV6Family, 128,
				reportxml.SetProperty("IPStack", netparam.IPV6Family),
				reportxml.SetProperty("PrefixLength", netparam.IPSubnet128)),
		)

		It("BGP Timer update", reportxml.ID("47180"), func() {
			frrk8sPods, extFrrPod, _ := setupIPv4TestEnv(32, false)

			By("Verify BGP Timers of neighbors in external FRR Pod")
			verifyBGPTimer(extFrrPod, ipv4NodeAddrList, 180000, 60000)

			By("Update BGP Timers")
			bgpPeer, err := metallb.PullBGPPeer(APIClient, tsparams.BgpPeerName1, NetConfig.MlbOperatorNamespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to fetch BGP peer")

			_, err = bgpPeer.WithHoldTime(metav1.Duration{Duration: 30000 * time.Millisecond}).
				WithKeepalive(metav1.Duration{Duration: 10000 * time.Millisecond}).
				Update(false)
			Expect(err).NotTo(HaveOccurred(), "Failed to update BGP peer")

			By("Verify Timers updated in frrk8s pods")
			for _, frrk8sPod := range frrk8sPods {
				Eventually(frr.CheckFRRConfigLine,
					time.Minute, tsparams.DefaultRetryInterval).WithArguments(frrk8sPod, " timers 10 30").
					Should(BeTrue(), "BFD is not configured on the Speakers")
			}

			By("Verify BGP Timers of neighbors in external FRR Pod are updated")
			err = frr.ResetBGPConnection(extFrrPod)
			Expect(err).NotTo(HaveOccurred(), "Failed to reset BGP connection")

			verifyBGPTimer(extFrrPod, ipv4NodeAddrList, 30000, 10000)
		})
	})
})

func setupTestEnv(ipStack string, prefixLen int, extFrrAdv bool) (
	[]*pod.Builder,
	*pod.Builder,
	*metallb.BGPAdvertisementBuilder,
) {
	By("Fetching frrk8s pods list running on the worker nodes selected for metallb tests")

	frrk8sPods := verifyAndCreateFRRk8sPodList()

	By("Creating BGPPeer with external FRR Pod")
	createBGPPeerAndVerifyIfItsReady(tsparams.BgpPeerName1,
		metallbAddrList[ipStack][0], "", tsparams.LocalBGPASN, false, 0, frrk8sPods)

	By("Creating an IPAddressPool")

	ipAddressPool, err := metallb.NewIPAddressPoolBuilder(
		APIClient,
		"address-pool",
		NetConfig.MlbOperatorNamespace,
		[]string{fmt.Sprintf("%s-%s", tsparams.LBipRange1[ipStack][0], tsparams.LBipRange1[ipStack][1])}).Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create IPAddressPool")

	By("Creating a BGPAdvertisement")

	var bgpAdvertisement *metallb.BGPAdvertisementBuilder

	bgpAdvertisement = metallb.
		NewBGPAdvertisementBuilder(APIClient, "bgpadvertisement", NetConfig.MlbOperatorNamespace).
		WithIPAddressPools([]string{ipAddressPool.Definition.Name}).
		WithCommunities([]string{tsparams.NoAdvertiseCommunity}).
		WithLocalPref(100)

	if ipStack == netparam.IPV6Family {
		bgpAdvertisement = bgpAdvertisement.WithAggregationLength6(int32(prefixLen))
	} else {
		bgpAdvertisement = bgpAdvertisement.WithAggregationLength4(int32(prefixLen))
	}

	bgpAdvertisement, err = bgpAdvertisement.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create BGPAdvertisement")

	By("Deploy nginx on single worker with LB service")
	setupNGNXPod("nginxpod1", workerNodeList[0].Object.Name, tsparams.LabelValue1)
	setupMetalLbService(tsparams.MetallbServiceName, ipStack, tsparams.LabelValue1, ipAddressPool,
		corev1.ServiceExternalTrafficPolicyTypeCluster)

	By("Creating configMap with selected worker nodes as BGP Peers for external FRR Pod")

	var masterConfigMap *configmap.Builder

	if extFrrAdv {
		masterConfigMap = createConfigMapWithNetwork(
			ipStack, tsparams.LocalBGPASN, nodeAddrList[ipStack], tsparams.ExtFrrConnectedPools[ipStack])
	} else {
		masterConfigMap = createConfigMap(
			tsparams.LocalBGPASN, nodeAddrList[ipStack], false, false)
	}

	By("Creating macvlan NAD for external FRR Pod")

	err = define.CreateExternalNad(APIClient, frrconfig.ExternalMacVlanNADName, tsparams.TestNamespaceName)
	Expect(err).ToNot(HaveOccurred(), "Failed to create a macvlan NAD")

	By("Creating external FRR Pod with configMap mount and external NAD")

	extFrrPod := createFrrPod(masterNodeList[0].Object.Name, masterConfigMap.Object.Name, []string{},
		pod.StaticIPAnnotation(frrconfig.ExternalMacVlanNADName, []string{fmt.Sprintf("%s/%s", metallbAddrList[ipStack][0], frrPodSubnet[ipStack])}))

	By("Checking that BGP session is established on external FRR Pod")
	verifyMetalLbBGPSessionsAreUPOnFrrPod(extFrrPod, nodeAddrList[ipStack])

	return frrk8sPods, extFrrPod, bgpAdvertisement
}

func setupIPv4TestEnv(prefixLen int, extFrrAdv bool) (
	[]*pod.Builder,
	*pod.Builder,
	*metallb.BGPAdvertisementBuilder,
) {
	By("Fetching frrk8s pods list running on the worker nodes selected for metallb tests")

	frrk8sPods := verifyAndCreateFRRk8sPodList()

	By("Creating BGPPeer with external FRR Pod")
	createBGPPeerAndVerifyIfItsReady(tsparams.BgpPeerName1,
		ipv4metalLbIPList[0], "", tsparams.LocalBGPASN, false, 0, frrk8sPods)

	By("Creating an IPAddressPool")

	ipAddressPool, err := metallb.NewIPAddressPoolBuilder(
		APIClient,
		"address-pool",
		NetConfig.MlbOperatorNamespace,
		[]string{fmt.Sprintf("%s-%s", tsparams.LBipv4Range[0], tsparams.LBipv4Range[1])}).Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create IPAddressPool")

	By("Creating a BGPAdvertisement")

	bgpAdvertisement, err := metallb.
		NewBGPAdvertisementBuilder(APIClient, "bgpadvertisement", NetConfig.MlbOperatorNamespace).
		WithIPAddressPools([]string{ipAddressPool.Definition.Name}).
		WithAggregationLength4(int32(prefixLen)).
		WithCommunities([]string{tsparams.NoAdvertiseCommunity}).
		WithLocalPref(100).
		Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create BGPAdvertisement")

	By("Deploy nginx on single worker with LB service")
	setupNGNXPod("nginxpod1", workerNodeList[0].Object.Name, tsparams.LabelValue1)
	setupMetalLbService(tsparams.MetallbServiceName, netparam.IPV4Family, tsparams.LabelValue1, ipAddressPool,
		corev1.ServiceExternalTrafficPolicyTypeCluster)

	By("Creating configMap with selected worker nodes as BGP Peers for external FRR Pod")

	var masterConfigMap *configmap.Builder

	if extFrrAdv {
		masterConfigMap = createConfigMapWithNetwork("ipv4",
			tsparams.LocalBGPASN, ipv4NodeAddrList, tsparams.ExtFrrConnectedPool)
	} else {
		masterConfigMap = createConfigMap(tsparams.LocalBGPASN, ipv4NodeAddrList, false, false)
	}

	By("Creating macvlan NAD for external FRR Pod")

	err = define.CreateExternalNad(APIClient, frrconfig.ExternalMacVlanNADName, tsparams.TestNamespaceName)
	Expect(err).ToNot(HaveOccurred(), "Failed to create a macvlan NAD")

	By("Creating external FRR Pod with configMap mount and external NAD")

	extFrrPod := createFrrPod(masterNodeList[0].Object.Name, masterConfigMap.Object.Name, []string{},
		pod.StaticIPAnnotation(frrconfig.ExternalMacVlanNADName, []string{fmt.Sprintf("%s/%s", ipv4metalLbIPList[0], "24")}))

	By("Checking that BGP session is established on external FRR Pod")
	verifyMetalLbBGPSessionsAreUPOnFrrPod(extFrrPod, ipv4NodeAddrList)

	return frrk8sPods, extFrrPod, bgpAdvertisement
}

func verifyBGPTimer(frrPod *pod.Builder, peerAddrList []string, hTimer, aTimer int) {
	for _, peerAddress := range netcmd.RemovePrefixFromIPList(peerAddrList) {
		Eventually(frr.VerifyBGPNeighborTimer,
			time.Minute*3, tsparams.DefaultRetryInterval).
			WithArguments(frrPod, peerAddress, hTimer, aTimer).Should(
			BeTrue(), "Failed to verify BGP Timer on peer")
	}
}

func createConfigMapWithNetwork(
	ipStack string,
	bgpAsn int,
	nodeAddrList, externalAdvertisedRoutes []string) *configmap.Builder {

	var frrBFDConfig string

	if ipStack == ipv6 {
		frrBFDConfig = frr.DefineBGPConfigWithIPv6Network(
			bgpAsn,
			tsparams.LocalBGPASN,
			externalAdvertisedRoutes,
			netcmd.RemovePrefixFromIPList(nodeAddrList),
			false,
			false,
		)
	} else {
		frrBFDConfig = frr.DefineBGPConfigWithIPv4Network(
			bgpAsn,
			tsparams.LocalBGPASN,
			externalAdvertisedRoutes,
			netcmd.RemovePrefixFromIPList(nodeAddrList),
			false,
			false,
		)
	}

	configMapData := frrconfig.DefineBaseConfig(frrconfig.DaemonsFile, frrBFDConfig, "")

	masterConfigMap, err := configmap.NewBuilder(APIClient, "frr-master-node-config", tsparams.TestNamespaceName).
		WithData(configMapData).
		Create()

	Expect(err).ToNot(HaveOccurred(), "Failed to create config map")

	return masterConfigMap
}
