package failover

import (
	v2 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	"fmt"
	"github.com/google/gopacket/pcap"
	"ha-bridge/pkg/garp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "kubevirt.io/client-go/api/v1"
	"net"
	"strconv"
	"time"
)

var VirtInformer cache.SharedIndexInformer
var IpamInformer cache.SharedIndexInformer

const indexName = "node"

const broadcastMacStr = "ff:ff:ff:ff:ff:ff"

var HOST_NAME string

func OnBondFailOver() {
	klog.Infoln("bond fail over.....")
	vmList := getAllLocalVMList()
	if vmList == nil || len(vmList) == 0 {
		klog.Infof("can not find vmi on node %s", HOST_NAME)

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

func sendNdp(macstr, ip6str, linkBridgeOnHost string) {

	ndp, err := garp.NewNDPResponder(linkBridgeOnHost, macstr)
	if err != nil {
		klog.Fatalf("failed to create new NDP Responder")
	}
	if ndp != nil {
		defer ndp.Close()
	}
	ndp.SendGratuitous(ip6str)

}

func getAllLocalVMList() []v1.VirtualMachineInstance {
	var result []v1.VirtualMachineInstance
	obj, err := VirtInformer.GetIndexer().ByIndex(indexName, HOST_NAME)

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

func getipvlan(ip string) (string, error) {
	obj, err := IpamInformer.GetIndexer().ByIndex("ipaddress", ip)
	if err != nil {
		klog.Fatal(err)
	}
	if len(obj) == 1 {
		result := strconv.Itoa(obj[0].(*v2.IPRecorder).IPLists[0].Vlan)
		return result, nil
	} else {
		return ip, fmt.Errorf("coun't find ip with vmi")
	}
}

//TODO get vlan with ip from ipam cr
func getBridgeOnHOst(ip string) string {
	vlanid, err := getipvlan(ip)
	if err != nil {
		klog.Errorln(err)
	}
	result := fmt.Sprint("vlan", vlanid)
	return result
}

func handleVMI(vmList []v1.VirtualMachineInstance) {
	for _, vm := range vmList {
		klog.Infoln("get vm  ", vm.Name)
		for _, intf := range vm.Status.Interfaces {
			if intf.InterfaceName == "eth0" {
				//if strings.Contains(intf.InterfaceName, "eth") {
				klog.Infoln("get vm has eth0  ", vm.Name)
				mac := intf.MAC
				hasVlanip := intf.IP
				ip := intf.IPs
				linkBridgeOnHost := getBridgeOnHOst(hasVlanip)
				for _, vmip := range ip {
					Ipfamily := ipfamily(vmip)
					switch Ipfamily {
					case 4:
						go sendGarp(mac, vmip, linkBridgeOnHost)
					case 6:
						go sendNdp(mac, vmip, linkBridgeOnHost)

					}
				}
				//linkBridgeOnHost := getBridgeOnHOst(hasVlanip)
				//go sendGarp(mac, hasVlanip, linkBridgeOnHost)
			}
		}
	}
}

func ipfamily(s string) int {
	ip := net.ParseIP(s)
	if ip == nil {
		return 0
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return 4
		case ':':
			if s[0:4] != "fe80" {
				return 6
			}
		}
	}
	return 0
}

