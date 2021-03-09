package failover

import (
	"github.com/google/gopacket/pcap"
	"ha-bridge/pkg/garp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "kubevirt.io/client-go/api/v1"
	"net"
	"strings"
	"time"
)

var VirtInformer cache.SharedIndexInformer

const indexName = "node"

const bridgeNetwork = "kubevirt-bridge"

const broadcastMacStr = "ff:ff:ff:ff:ff:ff"

var hostname = "10.100.100.148-share"

func OnBondFailOver() {
	klog.Infoln("bond fail over.....")
	vmList := getAllLocalVMList()
	if vmList == nil || len(vmList) == 0 {
		klog.Infof("can not find vmi on node %s", hostname)

	}
	handleVMI(vmList)

}

//todo benchmark
func sendGarp(macstr, ipstr, linkBridgeOnHost string) {
	klog.Infof("send gratuitous arp from ip:%s ,mac:%s  on  interface: %s ", ipstr, macstr, linkBridgeOnHost)
	handle, err := pcap.OpenLive(linkBridgeOnHost, 65536, true, 3*time.Millisecond)
	if err != nil {
		klog.Fatal(err)
	}
	defer handle.Close()
	src := net.ParseIP(ipstr)
	mac, err := net.ParseMAC(macstr)
	if err != nil {
		klog.Error(err)
	}
	broadcastMac, err := net.ParseMAC(broadcastMacStr)
	if err != nil {
		klog.Error(err)
	}
	garp.SendAFakeArpRequest(handle, src, src, broadcastMac, mac)
}

func getAllLocalVMList() []v1.VirtualMachineInstance {
	var result []v1.VirtualMachineInstance
	obj, err := VirtInformer.GetIndexer().ByIndex(indexName, hostname)

	if err != nil {
		klog.Fatal(err)
	}
	for i := 0; i < len(obj); i++ {
		if vmi, ok := obj[i].(*v1.VirtualMachineInstance); ok {
			result = append(result, *vmi)
		}
	}
	return result
}

//TODO get vlan with ip from ipam cr
func getBridgeOnHOst() string {
	return "vlan315"
}

func handleVMI(vmList []v1.VirtualMachineInstance) {
	for _, vm := range vmList {
		for _, intf := range vm.Status.Interfaces {
			if strings.Contains(intf.Name, bridgeNetwork) {
				mac := intf.MAC
				ip := intf.IP
				linkBridgeOnHost := getBridgeOnHOst()
				go sendGarp(mac, ip, linkBridgeOnHost)
			}
		}
	}
}
