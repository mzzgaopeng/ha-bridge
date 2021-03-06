package failover

import (
	"github.com/google/gopacket/pcap"
	"ha-bridge/pkg/garp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "kubevirt.io/client-go/api/v1"
	"log"
	"net"
	"strings"
	"time"
)

var VirtInformer *cache.SharedIndexInformer

const indexName = "node"

const bridgeNetwork = "vlan"

const broadcastMacStr = "ff:ff:ff:ff:ff:ff"

var hostname = "10.100.100.148-share"

func OnBondFailOver() {
	vmList := getAllLocalVMList()
	handleVMI(vmList)

}
func sendGarp(macstr, ipstr, linkBridgeOnHost string) {
	handle, err := pcap.OpenLive(linkBridgeOnHost, 65536, true, 3*time.Millisecond)
	if err != nil {
		log.Fatal(err)
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
func getBridgeOnHOst() string {
	return ""
}

func handleVMI(vmList []v1.VirtualMachineInstance) {
	for _, vm := range vmList {
		for _, intf := range vm.Status.Interfaces {
			if strings.Contains(intf.Name, bridgeNetwork) {
				mac := intf.MAC
				ip := intf.IP
				linkBridgeOnHost := getBridgeOnHOst()
				sendGarp(mac, ip, linkBridgeOnHost)
			}
		}
	}
}
