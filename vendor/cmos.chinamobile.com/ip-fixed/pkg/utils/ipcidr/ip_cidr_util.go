package ipcidr

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
)

/*
	https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing
	CIDR表示法:
	IPv4   	网络号/前缀长度		192.168.1.0/24  CIDR字符串中网络前缀所占位数NumberOfNetworkPrefixes-24
	IPv6	接口号/前缀长度		2001:db8::/64
*/
//TODO 考虑重构CIDR结构体, 注释部分初始化时赋值
type CIDR struct {
	IP      net.IP
	IPNet   *net.IPNet
	cidrStr string //cidrStr: 192.168.1.0/24
	//mask             string //子网掩码
	ipCount          int64  //IP总数: 256
	availableIPCount int64  //可用IP总数: 254
	minAvailableIP   string //最小可用IP: 192.168.1.1
	maxAvailableIP   string //最大可用IP: 192.168.1.254
	network          string //网络号: 192.168.1.0
	broadcast        string //广播地址: 192.168.1.255
}

// 解析CIDR网段
func NewCIDR(cidrStr string) (*CIDR, error) {
	ip, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, err
	}
	ones, bits := ipNet.Mask.Size()
	ipCount := big.NewInt(0).Lsh(big.NewInt(1), uint(bits-ones)).Int64()
	availableIPCount := ipCount - 2
	network := ipNet.IP.String()
	minAvailableIPNum := big.NewInt(0).Add(ipToBigInt(ipNet.IP), big.NewInt(1))
	maxAvailableIPNum := big.NewInt(0).Add(ipToBigInt(ipNet.IP), big.NewInt(availableIPCount))
	//broadcastNum := big.NewInt(0).Add(ipToBigInt(ipNet.IP), big.NewInt(availableIPCount+1))
	ipMask := ipNet.Mask
	bcst := make(net.IP, len(ipNet.IP))
	copy(bcst, ipNet.IP)
	for i := 0; i < len(ipMask); i++ {
		ipIdx := len(bcst) - i - 1
		bcst[ipIdx] = ipNet.IP[ipIdx] | ^ipMask[len(ipMask)-i-1]
	}
	broadcast := bcst.String()

	return &CIDR{
		IP:               ip,
		IPNet:            ipNet,
		cidrStr:          cidrStr,
		ipCount:          ipCount,
		availableIPCount: availableIPCount,
		minAvailableIP:   bigIntToIP(minAvailableIPNum).String(),
		maxAvailableIP:   bigIntToIP(maxAvailableIPNum).String(),
		network:          network,
		//broadcast: bigIntToIP(broadcastNum).String(),
		broadcast: broadcast,
	}, nil
}

// 根据子网掩码长度校准后的CIDR
func (c CIDR) GetCIDRStr() string {
	return c.IPNet.String()
}

// CIDR字符串中的IP部分
func (c CIDR) GetIPStr() string {
	return c.IP.String()
}

//// 网关(网段第二个IP)
//func (c CIDR) GetGateway() (gateway string) {
//	gw := make(net.IP, len(c.ipNet.IP))
//	copy(gw, c.ipNet.IP)
//	for step := 0; step < 2 && c.ipNet.Contains(gw); step++ {
//		gateway = gw.String()
//		IncrIP(gw)
//	}
//	return
//}
// 最小可用IP(网关gateway, 即网段第二个IP)
func (c CIDR) GetMinAvailableIP() string {
	return c.minAvailableIP
}

// 获取IP总数量
func (c CIDR) GetIPCount() int64 {
	return c.ipCount
}

// 获取有效IP数量
func (c CIDR) GetAvailableIPCount() int64 {
	return c.availableIPCount
}

// 获取网络号(网段第一个IP)
func (c CIDR) GetNetwork() string {
	return c.network
}

// 获取广播地址(网段最后一个IP)
func (c CIDR) GetBroadcast() string {
	return c.broadcast
}

// 起始IP、结束IP(网络号与广播地址)
func (c CIDR) IPRange() (start, end string) {
	return c.network, c.broadcast
}

// 子网掩码
func (c CIDR) GetMask() string {
	mask, _ := hex.DecodeString(c.IPNet.Mask.String())
	return net.IP([]byte(mask)).String()
}

// 子网掩码位数
func (c CIDR) GetMaskSize() (ones, bits int) {
	ones, bits = c.IPNet.Mask.Size()
	return
}

