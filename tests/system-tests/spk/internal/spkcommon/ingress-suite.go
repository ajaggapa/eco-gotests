package spkcommon

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/openshift-kni/eco-goinfra/pkg/pod"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/glog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/internal/url"
	. "github.com/openshift-kni/eco-gotests/tests/system-tests/spk/internal/spkinittools"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/spk/internal/spkparams"
)

//nolint:unparam
func reachURL(targetURL string, expectedCode int) {
	glog.V(spkparams.SPKLogLevel).Infof("Accessing %q via SPK Ingress", targetURL)

	var ctx SpecContext

	Eventually(func() bool {
		data, httpCode, err := url.Fetch(targetURL, "GET")

		if err != nil {
			glog.V(spkparams.SPKLogLevel).Infof("Failed to reach %q: %v", targetURL, err)

			return false
		}

		glog.V(spkparams.SPKLogLevel).Infof("Reached URL %q", targetURL)
		glog.V(spkparams.SPKLogLevel).Infof("HTTP Code: %d", httpCode)
		glog.V(spkparams.SPKLogLevel).Infof("Reached data\n%v", data)

		return httpCode == expectedCode
	}).WithContext(ctx).WithPolling(5*time.Second).WithTimeout(1*time.Minute).Should(BeTrue(),
		fmt.Sprintf("Failed to reach %q URL", targetURL))
}

// AssertIPv4WorkloadURL access workload via IPv4 address.
func AssertIPv4WorkloadURL(ctx SpecContext) {
	glog.V(spkparams.SPKLogLevel).Infof("*** AssertIPv4WorkloadURL ***")

	if SPKConfig.IngressTCPIPv4URL == "" {
		Skip("IPv4 URL for SPK backed workload not defined")
	}

	reachURL(SPKConfig.IngressTCPIPv4URL, int(200))
}

// AssertIPv4WorkloadURLAfterAppRecreated access workload via IPv4 address,
// after target workload was re-created.
func AssertIPv4WorkloadURLAfterAppRecreated(ctx SpecContext) {
	glog.V(spkparams.SPKLogLevel).Infof("*** AssertIPv4WorkloadURL ***")

	if SPKConfig.IngressTCPIPv4URL == "" {
		Skip("IPv4 URL for SPK backed workload not defined")
	}

	SetupSPKBackendWorkload()

	reachURL(SPKConfig.IngressTCPIPv4URL, int(200))
}

// AssertIPv6WorkloadURL access workload via IPv6 address.
func AssertIPv6WorkloadURL(ctx SpecContext) {
	glog.V(spkparams.SPKLogLevel).Infof("*** AssertIPv6WorkloadURL ***")

	if SPKConfig.IngressTCPIPv6URL == "" {
		Skip("IPv6 URL for SPK backed workload not defined")
	}

	reachURL(SPKConfig.IngressTCPIPv6URL, int(200))
}

// AssertIPv6WorkloadURLAfterAppRecreated access workload via IPv6 address,
// after target workload was re-created.
func AssertIPv6WorkloadURLAfterAppRecreated(ctx SpecContext) {
	glog.V(spkparams.SPKLogLevel).Infof("*** AssertIPv6WorkloadURL ***")

	if SPKConfig.IngressTCPIPv6URL == "" {
		Skip("IPv6 URL for SPK backed workload not defined")
	}

	SetupSPKBackendWorkload()
	reachURL(SPKConfig.IngressTCPIPv6URL, int(200))
}

