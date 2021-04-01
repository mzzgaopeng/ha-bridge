package ip_fixed_ipam

import (
	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	ipfixedclientset "cmos.chinamobile.com/ip-fixed/generated/ipfixed/clientset/versioned"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/inslice"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/ipcidr"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/log"
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
	"sync"
	"testing"
)

//测试说明:
// 1. 环境: 需要运行IPPool控制器
// 2. 步骤:
//	测试前需要执行createTestIPPools(),创建测试IPPool: th-ippool01 th-ippool02 th-ippool03
//  测试后可执行deleteTestIPPools(),删除创建的测试IPPool.
const (
	kubeConfigPath   = "D:\\Development\\GOPATH\\src\\cmos.chinamobile.com\\ip-fixed\\config29-15"
	testIPPoolName01 = "th-ippool01"
	testIPPoolName02 = "th-ippool02"
	testIPPoolName03 = "th-ippool03"
	testPodName01    = "th-pod01"
	testPodName02    = "th-pod02"
	testPodName03    = "th-pod03"
)

var (
	testIPPools = []*ipfixedv1alpha1.IPPool{
		&ipfixedv1alpha1.IPPool{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testIPPoolName01,
			},
			Spec: ipfixedv1alpha1.IPPoolSpec{
				Cidr: "192.168.11.0/24",
				Vlan: 11,
				ExcludeIPs: []string{
					"192.168.11.0",
					"192.168.11.1",
					"192.168.11.255",
				},
				Gateway: "192.168.11.1",
			},
		},
		&ipfixedv1alpha1.IPPool{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testIPPoolName02,
			},
			Spec: ipfixedv1alpha1.IPPoolSpec{
				Cidr: "192.168.12.0/24",
				Vlan: 12,
				ExcludeIPs: []string{
					"192.168.12.0",
					"192.168.12.1",
					"192.168.12.2",
					"192.168.12.254",
					"192.168.12.255",
				},
				Gateway: "192.168.12.1",
			},
		},
		&ipfixedv1alpha1.IPPool{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testIPPoolName03,
			},
			Spec: ipfixedv1alpha1.IPPoolSpec{
				Cidr: "192.168.13.0/24",
				Vlan: 13,
				ExcludeIPs: []string{
					"192.168.13.0",
					"192.168.13.1",
					"192.168.13.2",
					"192.168.13.3",
					"192.168.13.253",
					"192.168.13.254",
					"192.168.13.255",
				},
				Gateway: "192.168.13.1",
			},
		},
	}

	podSpec = corev1.PodSpec{
		Containers: []corev1.Container{
			corev1.Container{
				Name:  "nginx",
				Image: "nginx:1.15.10",
				Ports: []corev1.ContainerPort{
					corev1.ContainerPort{
						ContainerPort: 80,
					},
				},
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
		},
	}

	testPods = []*corev1.Pod{
		// 非固定随机分配IP
		&corev1.Pod{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testPodName01,
			},
			Spec: podSpec,
		},
		// 非固定指定IPPool随机分配IP
		&corev1.Pod{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testPodName02,
				Annotations: map[string]string{
					AssignIPPoolAnnotation: testIPPoolName01 + "," + testIPPoolName02,
				},
			},
			Spec: podSpec,
		},
		// 非固定指定分配IP
		&corev1.Pod{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: testPodName03,
				Annotations: map[string]string{
					AssignIPPoolAnnotation: testIPPoolName03,
					AssignIPAnnotation:     "192.168.13.100",
				},
			},
			Spec: podSpec,
		},
	}
)

func getDefaultAllocator() (*Allocator, error) {
	var (
		k8sArgs = &K8SArgs{
			K8S_POD_NAMESPACE:          types.UnmarshallableString("default"),
			K8S_POD_NAME:               types.UnmarshallableString("default-pod"),
			K8S_POD_INFRA_CONTAINER_ID: types.UnmarshallableString("0678b974e5bace7f2b856c4cd375d12d7cc232996e1117b867a33a54d53fe797"),
		}
		retry = 10
	)
	return NewAllocator(kubeConfigPath, k8sArgs, retry)
}