// 判断网段是否相等
func (c CIDR) Equal(cidrStr string) bool {
	c2, err := NewCIDR(cidrStr)
	if err != nil {
		return false
	}
	return c.IPNet.IP.Equal(c2.IPNet.IP) /* && c.ipNet.IP.Equal(c2.ip) */
}

// 判断是否IPv4
func (c CIDR) IsIPv4() bool {
	_, bits := c.IPNet.Mask.Size()
	return bits/8 == net.IPv4len
}

// 判断是否IPv6
func (c CIDR) IsIPv6() bool {
	_, bits := c.IPNet.Mask.Size()
	return bits/8 == net.IPv6len
}

// 判断IP是否包含在网段中
func (c CIDR) Contains(ipStr string) bool {
	return c.IPNet.Contains(net.ParseIP(ipStr))
}

// 遍历网段下所有IP
func (c CIDR) ForEachIP(iterator func(ip string) error) error {
	ipCount := c.ipCount
	next := make(net.IP, len(c.IPNet.IP))
	copy(next, c.IPNet.IP)
	for i := int64(0); i < ipCount; i++ {
		ipStr := next.String()
		if err := iterator(ipStr); err != nil {
			return err
		}
		IncrIP(next)
	}
	return nil
}

// 从指定IP开始遍历网段下后续的IP
func (c CIDR) ForEachIPBeginWith(beginIP string, iterator func(ip string) error) error {
	next := net.ParseIP(beginIP)
	for c.IPNet.Contains(next) {
		if err := iterator(next.String()); err != nil {
			return err
		}
		IncrIP(next)
	}
	return nil
}

// 遍历网段下所有可用IP, 带索引位
func (c CIDR) ForEachAvailableIPAndIndex(iterator func(index int64, ip string) error) error {
	availableIPCount := c.availableIPCount
	next := make(net.IP, len(c.IPNet.IP))
	copy(next, c.IPNet.IP)
	for i := int64(0); i < availableIPCount; i++ {
		IncrIP(next)
		ipStr := next.String()
		if err := iterator(i, ipStr); err != nil {
			return err
		}
	}
	return nil
}

//除Network网络号(网段第一个IP), 广播地址(网段最后一个IP)外, 其余IP地址存入Map: key为序号, value为IP, 例如:
//cidr: 192.168.2.0/24 ,则key: 0-value: 192.168.2.1
func (c CIDR) GetAvailableIPMap() map[int64]string {
	availableIPCount := c.availableIPCount
	ipMap := make(map[int64]string, availableIPCount)
	next := make(net.IP, len(c.IPNet.IP))
	copy(next, c.IPNet.IP)
	for i := int64(0); i < availableIPCount && c.IPNet.Contains(next); i++ {
		IncrIP(next) //第一次循环跳过Network网络号(网段第一个IP)
		ipMap[i] = next.String()
	}
	return ipMap
}

// cidr: 192.168.2.0/24 ,则 num=0 获得192.168.2.0. num索引超过最大ip数将返回error.
func (c *CIDR) GetNumIP(num int64) (string, error) {
	if num >= c.ipCount {
		return "", errors.New(fmt.Sprintf("Maximum IP count exceeded, IP count: %d", c.ipCount))
	}
	minIPNum := ipToBigInt(net.ParseIP(c.network))
	ipNum := minIPNum.Add(minIPNum, big.NewInt(num))
	return bigIntToIP(ipNum).String(), nil
}

// cidr: 192.168.2.0/24 ,则 ipStr=192.168.2.0 获得0. ipStr不在网段内将返回error.
func (c *CIDR) GetIPNum(ipStr string) (int64, error) {
	if !c.Contains(ipStr) {
		return -1, errors.New("The network segment does not contain this IP: " + ipStr)
	}
	networkNum := ipToBigInt(net.ParseIP(c.network))
	ipNum := ipToBigInt(net.ParseIP(ipStr))
	return big.NewInt(0).Sub(ipNum, networkNum).Int64(), nil
}

// 可用IP. cidr: 192.168.2.0/24 ,则 num=0 获得192.168.2.1. num索引超过最大可用ip数将返回error.
func (c *CIDR) GetAvailableNumIP(num int64) (string, error) {
	if num >= c.availableIPCount {
		return "", errors.New(fmt.Sprintf("Maximum Available IP count exceeded, Available IP count: %d", c.availableIPCount))
	}
	minIPNum := ipToBigInt(net.ParseIP(c.network))
	ipNum := minIPNum.Add(minIPNum, big.NewInt(num+1))
	return bigIntToIP(ipNum).String(), nil
}

