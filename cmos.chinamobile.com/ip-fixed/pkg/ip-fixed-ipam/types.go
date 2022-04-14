package ip_fixed_ipam

import (
	"encoding/json"
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"os"
)

// const of resources
const (
	ResourcesPod                    string = "pods"
	ResourcesStatefulSet            string = "statefulsets"
	ResourcesVirtualMachine         string = "virtualmachines"
	ResourcesVirtualMachineInstance string = "virtualmachineinstances"
)

// const of kind
const (
	KindPod                    string = "Pod"
	KindStatefulSet            string = "StatefulSet"
	KindVirtualMachine         string = "VirtualMachine"
	KindVirtualMachineInstance string = "VirtualMachineInstance"
)

const (
	AssignIPPoolAnnotation     string = "cmos.ippool"
	AssignIPAnnotation         string = "cmos.ip"
	IPRecorderNamePrefix       string = "k8s-pod-network"
	IPRecorderNameSeparator    string = "."
	IPRecorderLabelIPPoolValue string = "ippool"
)

// const of IPAMConfig
const (
	defaultRetry          int    = 10
	defaultKubeConfigPath string = "/etc/cni/net.d/ip-fixed-ipam/config"

	defaultLogLevel      string = "info"
	defaultLogFilePath   string = "/var/log/ip-fixed-ipam/ip-fixed-ipam.log"
	defaultLogMaxSize    int    = 1024
	defaultLogMaxBackups int    = 0
	defaultLogMaxAge     int    = 3
)

// The top-level network config - IPAM plugins are passed the full configuration
// of the calling plugin, not just the IPAM section.
type Net struct {
	Name       string      `json:"name"`
	CNIVersion string      `json:"cniVersion"`
	IPAM       *IPAMConfig `json:"ipam"`
}

type IPAMConfig struct {
	Type           string     `json:"type"`
	Retry          int        `json:"retry"`
	KubeConfigPath string     `json:"kubeConfigPath"`
	LogConfig      *LogConfig `json:"logConfig,omitempty"`
}

type LogConfig struct {
	LogLevel      string `json:"logLevel,omitempty"`
	LogFilePath   string `json:"logFilePath,omitempty"`
	LogMaxSize    int    `json:"logMaxSize,omitempty"`
	LogMaxBackups int    `json:"logMaxBackups,omitempty"`
	LogMaxAge     int    `json:"logMaxAge,omitempty"`
}

type K8SArgs struct {
	types.CommonArgs
	K8S_POD_NAMESPACE          types.UnmarshallableString
	K8S_POD_NAME               types.UnmarshallableString
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
}

func LoadIPAMConfig(bytes []byte) (*IPAMConfig, string, error) {
	n := &Net{
		IPAM: &IPAMConfig{
			Retry:          defaultRetry,
			KubeConfigPath: defaultKubeConfigPath,
			LogConfig: &LogConfig{
				LogLevel:      defaultLogLevel,
				LogFilePath:   defaultLogFilePath,
				LogMaxSize:    defaultLogMaxSize,
				LogMaxBackups: defaultLogMaxBackups,
				LogMaxAge:     defaultLogMaxAge,
			},
		},
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, "", fmt.Errorf("failed to load IPAM config: %v", err)
	}
	return n.IPAM, n.CNIVersion, nil
}

type VlanResult interface {
	cnitypes.Result
	Vlan() int
	ConvertToResult() (cnitypes.Result, error)
}

func NewVlanResult() *vlanResult {
	return &vlanResult{}
}

type vlanResult struct {
	current.Result
	VlanNum int `json:"vlan,omitempty"`
}

func (r *vlanResult) Print() error {
	data, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func (r *vlanResult) Vlan() int {
	return r.VlanNum
}

func (r *vlanResult) ConvertToResult() (cnitypes.Result, error) {
	return r.Convert()
}