func createTestIPPools() error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return err
	}
	ipfixedClient, err := ipfixedclientset.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	for _, ipPool := range testIPPools {
		_, err := ipfixedClient.IpfixedV1alpha1().IPPools().Create(ipPool)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTestIPPools() error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return err
	}
	ipfixedClient, err := ipfixedclientset.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	for _, ipPool := range testIPPools {
		err := ipfixedClient.IpfixedV1alpha1().IPPools().Delete(ipPool.Name, &k8smetav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestPod() error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	for _, pod := range testPods {
		_, err := kubeClient.CoreV1().Pods(k8smetav1.NamespaceDefault).Create(pod)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTestPod() error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	for _, pod := range testPods {
		err := kubeClient.CoreV1().Pods(k8smetav1.NamespaceDefault).Delete(pod.Name, &k8smetav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func initTestLogger() {
	var (
		logLevel      string = "info"
		filePath      string = "/home/devhan/var/log/ip-fixed-ipam/ip-fixed-ipam.log"
		logMaxSize    int    = 1024
		logMaxBackups int    = 0
		logMaxAge     int    = 3
	)
	logger = log.InitLogger(logLevel, filePath, logMaxSize, logMaxBackups, logMaxAge, true)
	zap.ReplaceGlobals(logger)
}

func TestInit(t *testing.T) {
	if err := createTestIPPools(); err != nil {
		t.Fatal(err)
	}
	if err := createTestPod(); err != nil {
		t.Fatal(err)
	}
}

func TestRelease(t *testing.T) {
	if err := deleteTestIPPools(); err != nil {
		t.Fatal(err)
	}
	if err := deleteTestPod(); err != nil {
		t.Fatal(err)
	}
}

func TestNewAllocator(t *testing.T) {
	var (
		k8sArgs = &K8SArgs{
			K8S_POD_NAMESPACE:          types.UnmarshallableString("default"),
			K8S_POD_NAME:               types.UnmarshallableString("test-pod"),
			K8S_POD_INFRA_CONTAINER_ID: types.UnmarshallableString("qweasdzxc"),
		}
		retry = 10
	)

	_, err := NewAllocator(kubeConfigPath, k8sArgs, retry)
	if err != nil {
		t.Errorf("NewAllocator error.")
	}
}

//TODO
func TestIsPodFixedIP(t *testing.T) {

}

//TODO
func TestGetPodIPInfo(t *testing.T) {

}

func TestGetIPPoolListByAnnotation(t *testing.T) {
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}

	tests := []struct {
		name           string
		ipPoolNamesStr string
		result         int //IPPool数量
	}{
		{
			name:           "Case 1: one IPPool",
			ipPoolNamesStr: testIPPoolName01,
			result:         1,
		},
		{
			name:           "Case 2: two IPPool",
			ipPoolNamesStr: fmt.Sprintf("%s,%s", testIPPoolName01, testIPPoolName02),
			result:         2,
		},
		{
			name:           "Case 3: three IPPool",
			ipPoolNamesStr: fmt.Sprintf("%s,%s,%s", testIPPoolName01, testIPPoolName02, testIPPoolName03),
			result:         3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipPools, err := allocator.getIPPoolListByAnnotation(test.ipPoolNamesStr)
			if err != nil {
				t.Errorf("getIPPoolListByAnnotation() return error: %s", err.Error())
			}
			if len(ipPools) != test.result {
				t.Errorf("error result: got = %d, Want = %d", len(ipPools), test.result)
			}
		})
	}
}

//func (a *Allocator) updateIPPoolDetail(ipPoolDetail *ipfixedv1alpha1.IPPoolDetail, isAssign bool, ipRecorderName string, ipIndex int) (*ipfixedv1alpha1.IPPoolDetail, error) {
// 只需测试 Unallocated Allocations Recorders 是否修改成功
func TestUpdateIPPoolDetail(t *testing.T) {
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}

	var (
		testIPPoolName = testIPPoolName02
	)

	tests := []struct {
		name           string
		isAssign       bool
		ipRecorderName string
		ipIndex        int
	}{
		{
			name:           "Case 1.",
			isAssign:       true,
			ipRecorderName: strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, k8smetav1.NamespaceDefault, "test01"}, IPRecorderNameSeparator),
			ipIndex:        2,
		},
		{
			name:           "Case 2.",
			isAssign:       false,
			ipRecorderName: strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, k8smetav1.NamespaceDefault, "test02"}, IPRecorderNameSeparator),
			ipIndex:        2,
		},
	}

	for _, test := range tests {
		testIPPoolDetail, err := allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Get(testIPPoolName, k8smetav1.GetOptions{})
		if err != nil {
			t.Fatal("get test IPPoolDetail failed.", err)
		}
		t.Run(test.name, func(t *testing.T) {
			result, err := allocator.updateIPPoolDetail(testIPPoolDetail, test.isAssign, test.ipRecorderName, test.ipIndex)
			if err != nil {
				t.Errorf("updateIPPoolDetail() return error: %s", err.Error())
			}
			if test.isAssign {
				//测试分配
				if result.Spec.Allocations[test.ipIndex] == nil {
					t.Error("error result: assign IP, spec.allocations update failed.")
				}
				inIntFunc := inslice.InIntSliceMapKeyFunc(result.Spec.Unallocated)
				if inIntFunc(test.ipIndex) {
					t.Error("error result: assign IP, spec.unallocated update failed.")
				}
				inStringFunc := inslice.InStringSliceMapKeyFunc(result.Spec.Recorders)
				if !inStringFunc(test.ipRecorderName) {
					t.Error("error result: assign IP, spec.recorders update failed.")
				}
			} else {
				//测试释放
				if result.Spec.Allocations[test.ipIndex] != nil {
					t.Error("error result: release IP, spec.allocations update failed.")
				}
				inIntFunc := inslice.InIntSliceMapKeyFunc(result.Spec.Unallocated)
				if !inIntFunc(test.ipIndex) {
					t.Error("error result: release IP, spec.unallocated update failed.")
				}
				inStringFunc := inslice.InStringSliceMapKeyFunc(result.Spec.Recorders)
				if inStringFunc(test.ipRecorderName) {
					t.Error("error result: release IP, spec.recorders update failed.")
				}
			}
		})
	}

	//测试后恢复, 删除IPPoolDetail, 交给IPPool控制器重新生成即可.
	err = allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Delete(testIPPoolName, &k8smetav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("test has ended, recover IPPoolDetail %s failed, error: %s", testIPPoolName, err.Error())
	}
}

