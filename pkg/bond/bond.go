package bond

import (
	"fmt"
	"ha-bridge/pkg/failover"
	"k8s.io/klog/v2"
	"net"
	"strings"
	"syscall"
	"unsafe"
)

const ifaceName = "bond0"

func Start() {
	GetNotifyArp(ifaceName)
}

func GetNotifyArp(bond string) {
	l, err := ListenNetlink()
	if err != nil {
		klog.Error(err)
		return
	}

	for {
		msgs, err := l.ReadMsgs()
		if err != nil {
			klog.Error("Could not read netlink:\n %s", err) // can't find this netlink
		}
	loop:
		for _, m := range msgs {
			switch m.Header.Type {
			case syscall.NLMSG_DONE, syscall.NLMSG_ERROR:
				break loop
			case syscall.RTM_NEWLINK, syscall.RTM_DELLINK: // get netlink message
				res, err := PrintLinkMsg(&m)
				if err != nil {
					klog.Error("Could not find netlink ", err)
				} else {
					ethInfo := strings.Fields(res)
					if ethInfo[2] == bond && ethInfo[1] == "up" {
						failover.OnBondFailOver()
					}
				}
			}

		}
	}
}

func ListenNetlink() (*NetlinkListener, error) { // Listen netlink
	groups := syscall.RTNLGRP_LINK
	//|
	//syscall.RTNLGRP_IPV4_IFADDR |
	//syscall.RTNLGRP_IPV4_ROUTE |
	//syscall.RTNLGRP_IPV6_IFADDR |
	//syscall.RTNLGRP_IPV6_ROUTE

	s, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM,
		syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("socket: %s", err)
	}

	saddr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(0),
		Groups: uint32(groups),
	}

	err = syscall.Bind(s, saddr)
	if err != nil {
		return nil, fmt.Errorf("bind: %s", err)
	}

	return &NetlinkListener{fd: s, sa: saddr}, nil
}

type NetlinkListener struct {
	fd int
	sa *syscall.SockaddrNetlink
}

func (l *NetlinkListener) ReadMsgs() ([]syscall.NetlinkMessage, error) { // read netlink message
	defer func() {
		recover()
	}()

	pkt := make([]byte, 2048)

	n, err := syscall.Read(l.fd, pkt)
	if err != nil {
		return nil, fmt.Errorf("read: %s", err)
	}

	msgs, err := syscall.ParseNetlinkMessage(pkt[:n])
	if err != nil {
		return nil, fmt.Errorf("parse: %s", err)
	}

	return msgs, nil
}

func PrintLinkMsg(msg *syscall.NetlinkMessage) (string, error) { // when netlink changed, function can listen the message and notify user
	defer func() {
		recover()
	}()

	var str, res string
	ifim := (*syscall.IfInfomsg)(unsafe.Pointer(&msg.Data[0]))
	eth, err := net.InterfaceByIndex(int(ifim.Index))
	if err != nil {
		return "", fmt.Errorf("LinkDev %d: %s", int(ifim.Index), err)
	}
	if eth.Flags&syscall.IFF_UP == 1 {
		str = "up"
	} else {
		str = "down"
	}
	if msg.Header.Type == syscall.RTM_NEWLINK {
		res = "NEWLINK: " + str + " " + eth.Name
	} else {
		res = "DELLINK: " + eth.Name
	}

	return res, nil
}

func Print() {
	klog.Warning("bond0 failover......")
}