// AssertIPv4WorkloadURLAfterIngressPodDeleted assert workoads are reachable over IPv4 SPK Ingress,
// after SPK Ingress and TMM pods are deleted.
func AssertIPv4WorkloadURLAfterIngressPodDeleted(ctx SpecContext) {
	if SPKConfig.IngressTCPIPv4URL == "" {
		Skip("IPv4 URL for SPK backed workload not defined. Skipping")
	}

	deletePodMatchingLabel(SPKConfig.SPKDataNS, ingressDataLabel, "3m")
	deletePodMatchingLabel(SPKConfig.SPKDnsNS, ingressDNSLabel, "3m")

	deletePodMatchingLabel(SPKConfig.SPKDataNS, tmmLabel, "5m")

	dataPods := findPodWithSelector(SPKConfig.SPKDataNS, tmmLabel, "3s", "60s")

	for _, dPod := range dataPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	deletePodMatchingLabel(SPKConfig.SPKDnsNS, tmmLabel, "5m")

	dnsPods := findPodWithSelector(SPKConfig.SPKDnsNS, tmmLabel, "3s", "60s")

	for _, dPod := range dnsPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	glog.V(spkparams.SPKLogLevel).Infof("Sleeping for 4 minutes")
	time.Sleep(4 * time.Minute)

	AssertIPv4WorkloadURL(ctx)
}

// AssertIPv6WorkloadURLAfterIngressPodDeleted assert workoads are reachable over IPv6 SPK Ingress,
// after SPK Ingress and TMM pods are deleted.
func AssertIPv6WorkloadURLAfterIngressPodDeleted(ctx SpecContext) {
	if SPKConfig.IngressTCPIPv6URL == "" {
		Skip("IPv6 URL for SPK backed workload not defined. Skipping")
	}

	deletePodMatchingLabel(SPKConfig.SPKDataNS, ingressDataLabel, "3m")
	deletePodMatchingLabel(SPKConfig.SPKDnsNS, ingressDNSLabel, "3m")

	deletePodMatchingLabel(SPKConfig.SPKDataNS, tmmLabel, "5m")

	dataPods := findPodWithSelector(SPKConfig.SPKDataNS, tmmLabel, "3s", "60s")

	for _, dPod := range dataPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	deletePodMatchingLabel(SPKConfig.SPKDnsNS, tmmLabel, "5m")

	dnsPods := findPodWithSelector(SPKConfig.SPKDnsNS, tmmLabel, "3s", "60s")

	for _, dPod := range dnsPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	glog.V(spkparams.SPKLogLevel).Infof("Sleeping for 4 minutes")
	time.Sleep(4 * time.Minute)

	AssertIPv6WorkloadURL(ctx)
}

// AssertIPv4UDPWorkloadURLAfterIngressPodDeleted assert workoads are reachable over IPv4 SPK Ingress,
// after SPK Ingress and TMM pods are deleted.
func AssertIPv4UDPWorkloadURLAfterIngressPodDeleted(ctx SpecContext) {
	if SPKConfig.IngressUDPIPv4URL == "" {
		glog.V(spkparams.SPKLogLevel).Infof("IPv4 URL for SPK UDP backed workload not defined")
		Skip("IPv4 URL for SPK UDP backed workload not defined. Skipping")
	}

	deletePodMatchingLabel(SPKConfig.SPKDataNS, ingressDataLabel, "3m")
	deletePodMatchingLabel(SPKConfig.SPKDnsNS, ingressDNSLabel, "3m")

	deletePodMatchingLabel(SPKConfig.SPKDataNS, tmmLabel, "5m")

	dataPods := findPodWithSelector(SPKConfig.SPKDataNS, tmmLabel, "5s", "60s")

	for _, dPod := range dataPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	deletePodMatchingLabel(SPKConfig.SPKDnsNS, tmmLabel, "5m")

	dnsPods := findPodWithSelector(SPKConfig.SPKDnsNS, tmmLabel, "5s", "60s")

	for _, dPod := range dnsPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	glog.V(spkparams.SPKLogLevel).Infof("Sleeping for 4 minutes")
	time.Sleep(4 * time.Minute)

	VerifySPKIngressUDPviaIPv4()
}