//func (a *Allocator) createIPRecorder(ipPool *ipfixedv1alpha1.IPPool, ipRecorderName string, ipIndex int, ipStr string) (*ipfixedv1alpha1.IPRecorder, error) {
func TestCreateIPRecorder(t *testing.T) {

}

//func (a *Allocator) updateIPRecorderToReleasedIP(ipRecorder *ipfixedv1alpha1.IPRecorder, ipListsName string) (*ipfixedv1alpha1.IPRecorder, error) {
func TestUpdateIPRecorderToReleasedIP(t *testing.T) {

}

//TODO
//func (a *Allocator) assignIPFromIPPool(podIPInfo *PodIPInfo) (*IPInfo, error) {
func TestAssignIPFromIPPool_Random(t *testing.T) {
	//IPPools中随机IP分配
	//1.IPPools无可分配IP
	//2.正常修改IPPoolDetail&创建IPRecorder

}

//func (a *Allocator) assignIPFromIPPool(podIPInfo *PodIPInfo) (*IPInfo, error) {
func TestAssignIPFromIPPool_Specify(t *testing.T) {
	initTestLogger()
	//指定IP分配(assignIPStr)
	//1. 指定IP被占用
	//2. 正常修改IPPoolDetail&创建IPRecorder

	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}

	tests := []struct {
		name      string
		podIPInfo *PodIPInfo
		result    bool
	}{
		{
			//will error
			name: "Case 1: 指定IP被占用.",
			podIPInfo: &PodIPInfo{
				ipRecorderName: strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, k8smetav1.NamespaceDefault, "test01"}, IPRecorderNameSeparator),
				irResources:    KindVirtualMachine,
				irNamespace:    k8smetav1.NamespaceDefault,
				irName:         "test01",
				ipPools: []ipfixedv1alpha1.IPPool{
					*testIPPools[0],
				},
				assignIPStr: "192.168.11.1",
			},
			result: false,
		},
		{
			name: "Case 2: 正常修改IPPoolDetail&创建IPRecorder.",
			podIPInfo: &PodIPInfo{
				ipRecorderName: strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, k8smetav1.NamespaceDefault, "test02"}, IPRecorderNameSeparator),
				irResources:    KindVirtualMachine,
				irNamespace:    k8smetav1.NamespaceDefault,
				irName:         "test02",
				ipPools: []ipfixedv1alpha1.IPPool{
					*testIPPools[0],
				},
				assignIPStr: "192.168.11.2",
			},
			result: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := allocator.assignIPFromIPPool(test.podIPInfo)
			if err != nil {
				if !test.result && test.podIPInfo.assignIPStr == "192.168.11.1" {
					t.Log(err)
					return
				}
				t.Errorf("assignIPFromIPPool() return error: %s", err.Error())
			} else {
				if result.IP != test.podIPInfo.assignIPStr {
					t.Errorf("error result: specify ip is = %s, but retrun = %s", test.podIPInfo.assignIPStr, result.IP)
				}
			}
		})
	}

	//测试后恢复, 删除创建的IPRecorder. 删除IPPoolDetail, 交给IPPool控制器重新生成即可.
	for _, v := range tests {
		err = allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(v.podIPInfo.ipRecorderName, &k8smetav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			t.Errorf("test has ended, recover IPRecorder %s failed, err: %s", v.podIPInfo.ipRecorderName, err.Error())
		}
	}
	for _, v := range testIPPools {
		err = allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Delete(v.Name, &k8smetav1.DeleteOptions{})
		if err != nil {
			t.Fatalf("test has ended, recover IPPoolDetail %s failed, error: %s", v.Name, err.Error())
		}
	}
}

