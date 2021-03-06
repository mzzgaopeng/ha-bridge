package ha

import (
	"github.com/google/gopacket/pcap"
	"ha-bridge/pkg/fakearp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "kubevirt.io/client-go/api/v1"
	"log"
	"net"
	"strings"
	"time"
)

var virtInformer cache.SharedIndexInformer

const indexName = "node"

const bridgeNetwork = "vlan"

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
	broadcastMac, err := net.ParseMAC("ff:ff:ff:ff:ff:ff")
	if err != nil {
		klog.Error(err)
	}
	fakearp.SendAFakeArpRequest(handle, src, src, broadcastMac, mac)

}

func getAllLocalVMList() []v1.VirtualMachineInstance {
	var result []v1.VirtualMachineInstance
	obj, err := virtInformer.GetIndexer().ByIndex(indexName, hostname)

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
