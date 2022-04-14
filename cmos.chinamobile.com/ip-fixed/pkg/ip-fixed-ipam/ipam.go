package ip_fixed_ipam

import (
	"cmos.chinamobile.com/ip-fixed/pkg/utils/log"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"go.uber.org/zap"
	"net"
)

//func init() {
//	// this ensures that main runs only on main thread (thread group leader).
//	// since namespace ops (unshare, setns) are done for a single thread, we
//	// must ensure that the goroutine does not jump from OS thread to thread
//	runtime.LockOSThread()
//}

func Main() {
	skel.PluginMain(cmdAdd, nil, cmdDel, version.All, bv.BuildString("ip-fixed-ipam"))
}

func initLogger(logConfig *LogConfig) *zap.Logger {
	logger = log.InitLogger(logConfig.LogLevel, logConfig.LogFilePath, logConfig.LogMaxSize, logConfig.LogMaxBackups, logConfig.LogMaxAge, true)
	zap.ReplaceGlobals(logger)
	return logger
}

// cmdAdd is called for ADD requests
func cmdAdd(args *skel.CmdArgs) error {
	ipamConf, _, err := LoadIPAMConfig(args.StdinData)
	if err != nil {
		return err
	}
	logger = initLogger(ipamConf.LogConfig)
	defer logger.Sync()
	logger.Info("cmdAdd.", zap.Any("args", args))

	k8sArgs := &K8SArgs{}
	if err = cnitypes.LoadArgs(args.Args, k8sArgs); err != nil {
		logger.Error("load k8sArgs error!", zap.Any("args", args))
		return err
	}

	allocator, err := NewAllocator(ipamConf.KubeConfigPath, k8sArgs, ipamConf.Retry)
	if err != nil {
		return err
	}
	ipInfo, err := allocator.AssignIP()
	if err != nil {
		return err
	}

	return getVlanResult(ipInfo).Print()
}

func getVlanResult(ipInfo *IPInfo) *vlanResult {
	result := NewVlanResult()
	// TODO 适配IPv6, 当前CIDR字符串中网络前缀所占位数固定为24
	result.IPs = append(result.IPs, &current.IPConfig{
		Version: "4",
		Address: net.IPNet{
			IP: net.ParseIP(ipInfo.IP),
			//CIDR字符串中网络前缀所占位数固定24
			Mask: net.CIDRMask(24, 32),
		},
		Gateway: net.ParseIP(ipInfo.Gateway),
	})

	ip, ipNet, _ := net.ParseCIDR("0.0.0.0/0")
	ipNet.IP = ip
	result.Routes = append(result.Routes, &cnitypes.Route{
		Dst: *ipNet,
		GW:  net.ParseIP(ipInfo.Gateway),
	})
	result.VlanNum = ipInfo.Vlan
	return result
}

// cmdDel is called for DELETE requests
func cmdDel(args *skel.CmdArgs) error {
	ipamConf, _, err := LoadIPAMConfig(args.StdinData)
	if err != nil {
		return err
	}
	logger = initLogger(ipamConf.LogConfig)
	defer logger.Sync()
	logger.Info("cmdDel.", zap.Any("args", args))

	k8sArgs := &K8SArgs{}
	if err = cnitypes.LoadArgs(args.Args, k8sArgs); err != nil {
		logger.Error("load k8sArgs error!", zap.Any("args", args))
		return err
	}

	allocator, err := NewAllocator(ipamConf.KubeConfigPath, k8sArgs, ipamConf.Retry)
	if err != nil {
		return err
	}
	err = allocator.ReleaseIP()
	if err != nil {
		return err
	}

	return nil
}