func TestAssignIPFromIPPool_Specify_Concurrent(t *testing.T) {
	initTestLogger()
	//并发指定IP分配测试, 并发修改IPPoolDetail&创建IPRecorder
	//runtime.LockOSThread()

	//1) 准备测试数据
	var (
		testIPPool      = testIPPools[0]
		retry           = 10
		concurrency     = 10
		irResources     = ResourcesVirtualMachine
		irNamespace     = k8smetav1.NamespaceDefault
		irNames         = make([]string, concurrency)
		ipRecorderNames = make([]string, concurrency)
	)
	cidr, _ := ipcidr.NewCIDR(testIPPool.Spec.Cidr)
	//concurrency := int(cidr.GetIPCount()) - len(testIPPool.Spec.ExcludeIPs)
	availableIPMap := cidr.GetAvailableIPMap()
	for i := 0; i < concurrency; i++ {
		irNames[i] = fmt.Sprintf("case-%d", i+1)
		ipRecorderNames[i] = strings.Join([]string{IPRecorderNamePrefix, irResources, irNamespace, irNames[i]}, IPRecorderNameSeparator)
	}

	//2) 并发测试
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		podIPInfo := &PodIPInfo{
			ipRecorderName: ipRecorderNames[i],
			irResources:    irResources,
			irNamespace:    irNamespace,
			irName:         irNames[i],
			ipPools: []ipfixedv1alpha1.IPPool{
				*testIPPool,
			},
			assignIPStr: availableIPMap[int64(i+1)],
		}
		go func(t *testing.T, podIPInfo *PodIPInfo) {
			testAllocator, err := getDefaultAllocator()
			testAllocator.retry = retry
			if err != nil {
				t.Errorf("test %d get allocator failed.", i)
				return
			}
			defer wg.Done()
			_, err = testAllocator.assignIPFromIPPool(podIPInfo)
			if err != nil {
				t.Errorf("assignIPFromIPPoolSpecify() return error: %s", err.Error())
			}
		}(t, podIPInfo)
	}
	wg.Wait()

	//3) 测试后恢复, 删除创建的IPRecorder. 删除IPPoolDetail, 交给IPPool控制器重新生成即可.
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("test has ended and recover, but get default allocator failed.", err)
	}
	for _, v := range ipRecorderNames {
		err = allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(v, &k8smetav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			t.Errorf("test has ended, recover IPRecorder %s failed, err: %s", v, err.Error())
		}
	}
	err = allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Delete(testIPPool.Name, &k8smetav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("test has ended, recover IPPoolDetail %s failed, error: %s", testIPPool.Name, err.Error())
	}
}

