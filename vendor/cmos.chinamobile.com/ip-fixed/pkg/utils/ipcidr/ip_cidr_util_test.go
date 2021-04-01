package ipcidr

import (
	"cmos.chinamobile.com/ip-fixed/pkg/utils/inslice"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestCIDR_1(t *testing.T) {
	cidr, err := NewCIDR("192.168.2.0/24")
	if err != nil {
		panic(err)
	}
	fmt.Println(cidr.GetIPStr())
	fmt.Printf("IP Count: %d\n", cidr.ipCount)
	fmt.Printf("Available IP Count: %d\n", cidr.availableIPCount)
	fmt.Println(cidr.Contains("192.168.2.2"))
	fmt.Printf("Broadcast: %s\n", cidr.GetBroadcast())
	fmt.Println(cidr.GetCIDRStr())
	fmt.Println(cidr.GetMinAvailableIP())

	fmt.Println(cidr.IPNet.Mask)
	fmt.Println(net.CIDRMask(24, 32))

	fmt.Println(cidr.GetAvailableNumIP(2))
	fmt.Println(cidr.GetAvailableIPNum("192.168.2.2"))
	fmt.Println("-------------------------------------")

	var allocationsIP []string
	if err := cidr.ForEachIP(func(ip string) error {
		// do something
		allocationsIP = append(allocationsIP, ip)
		return nil
	}); err != nil {
		fmt.Println(err)
	}

	fmt.Println("-------------------------------------")
	for k, v := range allocationsIP {
		fmt.Printf("%d : %s\n", k, v)
	}
}

func TestCIDR_2(t *testing.T) {
	var (
		cidrStr    = "192.168.2.0/24"
		excludeIPs = []string{
			"192.168.2.0",
			"192.168.2.2",
			"192.168.2.3",
			"192.168.2.255",
		}
		unallocated = []int{}
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}
	ipMap := cidr.GetAvailableIPMap()

	allocations := make([]*int, len(ipMap))
	inFunc := inslice.InStringSliceMapKeyFunc(excludeIPs)

	//fmt.Printf("test %s\n",ipMap[int64(0)])//test 192.168.2.1
	//fmt.Printf("test %s\n",ipMap[int64(252)])//test 192.168.2.253
	//fmt.Printf("test %s\n",ipMap[int64(253)])//test 192.168.2.254 最后有效
	//fmt.Printf("test %s\n",ipMap[int64(254)])//test

	for i := 0; i < len(ipMap)-1; i++ {
		ipStr, _ := ipMap[int64(i)]
		if inFunc(ipStr) {
			var value = i
			allocations[i] = &value
		} else {
			allocations[i] = nil
			unallocated = append(unallocated, i)
		}
	}

	for k, v := range allocations {
		if v != nil {
			fmt.Println(k, *v)
		}
	}

	fmt.Println("-----------------------------------")

	for k, v := range unallocated {
		fmt.Println(k, v)
	}
}

func TestCIDR_3(t *testing.T) {
	var (
		cidrStr    = "192.168.2.0/24"
		excludeIPs = []string{
			//"192.168.2.0",
			"192.168.2.2",
			"192.168.2.3",
			//"192.168.2.255",
		}
		unallocated = []int{}
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}

	allocations := make([]*int, cidr.availableIPCount)
	inFunc := inslice.InStringSliceMapKeyFunc(excludeIPs)

	cidr.ForEachAvailableIPAndIndex(func(index int64, ipStr string) error {
		if inFunc(ipStr) {
			var value = int(index)
			allocations[index] = &value
		} else {
			allocations[index] = nil
			unallocated = append(unallocated, int(index))
		}
		return nil
	})

	for k, v := range allocations {
		if v != nil {
			fmt.Println(k, *v)
		}
	}

	fmt.Println("-----------------------------------")

	for k, v := range unallocated {
		fmt.Println(k, v)
	}
}

func TestCIDR_4(t *testing.T) {
	var (
		cidrStr = "192.168.2.0/24"
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}

	fmt.Println(cidr.GetNumIP(0))             //192.168.2.0
	fmt.Println(cidr.GetNumIP(256))           //error
	fmt.Println(cidr.GetIPNum("192.168.2.0")) //0
	fmt.Println(cidr.GetIPNum("192.168.3.0")) //error
	fmt.Println(cidr.ipCount)
	fmt.Println(cidr.availableIPCount)

	fmt.Println("------------------------------------------")

	fmt.Println(cidr.GetAvailableNumIP(0))               //192.168.2.1
	fmt.Println(cidr.GetAvailableNumIP(253))             //192.168.2.254
	fmt.Println(cidr.GetAvailableNumIP(254))             //error
	fmt.Println(cidr.GetAvailableIPNum("192.168.2.1"))   //0
	fmt.Println(cidr.GetAvailableIPNum("192.168.2.254")) //253
	fmt.Println(cidr.GetAvailableIPNum("192.168.2.0"))   //error
	fmt.Println(cidr.GetAvailableIPNum("192.168.2.255")) //error
}

func BenchmarkNewCIDR(b *testing.B) {
	b.ReportAllocs()
	var (
		cidrStr = "192.168.2.0/24"
	)

	for i := 0; i < b.N; i++ {
		_, err := NewCIDR(cidrStr)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkForEachCIDR_1(b *testing.B) {
	b.ReportAllocs()
	var (
		cidrStr    = "192.168.2.0/24"
		excludeIPs = []string{
			"192.168.2.0",
			"192.168.2.2",
			"192.168.2.3",
			"192.168.2.255",
		}
		unallocated = []int{}
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ipMap := cidr.GetAvailableIPMap()

		allocations := make([]*int, len(ipMap))
		inFunc := inslice.InStringSliceMapKeyFunc(excludeIPs)

		for i := 0; i < len(ipMap)-1; i++ {
			ipStr := ipMap[int64(i)]
			if inFunc(ipStr) {
				var value = i
				allocations[i] = &value
			} else {
				allocations[i] = nil
				unallocated = append(unallocated, i)
			}
		}
	}
}

func BenchmarkForEachCIDR_2(b *testing.B) {
	b.ReportAllocs()
	var (
		cidrStr    = "192.168.2.0/24"
		excludeIPs = []string{
			"192.168.2.0",
			"192.168.2.2",
			"192.168.2.3",
			"192.168.2.255",
		}
		unallocated = []int{}
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allocations := make([]*int, cidr.availableIPCount)
		inFunc := inslice.InStringSliceMapKeyFunc(excludeIPs)

		cidr.ForEachAvailableIPAndIndex(func(index int64, ipStr string) error {
			if inFunc(ipStr) {
				var value = int(index)
				allocations[index] = &value
			} else {
				allocations[index] = nil
				unallocated = append(unallocated, int(index))
			}
			return nil
		})
	}
}

func BenchmarkNewCIDR_FOREACH(b *testing.B) {
	var (
		cidrStr = "192.168.2.0/24"
	)

	cidr, err := NewCIDR(cidrStr)
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cidr.ForEachIP(func(ip string) error {
			// do something
			return nil
		}); err != nil {
			fmt.Println(err)
		}
	}
}

func BenchmarkNewCIDR_SPLIT(b *testing.B) {
	cidrStr := "192.168.2.0/192.168.2.0/24"

	for i := 0; i < b.N; i++ {
		str := cidrStr[strings.LastIndex(cidrStr, "/")+1:]
		if str == "24" {
			continue
		}
	}
}
