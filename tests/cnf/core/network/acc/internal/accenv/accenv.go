package accenv

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/openshift-kni/eco-goinfra/pkg/clients"
	"github.com/openshift-kni/eco-goinfra/pkg/nodes"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netconfig"
	"github.com/openshift-kni/eco-gotests/tests/cnf/core/network/internal/netenv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// DoesClusterSupportAcceleratorTests verifies if given cluster supports accelerator workload and test cases.
func DoesClusterSupportAcceleratorTests(
	apiClient *clients.Settings, netConfig *netconfig.NetworkConfig) error {
	glog.V(90).Infof("Verifying if cluster supports accelerator tests")

	err := netenv.DoesClusterHasEnoughNodes(apiClient, netConfig, 1, 2)

	if err != nil {
		return err
	}

	return nil
}