func TestAssignIPFromIPPool_Random_Concurrent(t *testing.T) {
	initTestLogger()
	// 并发随机分配IP测试, 并发修改IPPoolDetail&创建IPRecorder
	//runtime.LockOSThread()

	//1) 准备测试数据
	var (
		concurrency     = 10
		retry           = 10
		irResources     = ResourcesVirtualMachine
		irNamespace     = k8smetav1.NamespaceDefault
		ipPoolList      = make([]ipfixedv1alpha1.IPPool, len(testIPPools))
		irNames         = make([]string, concurrency)
		ipRecorderNames = make([]string, concurrency)
	)
	for i := 0; i < len(ipPoolList); i++ {
		ipPoolList[i] = *testIPPools[i]
	}
	for i := 0; i < concurrency; i++ {
		irNames[i] = fmt.Sprintf("case-%d", i+1)
		ipRecorderNames[i] = strings.Join([]string{IPRecorderNamePrefix, irResources, irNamespace, irNames[i]}, IPRecorderNameSeparator)
	}

	//2) 并发测试
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		podIPInfo := &PodIPInfo{
			ipRecorderName: ipRecorderNames[i],
			irResources:    irResources,
			irNamespace:    irNamespace,
			irName:         irNames[i],
			ipPools:        ipPoolList,
			assignIPStr:    "",
		}
		go func(t *testing.T, podIPInfo *PodIPInfo) {
			testAllocator, err := getDefaultAllocator()
			testAllocator.retry = retry
			if err != nil {
				t.Errorf("test %d get allocator failed.", i)
				return
			}
			defer wg.Done()
			_, err = testAllocator.assignIPFromIPPool(podIPInfo)
			if err != nil {
				t.Errorf("assignIPFromIPPool() return error: %s", err.Error())
			}
		}(t, podIPInfo)
	}
	wg.Wait()

	//3) 测试后恢复, 删除创建的IPRecorder. 删除IPPoolDetail, 交给IPPool控制器重新生成即可.
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("test has ended and recover, but get default allocator failed.", err)
	}
	for _, v := range ipRecorderNames {
		err = allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(v, &k8smetav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			t.Errorf("test has ended, recover IPRecorder %s failed, err: %s", v, err.Error())
		}
	}
	for _, v := range ipPoolList {
		err = allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Delete(v.Name, &k8smetav1.DeleteOptions{})
		if err != nil {
			t.Fatalf("test has ended, recover IPPoolDetail %s failed, error: %s", v.Name, err.Error())
		}
	}
}