// AssertIPv6UDPWorkloadURLAfterIngressPodDeleted assert workoads are reachable over IPv6 SPK Ingress,
// after SPK Ingress and TMM pods are deleted.
func AssertIPv6UDPWorkloadURLAfterIngressPodDeleted(ctx SpecContext) {
	if SPKConfig.IngressUDPIPv6URL == "" {
		Skip("IPv6 UDP URL for SPK backed workload not defined. Skipping")
	}

	deletePodMatchingLabel(SPKConfig.SPKDataNS, ingressDataLabel, "3m")
	deletePodMatchingLabel(SPKConfig.SPKDnsNS, ingressDNSLabel, "3m")

	deletePodMatchingLabel(SPKConfig.SPKDataNS, tmmLabel, "5m")

	dataPods := findPodWithSelector(SPKConfig.SPKDataNS, tmmLabel, "5s", "60s")

	for _, dPod := range dataPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	deletePodMatchingLabel(SPKConfig.SPKDnsNS, tmmLabel, "5m")

	dnsPods := findPodWithSelector(SPKConfig.SPKDnsNS, tmmLabel, "5s", "60s")

	for _, dPod := range dnsPods {
		err := dPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Pod %q is not Ready", dPod.Definition.Name))
	}

	glog.V(spkparams.SPKLogLevel).Infof("Sleeping for 4 minutes")
	time.Sleep(4 * time.Minute)

	VerifySPKIngressUDPviaIPv6()
}

func verifyMsgInPodLogs(podObj *pod.Builder, msg, cName string, timeSpan time.Time) {
	glog.V(spkparams.SPKLogLevel).Infof("Parsing duration %q", timeSpan)

	var (
		podLog string
		err    error
		ctx    SpecContext
	)

	Eventually(func() bool {
		logStartTimestamp := time.Since(timeSpan)
		glog.V(spkparams.SPKLogLevel).Infof("\tTime duration is %s", logStartTimestamp)

		if logStartTimestamp.Abs().Seconds() < 1 {
			logStartTimestamp, err = time.ParseDuration("1s")
			Expect(err).ToNot(HaveOccurred(), "Failed to parse time duration")
		}

		podLog, err = podObj.GetLog(logStartTimestamp, cName)

		if err != nil {
			glog.V(spkparams.SPKLogLevel).Infof("Failed to get logs from pod %q: %v", podObj.Definition.Name, err)

			return false
		}

		glog.V(spkparams.SPKLogLevel).Infof("Logs from pod %s:\n%s", podObj.Definition.Name, podLog)

		return true
	}).WithContext(ctx).WithPolling(5*time.Second).WithTimeout(1*time.Minute).Should(BeTrue(),
		fmt.Sprintf("Failed to get logs from pod %q", podObj.Definition.Name))

	Expect(podLog).Should(ContainSubstring(msg))
}

func findPodWithSelector(fNamespace, podLabel, pollInterval, pollDuration string) []*pod.Builder {
	By(fmt.Sprintf("Getting pod(s) matching selector %q", podLabel))

	var (
		podMatchingSelector []*pod.Builder
		err                 error
		ctx                 SpecContext
	)

	pollFrequencey, err := time.ParseDuration(pollInterval)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Failed to parse polling interval %q", pollInterval))

	pollTimeout, err := time.ParseDuration(pollDuration)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Failed to parse polling interval %q", pollDuration))

	podOneSelector := metav1.ListOptions{
		LabelSelector: podLabel,
	}

	Eventually(func() bool {
		podMatchingSelector, err = pod.List(APIClient, fNamespace, podOneSelector)
		if err != nil {
			glog.V(spkparams.SPKLogLevel).Infof("Failed to list pods in %q namespace: %v",
				fNamespace, err)

			return false
		}

		if len(podMatchingSelector) == 0 {
			glog.V(spkparams.SPKLogLevel).Infof("Found 0 pods matching label %q in namespace %q",
				podLabel, fNamespace)

			return false
		}

		return true
	}).WithContext(ctx).WithPolling(pollFrequencey).WithTimeout(pollTimeout).Should(BeTrue(),
		fmt.Sprintf("Failed to find pod matching label %q in %q namespace", podLabel, fNamespace))

	return podMatchingSelector
}

