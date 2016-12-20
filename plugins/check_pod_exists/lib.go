package check_pod_exists

import (
	"fmt"
	"os"
	"strings"

	flags "github.com/appscode/go-flags"
	"github.com/appscode/searchlight/pkg/config"
	"github.com/appscode/searchlight/util"
	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

type request struct {
	host  string
	count int
}

type objectInfo struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status,omitempty"`
}

type serviceOutput struct {
	Objects []*objectInfo `json:"objects,omitempty"`
	Message string        `json:"message,omitempty"`
}

func checkPodExists(req *request, namespace, objectType, objectName string, checkCount bool) {
	kubeClient, err := config.NewKubeClient()
	if err != nil {
		fmt.Fprintln(os.Stdout, util.State[3], err)
		os.Exit(3)
	}

	total_pod := 0
	if objectType == config.TypePods {
		pod, err := kubeClient.Client.Core().Pods(namespace).Get(objectName)
		if err != nil {
			fmt.Fprintln(os.Stdout, util.State[3], err)
			os.Exit(3)
		}
		if pod != nil {
			total_pod = 1
		}
	} else {
		labelSelector := labels.Everything()
		if objectType != "" {
			if labelSelector, err = util.GetLabels(kubeClient, namespace, objectType, objectName); err != nil {
				fmt.Fprintln(os.Stdout, util.State[3], err)
				os.Exit(3)
			}
		}

		podList, err := kubeClient.Client.Core().
			Pods(namespace).List(
			kapi.ListOptions{
				LabelSelector: labelSelector,
			},
		)
		if err != nil {
			fmt.Fprintln(os.Stdout, util.State[3], err)
			os.Exit(3)
		}

		total_pod = len(podList.Items)
	}

	if checkCount {
		if req.count != total_pod {
			fmt.Fprintln(os.Stdout, util.State[2], fmt.Sprintf("Found %d pod(s) instead of %d", total_pod, req.count))
			os.Exit(2)
		} else {
			fmt.Fprintln(os.Stdout, util.State[0], "Found all pods")
			os.Exit(0)
		}
	} else {
		if total_pod == 0 {
			fmt.Fprintln(os.Stdout, util.State[2], "No pod found")
			os.Exit(2)
		} else {
			fmt.Fprintln(os.Stdout, util.State[0], fmt.Sprintf("Found %d pods(s)", total_pod))
			os.Exit(0)
		}
	}
}

func NewCmd() *cobra.Command {
	var req request
	c := &cobra.Command{
		Use:     "check_pod_exists",
		Short:   "Check Kubernetes Pod(s)",
		Example: "",

		Run: func(cmd *cobra.Command, args []string) {
			flags.EnsureRequiredFlags(cmd, "host")

			parts := strings.Split(req.host, "@")
			if len(parts) != 2 {
				fmt.Fprintln(os.Stdout, util.State[3], "Invalid icinga host.name")
				os.Exit(3)
			}

			name := parts[0]
			namespace := parts[1]

			objectType := ""
			objectName := ""
			if name != "pod_status" {
				parts = strings.Split(name, "|")
				if len(parts) == 1 {
					objectType = config.TypePods
					objectName = parts[0]
				} else if len(parts) == 2 {
					objectType = parts[0]
					objectName = parts[1]
				} else {
					fmt.Fprintln(os.Stdout, util.State[3], "Invalid icinga host.name")
					os.Exit(3)
				}
			}

			checkCount := cmd.Flag("count").Changed
			checkPodExists(&req, namespace, objectType, objectName, checkCount)
		},
	}
	c.Flags().StringVarP(&req.host, "host", "H", "", "Icinga host name")
	c.Flags().IntVarP(&req.count, "count", "c", 0, "Number of Kubernetes Node")
	return c
}