//TODO
//func (a *Allocator) AssignIP() (*IPInfo, error) {
func TestAssignIP(t *testing.T) {
	initTestLogger()
	//1. 从已存在的IPRecorder中分配(固定IP分配)
	//	1.1 指定IP与已存在IPRecorder记录冲突
	//  1.2	不冲突
	//2. 从IPPools中分配(非固定IP分配,或固定IP分配但不存在IPRecorder)
	//  2.1 指定IP分配(被占用&成功)
	//  2.2 pod指定IP池随机分配(cmos.ippool)
	//  2.2 pod随机分配IP

	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}
	//1)准备已存在的IPRecorder, k8sArgs, 固定IP的VM和Pod
	var (
		existsPodIPInfo = &PodIPInfo{
			irResources:    ResourcesVirtualMachine,
			irNamespace:    k8smetav1.NamespaceDefault,
			irName:         "vm-th",
			ipRecorderName: strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, k8smetav1.NamespaceDefault, "vm-th"}, IPRecorderNameSeparator),
		}
		existsIPRecorderIPPool  = testIPPools[0]
		existsIPRecorderIPIndex = 1
		existsIPRecorderIPStr   = "192.168.11.2"

		k8sArgs = &K8SArgs{
			K8S_POD_NAMESPACE:          types.UnmarshallableString(k8smetav1.NamespaceDefault),
			K8S_POD_NAME:               types.UnmarshallableString("virt-launcher-vm-th-dls8k"),
			K8S_POD_INFRA_CONTAINER_ID: types.UnmarshallableString("ebe2483213d8c930d5d1e49b12a066a0a7554447399ba4a76981ae509f2a1fc4"),
		}
	)
	if _, err := allocator.createIPRecorder(existsIPRecorderIPPool, existsPodIPInfo, existsIPRecorderIPIndex, existsIPRecorderIPStr); err != nil {
		t.Fatal("create exists IPRecorder failed.", err)
	}

	//2)准备测试Allocator
	testAllocator, err := NewAllocator(kubeConfigPath, k8sArgs, 10)
	if err != nil {
		allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(existsPodIPInfo.ipRecorderName, &k8smetav1.DeleteOptions{})
		t.Fatal("get default allocator failed.", err)
	}

	//3)准备测试数据
	tests := []struct {
		name       string
		cmosIPPool string
		cmosIP     string
	}{
		{
			//will error
			name:       "Case 1.1: 固定IP,已存在IPRecorder,且指定IP与已存在IPRecorder记录冲突(即不相等).",
			cmosIPPool: existsIPRecorderIPPool.Name,
			cmosIP:     "192.168.11.3",
		},
		{
			name:       "Case 1.2: 固定IP,已存在IPRecorder,且指定IP与已存在IPRecorder记录不冲突.",
			cmosIPPool: existsIPRecorderIPPool.Name,
			cmosIP:     existsIPRecorderIPStr,
		},
		{
			name:       "Case 2.1: 固定IP,不存在IPRecorder,且指定IP与已存在IPRecorder记录不冲突.",
			cmosIPPool: existsIPRecorderIPPool.Name,
			cmosIP:     existsIPRecorderIPStr,
		},
	}

	//4)测试
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			//4.1 根据测试需要更新测试VM的Annotation
			vm, err := testAllocator.kubevirtClient.VirtualMachine(existsPodIPInfo.irNamespace).Get(existsPodIPInfo.irName, &k8smetav1.GetOptions{})
			if err != nil {
				t.Log(test.name, err)
				return
			}
			vm.Annotations[AssignIPPoolAnnotation] = test.cmosIPPool
			if test.cmosIP != "" {
				vm.Annotations[AssignIPAnnotation] = test.cmosIP
			}
			_, err = testAllocator.kubevirtClient.VirtualMachine(existsPodIPInfo.irNamespace).Update(vm)
			if err != nil {
				t.Log(test.name, err)
				return
			}

			//4.2 执行测试
			result, err := testAllocator.AssignIP()
			if err != nil {
				if test.cmosIPPool == existsIPRecorderIPPool.Name && test.cmosIP != existsIPRecorderIPStr {
					t.Log(err)
					return
				}
				t.Errorf("AssignIP() return error: %s", err.Error())
			} else {
				if result.IP != test.cmosIP {
					t.Errorf("error result: specify ip is = %s, but retrun = %s", test.cmosIPPool, result.IP)
				}
			}
		})
	}

	//5)测试后恢复, 删除已存在或后续创建的IPRecorder. 删除IPPoolDetail, 交给IPPool控制器重新生成即可.
	err = allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(existsPodIPInfo.ipRecorderName, &k8smetav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("test has ended, recover IPRecorder %s failed, err: %s", existsPodIPInfo.ipRecorderName, err.Error())
	}
	err = allocator.ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Delete(existsIPRecorderIPPool.Name, &k8smetav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("test has ended, recover IPPoolDetail %s failed, error: %s", existsIPRecorderIPPool.Name, err.Error())
	}
}