// 可用IP. cidr: 192.168.2.0/24 ,则 ipStr=192.168.2.1 获得0. ipStr不是可用ip将返回error.
func (c *CIDR) GetAvailableIPNum(ipStr string) (int64, error) {
	if !c.Contains(ipStr) || ipStr == c.network || ipStr == c.broadcast {
		return -1, errors.New("The network available segment does not contain this IP: " + ipStr)
	}
	networkNum := ipToBigInt(net.ParseIP(c.network))
	ipNum := ipToBigInt(net.ParseIP(ipStr))
	return big.NewInt(0).Sub(ipNum, networkNum).Int64() - 1, nil
}

func ipToBigInt(ip net.IP) *big.Int {
	if ip.To4() != nil {
		return big.NewInt(0).SetBytes(ip.To4())
	} else {
		return big.NewInt(0).SetBytes(ip.To16())
	}
}

func bigIntToIP(ipInt *big.Int) net.IP {
	return net.IP(ipInt.Bytes())
}

// 裂解子网的方式
const (
	SubnettingMethodSubnetNum = 0 // 基于子网数量
	SubnettingMethodHostNum   = 1 // 基于主机数量
)

// 裂解网段
func (c CIDR) SubNetting(method, num int) ([]*CIDR, error) {
	if num < 1 || (num&(num-1)) != 0 {
		return nil, fmt.Errorf("裂解数量必须是2的次方")
	}

	newOnes := int(math.Log2(float64(num)))
	ones, bits := c.GetMaskSize()
	switch method {
	default:
		return nil, fmt.Errorf("不支持的裂解方式")
	case SubnettingMethodSubnetNum:
		newOnes = ones + newOnes
		// 如果子网的掩码长度大于父网段的长度，则无法裂解
		if newOnes > bits {
			return nil, nil
		}
	case SubnettingMethodHostNum:
		newOnes = bits - newOnes
		// 如果子网的掩码长度小于等于父网段的掩码长度，则无法裂解
		if newOnes <= ones {
			return nil, nil
		}
		// 主机数量转换为子网数量
		num = int(math.Pow(float64(2), float64(newOnes-ones)))
	}

	cidrs := []*CIDR{}
	network := make(net.IP, len(c.IPNet.IP))
	copy(network, c.IPNet.IP)
	for i := 0; i < num; i++ {
		cidr, _ := NewCIDR(fmt.Sprintf("%v/%v", network.String(), newOnes))
		cidrs = append(cidrs, cidr)

		// 广播地址的下一个IP即为下一段的网络号
		network = net.ParseIP(cidr.broadcast)
		IncrIP(network)
	}

	return cidrs, nil
}

// 合并网段
func SuperNetting(ns []string) (*CIDR, error) {
	num := len(ns)
	if num < 1 || (num&(num-1)) != 0 {
		return nil, fmt.Errorf("子网数量必须是2的次方")
	}

	mask := ""
	cidrs := []*CIDR{}
	for _, n := range ns {
		// 检查子网CIDR有效性
		c, err := NewCIDR(n)
		if err != nil {
			return nil, fmt.Errorf("网段%v格式错误", n)
		}
		cidrs = append(cidrs, c)

		// TODO 暂只考虑相同子网掩码的网段合并
		if len(mask) == 0 {
			mask = c.GetMask()
		} else if c.GetMask() != mask {
			return nil, fmt.Errorf("子网掩码不一致")
		}
	}
	AscSortCIDRs(cidrs)

	// 检查网段是否连续
	var network net.IP
	for _, c := range cidrs {
		if len(network) > 0 {
			if !network.Equal(c.IPNet.IP) {
				return nil, fmt.Errorf("必须是连续的网段")
			}
		}
		network = net.ParseIP(c.broadcast)
		IncrIP(network)
	}

	// 子网掩码左移，得到共同的父网段
	c := cidrs[0]
	ones, bits := c.GetMaskSize()
	ones = ones - int(math.Log2(float64(num)))
	c.IPNet.Mask = net.CIDRMask(ones, bits)
	c.IPNet.IP.Mask(c.IPNet.Mask)

	return c, nil
}