// VerifySPKIngressUDPviaIPv4 verifies SPK UDP Ingress.
func VerifySPKIngressUDPviaIPv4() {
	glog.V(spkparams.SPKLogLevel).Infof("*** Verify SPK UPD Ingress over IPv4 ***")

	if SPKConfig.IngressUDPIPv4URL == "" {
		glog.V(spkparams.SPKLogLevel).Infof("IPv4 URL for SPK UDP backed workload not defined")
		Skip("IPv4 URL for SPK UDP backed workload not defined")
	}

	verifyUDPIngress(SPKConfig.IngressUDPIPv4URL)
}

// VerifySPKIngressUDPviaIPv6 verifies SPK UDP Ingress.
func VerifySPKIngressUDPviaIPv6() {
	glog.V(spkparams.SPKLogLevel).Infof("*** Verify SPK UPD Ingress over IPv6 ***")

	if SPKConfig.IngressUDPIPv6URL == "" {
		glog.V(spkparams.SPKLogLevel).Infof("IPv6 URL for SPK UDP backed workload not defined")
		Skip("IPv6 URL for SPK UDP backed workload not defined")
	}

	verifyUDPIngress(SPKConfig.IngressUDPIPv6URL)
}

func verifyUDPIngress(udpAddr string) {
	var (
		err      error
		ctx      SpecContext
		bWritten int
		rNum     int
	)

	// depending on the time when workload is started and data is sent
	// there's possible race condition, hence random sleep.
	rSrc := rand.NewSource(time.Now().Unix())
	rSrc.Seed(time.Now().Unix())

	Eventually(func() bool {
		rNum = rand.Intn(3)

		return rNum != 0
	}).WithContext(ctx).WithPolling(5*time.Millisecond).WithTimeout(1*time.Second).Should(BeTrue(),
		"Failed to generate pseudo number")

	rSleep, err := time.ParseDuration(fmt.Sprintf("%ds", rNum))

	Expect(err).ToNot(HaveOccurred(), "Failed to parse time duration")

	glog.V(spkparams.SPKLogLevel).Infof("Sleeping for %s", rSleep.String())
	time.Sleep(rSleep)

	By("Looking for UDP server pods")

	udpPods := findPodWithSelector(SPKConfig.Namespace, "systemtest-app=spk-udp-server", "5s", "1m")

	glog.V(spkparams.SPKLogLevel).Infof("Dialing to UDP endpoint %q", udpAddr)

	udpConnection, err := net.Dial("udp", udpAddr)

	Expect(err).ToNot(HaveOccurred(), "Failed to Dial to UDP endpoint")

	defer udpConnection.Close()

	timeStart := time.Now()
	udpMSG := fmt.Sprintf("UDP message sent at %d", time.Now().Unix())

	Eventually(func() bool {
		glog.V(spkparams.SPKLogLevel).Infof("Sending message: %q(%d bytes)", udpMSG, len([]byte(udpMSG)))

		bWritten, err = udpConnection.Write([]byte(udpMSG))

		if err != nil {
			glog.V(spkparams.SPKLogLevel).Infof("Failed to send message: %v", err)

			return false
		}

		if bWritten != len([]byte(udpMSG)) {
			glog.V(spkparams.SPKLogLevel).Infof("Sent %d bytes via UDP, expected %d",
				bWritten, len([]byte(udpMSG)))

			return false
		}

		glog.V(spkparams.SPKLogLevel).Infof("Successfully sent %d bytes via UDP", bWritten)

		return true
	}).WithContext(ctx).WithPolling(5*time.Second).WithTimeout(90*time.Second).Should(BeTrue(),
		"Failed to send message via UDP")

	for _, udpPod := range udpPods {
		glog.V(spkparams.SPKLogLevel).Infof("Checking logs in %q", udpPod.Definition.Name)
		verifyMsgInPodLogs(udpPod, udpMSG, udpPod.Definition.Spec.Containers[0].Name, timeStart)
	}
}