//func (a *Allocator) releaseFixedIP(ownerReference *k8smetav1.OwnerReference) error {
func TestReleaseFixedIP(t *testing.T) {
	initTestLogger()
	//释放固定IP
	//1. 已无VM,返回nil
	//2. 存在VM, 获取IPRecorder失败(无IPRecorder即可)
	//3. IPRecorder更新成功
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}

	var (
		ownerReference = &k8smetav1.OwnerReference{
			Kind: KindVirtualMachine,
			Name: "vm-th",
		}
		k8sArgs = &K8SArgs{
			K8S_POD_NAMESPACE:          types.UnmarshallableString(k8smetav1.NamespaceDefault),
			K8S_POD_NAME:               types.UnmarshallableString("virt-launcher-" + ownerReference.Name + "-xxxx"),
			K8S_POD_INFRA_CONTAINER_ID: types.UnmarshallableString("0678b974e5bace7f2b856c4cd375d12d7cc232996e1117b867a33a54d53fe797"),
		}
	)
	//1) 准备测试用的IPRecorder
	var (
		existsIrResources    = ResourcesVirtualMachine
		existsIrNamespace    = string(k8sArgs.K8S_POD_NAMESPACE)
		existsIrName         = ownerReference.Name
		existsIPRecorderName = strings.Join([]string{IPRecorderNamePrefix, existsIrResources, existsIrNamespace, existsIrName}, IPRecorderNameSeparator)
		podIPInfo            = &PodIPInfo{
			irResources:    existsIrResources,
			irNamespace:    existsIrNamespace,
			irName:         existsIrName,
			ipRecorderName: existsIPRecorderName,
		}
		existsIPRecorderIPPool  = testIPPools[0]
		existsIPRecorderIPIndex = 1
		existsIPRecorderIPStr   = "192.168.11.2"
	)
	if _, err := allocator.createIPRecorder(existsIPRecorderIPPool, podIPInfo, existsIPRecorderIPIndex, existsIPRecorderIPStr); err != nil {
		t.Fatal("create test IPRecorder failed.", err)
	}
	allocator.k8sArgs = k8sArgs

	tests := []struct {
		name           string
		ownerReference *k8smetav1.OwnerReference
	}{
		{
			name: "Case 1: 已无VM,返回nil.",
			ownerReference: &k8smetav1.OwnerReference{
				Kind: KindVirtualMachine,
				Name: "vm-fjdasilfjewuia",
			},
		},
		{
			name: "Case 2: 存在VM, 获取IPRecorder失败.",
			ownerReference: &k8smetav1.OwnerReference{
				Kind: KindVirtualMachine,
				Name: "vm-lyf",
			},
		},
	}

	//2) 开始测试(测试后查看日志)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := allocator.releaseFixedIP(test.ownerReference)
			if err != nil {
				if test.ownerReference.Name == "vm-lyf" {
					t.Log(err)
					return
				}
				t.Errorf("releaseFixedIP() return error: %s", err.Error())
			}
		})
	}

	//3) 测试后恢复, 删除创建的测试IPRecorder.
	err = allocator.ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(podIPInfo.ipRecorderName, &k8smetav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		t.Errorf("test has ended, recover IPRecorder %s failed, err: %s", podIPInfo.ipRecorderName, err.Error())
	}
}

//TODO
//func (a *Allocator) releaseUnFixedIP() error {
func TestReleaseUnFixedIP(t *testing.T) {

}

//TODO
func TestReleaseUnFixedIP_Concurrent(t *testing.T) {

}

//TODO
//func (a *Allocator) ReleaseIP() error {
func TestReleaseIP(t *testing.T) {
	//1. 固定IP(VM)
	//2. 非固定IP

}

func TestIPFixedClient(t *testing.T) {
	allocator, err := getDefaultAllocator()
	if err != nil {
		t.Fatal("get default allocator failed.", err)
	}

	var ipfixedClient = allocator.ipfixedClient

	ipPoolList, err := ipfixedClient.IpfixedV1alpha1().IPPools().List(k8smetav1.ListOptions{})
	if err != nil {
		t.Errorf("assign ip: random assign IP, get IPPoolList error: %s", err.Error())
		return
	}
	ipPools := ipPoolList.Items

	fmt.Println(ipPools)
}
