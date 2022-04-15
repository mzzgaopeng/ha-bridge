package ip_fixed_ipam

import (
	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	ipfixedclientset "cmos.chinamobile.com/ip-fixed/generated/ipfixed/clientset/versioned"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/inslice"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/ipcidr"
	"errors"
	"fmt"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"kubevirt.io/client-go/kubecli"
	"strings"
)

var logger = zap.L()

type Allocator struct {
	kubevirtClient kubecli.KubevirtClient
	ipfixedClient  *ipfixedclientset.Clientset
	k8sArgs        *K8SArgs
	retry          int
}

func NewAllocator(kubeConfigPath string, k8sArgs *K8SArgs, retry int) (*Allocator, error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		logger.Error("build config from flags error", zap.String("kubeConfigPath", kubeConfigPath), zap.Error(err))
		return nil, err
	}

	ipfixedClient, err := ipfixedclientset.NewForConfig(kubeConfig)
	if err != nil {
		logger.Error("build ipfixed clientset error", zap.Error(err))
		return nil, err
	}

	kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(kubeConfig)
	if err != nil {
		logger.Error("build kubevirt client error", zap.Error(err))
		return nil, err
	}

	return &Allocator{
		kubevirtClient: kubevirtClient,
		ipfixedClient:  ipfixedClient,
		k8sArgs:        k8sArgs,
		retry:          retry,
	}, nil
}

// 所分配的IP信息
type IPInfo struct {
	IP      string
	Gateway string
	Vlan    int
}

// 分配IP
func (a *Allocator) AssignIP() (*IPInfo, error) {
	var (
		kubevirtClient  = a.kubevirtClient
		ipfixedClient   = a.ipfixedClient
		k8sPodNamespace = string(a.k8sArgs.K8S_POD_NAMESPACE)
		k8sPodName      = string(a.k8sArgs.K8S_POD_NAME)
	)
	//1. 获取Pod
	pod, err := kubevirtClient.CoreV1().Pods(k8sPodNamespace).Get(k8sPodName, k8smetav1.GetOptions{})
	if err != nil {
		logger.Error("assign ip: get pod error", zap.String("podName", k8sPodName), zap.String("podNamespace", k8sPodNamespace), zap.Error(err))
		return nil, err
	}

	//2. 判断是否为固定IP
	isFixedIP, ownerReference, err := a.isPodFixedIP(pod)
	if err != nil {
		return nil, err
	}

	//3. 根据Pod获取IP分配相关信息
	podIPInfo, err := a.getPodIPInfo(pod, isFixedIP, ownerReference)
	if err != nil {
		return nil, err
	}

	//4. 若为固定IP, 即isFixedIP==true. 则查找是否已存在IPRecorder
	//TODO 未适配StatefulSet, 预计修改点: 添加判断是否存在当前Pod的IPLists, 若存在则修改并返回IPRecorder中分配的IP信息.
	if isFixedIP {
		ipRecorder, err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Get(podIPInfo.ipRecorderName, k8smetav1.GetOptions{})
		if err != nil {
			// 4.1 不存在IPRecorder(k8serrors.IsNotFound()), 继续下一步.
			if !k8serrors.IsNotFound(err) {
				logger.Error("assign ip: get IPRecorder error", zap.String("IPRecorder", podIPInfo.ipRecorderName), zap.Error(err))
				return nil, err
			}
		} else {
			// 4.2 若存在IPRecorder且其中有该Pod的记录, 则从IPRecorder中分配IP, 修改IPRecorder相应的IPLists[].released=false, 并返回IPRecorder中分配的IP信息.
			return a.assignIPFromIPRecorder(ipRecorder, podIPInfo)
		}
	}

	//5. 从IPPool中分配IP, 修改IPPoolDetail并创建IPRecorder.
	return a.assignIPFromIPPool(podIPInfo)
}

// 释放IP
func (a *Allocator) ReleaseIP() error {
	var (
		kubevirtClient  = a.kubevirtClient
		k8sPodNamespace = string(a.k8sArgs.K8S_POD_NAMESPACE)
		k8sPodName      = string(a.k8sArgs.K8S_POD_NAME)
		isFixedIP       = false //是否为固定IP标记
	)

	//1. 获取Pod
	pod, err := kubevirtClient.CoreV1().Pods(k8sPodNamespace).Get(k8sPodName, k8smetav1.GetOptions{})
	if err != nil {
		errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: get pod %s/%s error: %s", k8sPodName, k8sPodNamespace, err)
		logger.Error(errorInfo)
		return errors.New(errorInfo)
	}
	//2. 判断是否为固定IP
	isFixedIP, ownerReference, err := a.isPodFixedIP(pod)
	if err != nil {
		return err
	}

	//3. 调用相应方法释放IP
	if isFixedIP {
		//3.1 固定IP
		return a.releaseFixedIP(ownerReference)
	} else {
		//3.2 非固定IP
		return a.releaseUnFixedIP()
	}
}

// 判断Pod是否为固定IP, 若为固定IP则返回Pod记录IP分配的ownerreference相关信息.
func (a *Allocator) isPodFixedIP(pod *v1.Pod) (bool, *k8smetav1.OwnerReference, error) {
	var (
		kubevirtClient  = a.kubevirtClient
		k8sPodNamespace = string(a.k8sArgs.K8S_POD_NAMESPACE)
		isFixedIP       = false //是否为固定IP标记
	)
	//1. 判断是否为固定IP
	if pod.OwnerReferences != nil {
		ownerReference := pod.OwnerReferences[0]
		switch ownerReference.Kind {
		//1.1 判断是否为虚拟机固定IP
		case KindVirtualMachineInstance:
			vmi, err := kubevirtClient.VirtualMachineInstance(k8sPodNamespace).Get(ownerReference.Name, &k8smetav1.GetOptions{})
			if err != nil {
				errorInfo := fmt.Sprintf("ip-fixed-ipam: get pod ownerreference vmi %s/%s error: %s", ownerReference.Name, k8sPodNamespace, err.Error())
				logger.Error(errorInfo)
				return isFixedIP, nil, errors.New(errorInfo)
			}
			if vmi.OwnerReferences != nil && vmi.OwnerReferences[0].Kind == KindVirtualMachine {
				// 判定为VM固定IP,返回结果
				vmiOwnerReference := vmi.OwnerReferences[0]
				isFixedIP = true
				return isFixedIP, &vmiOwnerReference, nil
			} else {
				//vmi无ownerreferences, 进行非固定IP分配, 可以仅通过vmi创建虚拟机pod.
				isFixedIP = false
			}
		case KindStatefulSet:
			//TODO 暂不考虑StatefulSet固定IP, 应同default.
			fallthrough
		default:
			isFixedIP = false
		}
	}
	//2. 判定为非固定IP, 返回结果
	return isFixedIP, nil, nil
}

// Pod包含的有关IP分配的信息
type PodIPInfo struct {
	//是否为固定IP标记
	isFixedIP bool
	//需要创建的IPRecorder名称, 固定IP格式k8s-pod-network.{resources}.{namespaces}.{name}, 非固定IP格式k8s-pod-network.{containerid}
	ipRecorderName string
	//IPRecorder.IPLists[].resources, 用于记录实际占用IP的资源信息, 例如:
	// 虚拟机vm: virtualmachines, StatefulSet: pods
	irResources string
	//IPRecorder.IPLists[].namespace, 用于记录实际占用IP的资源信息
	irNamespace string
	//IPRecorder.IPLists[].name, 用于记录实际占用IP的资源信息
	irName string
	//指定的IP池集合
	ipPools []ipfixedv1alpha1.IPPool
	//指定分配的IP字符串, 为空则随机分配IP, 不为空则指定IP分配
	assignIPStr string
}

// 结合是否固定IP与Pod的ownerreference中相关注解获取Pod的IP分配相关信息.
func (a *Allocator) getPodIPInfo(pod *v1.Pod, isFixedIP bool, ownerReference *k8smetav1.OwnerReference) (*PodIPInfo, error) {
	var (
		kubevirtClient    = a.kubevirtClient
		ipfixedClient     = a.ipfixedClient
		k8sPodNamespace   = string(a.k8sArgs.K8S_POD_NAMESPACE)
		k8sPodContainerID = string(a.k8sArgs.K8S_POD_INFRA_CONTAINER_ID)
		err               error
		ipRecorderName    string //需要创建的IPRecorder名称, 固定IP格式k8s-pod-network.{resources}.{namespaces}.{name}, 非固定IP格式k8s-pod-network.{containerid}
		irResources       string //IPRecorder.IPLists[].resources, 用于记录实际占用IP的资源信息
		irNamespace       string
		irName            string
		ipPools           []ipfixedv1alpha1.IPPool //指定的IP池集合
		assignIPStr       = ""                     //指定分配的IP字符串, 为空则随机分配IP, 不为空则指定IP分配
	)

	//判断是否为固定IP, 并封装IP分配信息
	if isFixedIP && ownerReference != nil {
		//1. Pod为固定IP, 封装固定IP分配信息
		switch ownerReference.Kind {
		//1.1 虚拟机固定IP
		case KindVirtualMachine:
			vm, err := kubevirtClient.VirtualMachine(k8sPodNamespace).Get(ownerReference.Name, &k8smetav1.GetOptions{})
			if err != nil {
				logger.Error("assign ip: get vmi ownerreference vm error", zap.String("vmName", ownerReference.Name), zap.String("vmNamespace", k8sPodNamespace), zap.Error(err))
				return nil, err
			}
			ipPoolAnnotation, ok := vm.Annotations[AssignIPPoolAnnotation]
			if !ok {
				errorInfo := fmt.Sprintf("assign ip: vm %s/%s has no cmos.ippool annotation", ownerReference.Name, k8sPodNamespace)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}
			ipPools, err = a.getIPPoolListByAnnotation(ipPoolAnnotation)
			if err != nil {
				return nil, err
			}
			ipAnnotation, ok := vm.Annotations[AssignIPAnnotation]
			if ok {
				assignIPStr = ipAnnotation
			}
			//固定IPRecorder格式k8s-pod-network.{resources}.{namespaces}.{name}, 注意{name}应不含"."
			irResources = ResourcesVirtualMachine
			irNamespace = vm.Namespace
			irName = vm.Name
			ipRecorderName = strings.Join([]string{IPRecorderNamePrefix, irResources, irNamespace, irName}, IPRecorderNameSeparator)
		case KindStatefulSet:
			//TODO 暂不考虑StatefulSet固定IP, 应同default.
		}
	} else {
		//2. Pod为非固定IP, 封装非固定IP分配信息
		ipPoolAnnotation, ok := pod.Annotations[AssignIPPoolAnnotation]
		if !ok {
			//2.1 随机分配IP(pod不存在注解AssignIPPoolAnnotation)
			ipPoolList, err := ipfixedClient.IpfixedV1alpha1().IPPools().List(k8smetav1.ListOptions{})
			if err != nil {
				logger.Error("assign ip: random assign IP, get IPPoolList error", zap.Error(err))
				return nil, err
			}
			ipPools = ipPoolList.Items
			if len(ipPools) == 0 {
				errorInfo := "assign ip: error because no IPPool in cluster"
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}
		} else {
			//2.2 指定IPPool分配IP
			//2.2.1 指定IPPool中随机分配
			ipPools, err = a.getIPPoolListByAnnotation(ipPoolAnnotation)
			if err != nil {
				return nil, err
			}
			ipAnnotation, ok := pod.Annotations[AssignIPAnnotation]
			if ok {
				//2.2.2 指定IPPool中分配指定IP
				assignIPStr = ipAnnotation
			}
		}
		//非固定IPRecorder格式k8s-pod-network.{containerid}
		irResources = ResourcesPod
		irNamespace = pod.Namespace
		irName = pod.Name
		ipRecorderName = strings.Join([]string{IPRecorderNamePrefix, k8sPodContainerID}, IPRecorderNameSeparator)
	}

	return &PodIPInfo{
		isFixedIP:      isFixedIP,
		ipRecorderName: ipRecorderName,
		irResources:    irResources,
		irNamespace:    irNamespace,
		irName:         irName,
		ipPools:        ipPools,
		assignIPStr:    assignIPStr,
	}, nil
}

func (a *Allocator) getIPPoolListByAnnotation(ipPoolNamesStr string) (ipPools []ipfixedv1alpha1.IPPool, err error) {
	var (
		ipfixedClient = a.ipfixedClient
	)
	ipPoolNames := strings.Split(ipPoolNamesStr, ",")
	for _, ipPoolName := range ipPoolNames {
		ipPool, err := ipfixedClient.IpfixedV1alpha1().IPPools().Get(ipPoolName, k8smetav1.GetOptions{})
		if err != nil {
			logger.Error("assign ip: get IPPool error", zap.String("IPPool", ipPoolName), zap.Error(err))
			return nil, err
		}
		ipPools = append(ipPools, *ipPool)
	}
	return ipPools, nil
}

// 从IPPool中分配IP(随机or指定)
//TODO 暂不考虑StatefulSet固定IP ,预计修改点: 创建IPRecorder或修改IPRecorder(更新IPLists)
func (a *Allocator) assignIPFromIPPool(podIPInfo *PodIPInfo) (*IPInfo, error) {
	var (
		ipfixedClient  = a.ipfixedClient
		ipRecorderName = podIPInfo.ipRecorderName
		retry          = a.retry
	)

	//分配IP, 即修改IPPoolDetail&创建IPRecorder(具有重试机制)
	for i := 0; i < retry; i++ {
		var (
			ipPoolDetail *ipfixedv1alpha1.IPPoolDetail
			ipPool       *ipfixedv1alpha1.IPPool
			ipIndex      int //分配的IP索引
			cidr         *ipcidr.CIDR
			err          error
		)
		//1. 获取IPPoolDetail及IP索引位
		if podIPInfo.assignIPStr != "" {
			//指定IP分配
			//1.1 根据指定的 IPPool 获取 IPPoolDetail
			ipPool = &podIPInfo.ipPools[0]
			ipPoolDetail, err = ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Get(ipPool.Name, k8smetav1.GetOptions{})
			if err != nil {
				logger.Error("assign ip: get IPPoolDetail error", zap.String("IPPoolDetail", ipPool.Name), zap.Int("retry", i+1), zap.Error(err))
				continue
			}
			//1.2 若len(spec.unallocated) < 1, 表示无IP可分配, 直接返回error
			if len(ipPoolDetail.Spec.Unallocated) < 1 {
				errorInfo := fmt.Sprintf("assign ip: %s has no IP can be assigned, IPRecorderName: %s, assignIPStr: %s", ipPool.Name, ipRecorderName, podIPInfo.assignIPStr)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}

			//1.3 取assignIPStr所代表的索引位ipIndex, 检查IPPoolDetail中该索引位IP是否被占用
			cidr, _ = ipcidr.NewCIDR(ipPool.Spec.Cidr)
			ipNum, err := cidr.GetAvailableIPNum(podIPInfo.assignIPStr)
			if err != nil {
				logger.Error("assign ip: get assign IP index error", zap.Error(err))
				return nil, err
			}
			ipIndex = int(ipNum)
			inFunc := inslice.InIntSliceMapKeyFunc(ipPoolDetail.Spec.Unallocated)
			if !(ipPoolDetail.Spec.Allocations[ipIndex] == nil && inFunc(ipIndex)) {
				//assignIPStr已被占用, 直接返回error
				errorInfo := fmt.Sprintf("assign ip: specify IP %s in %s has been used", podIPInfo.assignIPStr, ipPool.Name)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}
		} else {
			//IPPools中随机IP分配
			//1.1 遍历ipPools, 获取可分配IP的IPPoolDetail.
			for _, v := range podIPInfo.ipPools {
				ipd, err := ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Get(v.Name, k8smetav1.GetOptions{})
				if err != nil {
					logger.Error("assign ip: get IPPoolDetail error, will retry", zap.String("IPPoolDetail", v.Name), zap.Int("retry", i+1), zap.Error(err))
					continue
				}
				if len(ipd.Spec.Unallocated) > 0 {
					//当IPPoolDetail可继续分配IP时跳出循环
					ipPoolDetail = ipd
					ipPool = &v
					break
				}
			}

			//1.2 若ipPoolDetail == nil, 表示无IP可分配, 直接返回error
			if ipPoolDetail == nil {
				errorInfo := fmt.Sprintf("assign ip: %v has no IP can be assigned, IPRecorderName: %s", podIPInfo.ipPools, ipRecorderName)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}

			//1.3 获取可分配IP(IPPoolDetail.spec.unallocated[0]).
			ipIndex = ipPoolDetail.Spec.Unallocated[0]
			cidr, _ = ipcidr.NewCIDR(ipPool.Spec.Cidr)
		}

		//2. 修改IPPoolDetail.
		_, err = a.updateIPPoolDetail(ipPoolDetail, true, ipRecorderName, ipIndex)
		if err != nil {
			//err != nil 表示之前获得的IPPoolDetail已过期(k8serrors.IsConflict()), 或其他错误. 需进行重试(跳转回2.1)
			if k8serrors.IsConflict(err) {
				logger.Warn("assign ip: update IPPoolDetail failed, will retry", zap.String("IPPoolDetail", ipPoolDetail.Name), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1), zap.Error(err))
			} else {
				logger.Error("assign ip: update IPPoolDetail error, will retry", zap.String("IPPoolDetail", ipPoolDetail.Name), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1), zap.Error(err))
			}
			continue
		} else {
			//3. 修改IPPoolDetail成功后, 再创建IPRecorder.(同样具有重试机制)
			ipStr, _ := cidr.GetAvailableNumIP(int64(ipIndex))
			for i := 0; i < retry; i++ {
				_, err := a.createIPRecorder(ipPool, podIPInfo, ipIndex, ipStr)
				if err != nil {
					// 已存在IPRecorder
					if k8serrors.IsAlreadyExists(err) {
						errorInfo := fmt.Sprintf("assign ip: failed to create IPRecorder %s is already exists, and already update IPPoolDetail: %s, ipStr=%s, ipIndex=%d", ipRecorderName, ipPool.Name, ipStr, ipIndex)
						logger.Error(errorInfo)
						return nil, errors.New(errorInfo)
					}
					logger.Error("assign ip: create IPRecorder failed, will retry", zap.String("IPRecorder", ipRecorderName), zap.String("IPPool", ipPool.Name), zap.Int("ipIndex", ipIndex), zap.String("ipStr", ipStr), zap.Int("retry", i+1), zap.Error(err))
					continue
				}
				logger.Info("assign ip: update IPPoolDetail and create IPRecorder success", zap.String("IPPoolDetail", ipPoolDetail.Name), zap.Int("ipIndex", ipIndex), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1))
				return &IPInfo{
					IP:      ipStr,
					Gateway: ipPool.Spec.Gateway,
					Vlan:    ipPool.Spec.Vlan,
				}, nil
			}
			errorInfo := fmt.Sprintf("assign ip: create IPRecorder: %s error and already update IPPoolDetail: %s, ipStr=%s, ipIndex=%d", ipRecorderName, ipPool.Name, ipStr, ipIndex)
			logger.Error(errorInfo)
			return nil, errors.New(errorInfo)
		}
	}
	//更新IPPoolDetail超过重试次数均失败
	errorInfo := fmt.Sprintf("assign ip: %s, try %d times to assign ip failed, update IPPoolDetail error", ipRecorderName, retry)
	logger.Error(errorInfo)
	return nil, errors.New(errorInfo)
}

// 固定IP分配时存在IPRecorder, 从IPRecorder分配IP.
func (a *Allocator) assignIPFromIPRecorder(ipRecorder *ipfixedv1alpha1.IPRecorder, podIPInfo *PodIPInfo) (*IPInfo, error) {
	var (
		ipfixedClient = a.ipfixedClient
		retry         = a.retry
	)

	// 存在IPRecorder, 则修改IPRecorder相应的IPLists[].released=false, 并返回IPRecorder中分配的IP信息.
	for i := 0; i < retry; i++ {
		if ipRecorder.IPLists == nil || len(ipRecorder.IPLists) == 0 {
			errorInfo := fmt.Sprintf("assign ip from IPRecorder: IPRecorder %s, but IPLists is nil or no value: %v", ipRecorder.Name, ipRecorder.IPLists)
			logger.Error(errorInfo)
			return nil, errors.New(errorInfo)
		}
		ipListsIndex := 0
		ipRecorderInfo := ipRecorder.IPLists[ipListsIndex]
		// 适配StatefulSet
		//for k, ipList := range ipRecorder.IPLists {
		//	if ipList.Name == podIPInfo.irName {
		//		ipListsIndex = k
		//		ipRecorderInfo = ipRecorder.IPLists[ipListsIndex]
		//		break
		//	}
		//}
		// 判断是否和已存在的IPRecorder记录冲突:
		//   冲突则先报错. TODO 报错原因: 暂未考虑指定IPPool修改或指定IP分配更改的情况
		//   不冲突则修改IPRecorder相应的IPLists[].released=false, 并返回IPRecorder中分配的IP信息.
		assignIPStr := podIPInfo.assignIPStr
		if assignIPStr != "" {
			//指定IP分配判断冲突
			if assignIPStr != ipRecorderInfo.IPAddress {
				errorInfo := fmt.Sprintf("assign ip from IPRecorder: the IPRecorder ip is not equal to the assigned ip, IPRecorderIP=%s, AssignIP=%s", ipRecorderInfo.IPAddress, assignIPStr)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}
		} else {
			//随机IP分配判断冲突
			isConflict := true
			assignIPPool := make([]string, len(podIPInfo.ipPools))
			for index, ipPool := range podIPInfo.ipPools {
				if ipRecorderInfo.Pool == ipPool.Name {
					isConflict = false
					break
				}
				assignIPPool[index] = ipPool.Name
			}
			if isConflict {
				//指定IPPool随机分配, 但指定的IPPool修改并排除已分配的IPPool的情况.
				errorInfo := fmt.Sprintf("assign ip from IPRecorder: the IPRecorder IPPool is not in assigned IPPool, IPPool=%s, AssignIPPool=%s", ipRecorderInfo.Pool, assignIPPool)
				logger.Error(errorInfo)
				return nil, errors.New(errorInfo)
			}
		}

		ipRecorderInfo.Released = false
		ipRecorder.IPLists[ipListsIndex] = ipRecorderInfo
		_, err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Update(ipRecorder)
		if err != nil {
			if k8serrors.IsConflict(err) {
				newIPRecorder, err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Get(ipRecorder.Name, k8smetav1.GetOptions{})
				if err != nil {
					continue
				}
				ipRecorder = newIPRecorder
				logger.Warn("assign ip from IPRecorder: update IPRecorder failed, will retry", zap.String("IPRecorder", ipRecorder.Name), zap.Int("retry", i+1), zap.Error(err))
			} else {
				logger.Error("assign ip from IPRecorder: update IPRecorder error, will retry", zap.String("IPRecorder", ipRecorder.Name), zap.Int("retry", i+1), zap.Error(err))
			}
			continue
		}
		return &IPInfo{
			IP:      ipRecorderInfo.IPAddress,
			Gateway: ipRecorderInfo.Gateway,
			Vlan:    ipRecorderInfo.Vlan,
		}, nil
	}
	//更新IPRecorder超过重试次数均失败
	errorInfo := fmt.Sprintf("assign ip from IPRecorder: try %d times to update IPRecorder %s failed", retry, ipRecorder.Name)
	logger.Error(errorInfo)
	return nil, errors.New(errorInfo)
}

// 释放固定IP(主要尝试更新IPLists[].released=true,cmdDel成功之前控制器可能会删除IPRecorder,因为可能固定IP的资源对象如VM已被删除)
func (a *Allocator) releaseFixedIP(ownerReference *k8smetav1.OwnerReference) error {
	var (
		kubevirtClient  = a.kubevirtClient
		ipfixedClient   = a.ipfixedClient
		k8sPodNamespace = string(a.k8sArgs.K8S_POD_NAMESPACE)
		k8sPodName      = string(a.k8sArgs.K8S_POD_NAME)
		ipRecorderName  string
	)
	if ownerReference == nil {
		errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: pod %s/%s is fixed IP, but parameter ownerreference is nil error", k8sPodName, k8sPodNamespace)
		logger.Error(errorInfo)
		return errors.New(errorInfo)
	}
	//由ownerReference获取固定IP的资源对象
	switch ownerReference.Kind {
	case KindVirtualMachine:
		//1. 由ownerReference获取vm
		vm, err := kubevirtClient.VirtualMachine(k8sPodNamespace).Get(ownerReference.Name, &k8smetav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				//若vm获取不到, 即k8serrors.IsNotFound(可能是ownerreferences已删除), 则返回nil, 记录日志, 交给控制器管理是否释放.
				logger.Warn("ip-fixed-ipam release ip: vmi ownerreference vm not found, will be returned and release is managed by the controller",
					zap.String("vmName", vm.Name), zap.String("vmNamespace", k8sPodNamespace))
				return nil
			}
			errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: get vmi ownerreference vm %s/%s error: %s", ownerReference.Name, k8sPodNamespace, err)
			logger.Error(errorInfo)
			return errors.New(errorInfo)
		}

		//2. 根据VM, 获取IPRecorder.
		ipRecorderName = strings.Join([]string{IPRecorderNamePrefix, ResourcesVirtualMachine, vm.Namespace, vm.Name}, IPRecorderNameSeparator)
		ipRecorder, err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Get(ipRecorderName, k8smetav1.GetOptions{})
		if err != nil {
			errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: get IPRecorder %s error: %s", ipRecorderName, err)
			logger.Error(errorInfo)
			return errors.New(errorInfo)
		}

		//3. 尝试更新IPRecorder的IPLists[].released=true(仅尝试一次, 失败则记录日志返回nil, 交给控制器管理是否释放)
		_, err = a.updateIPRecorderToReleasedIP(ipRecorder, vm.Name)
		if err != nil {
			// 失败则记录日志返回nil, 交给控制器管理释放.
			//if k8serrors.IsConflict(err) {
			//	logger.Warn("ip-fixed-ipam release ip: update IPRecorder to released IP, but the provided update conflicts.",
			//		zap.String("IPRecorder", ipRecorderName), zap.String("podName", k8sPodName), zap.String("podNamespaces", k8sPodNamespace))
			//}
			logger.Warn("ip-fixed-ipam release ip: update IPRecorder to released IP error, will be returned and release is managed by the controller",
				zap.String("IPRecorder", ipRecorderName), zap.String("podName", k8sPodName), zap.String("podNamespaces", k8sPodNamespace))
		} else {
			logger.Info("ip-fixed-ipam release ip: update IPRecorder to released IP success",
				zap.String("IPRecorder", ipRecorderName), zap.String("podName", k8sPodName), zap.String("podNamespaces", k8sPodNamespace))
		}
		return nil
	case KindStatefulSet:
		//TODO 暂不考虑StatefulSet固定IP, 应同default.
	}
	return nil
}

// 非固定IP释放: 尝试修改IPPoolDetail, 删除IPRecorder, 若超过重试次数均失败, 尝试更新IPRecorder的IPLists[].released=true后返回nil, 交给控制器释放.
func (a *Allocator) releaseUnFixedIP() error {
	var (
		ipfixedClient     = a.ipfixedClient
		k8sPodContainerID = string(a.k8sArgs.K8S_POD_INFRA_CONTAINER_ID)
		retry             = a.retry
	)

	//1. 获取IPRecorder
	ipRecorderName := strings.Join([]string{IPRecorderNamePrefix, k8sPodContainerID}, IPRecorderNameSeparator)
	ipRecorder, err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Get(ipRecorderName, k8smetav1.GetOptions{})
	if err != nil {
		errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: get IPRecorder %s error: %s", ipRecorderName, err)
		logger.Error(errorInfo)
		return errors.New(errorInfo)
	} else {
		if ipRecorder.IPLists == nil || len(ipRecorder.IPLists) == 0 {
			errorInfo := fmt.Sprintf("ip-fixed-ipam release ip: get IPRecorder %s success, but IPLists is nil or no value: %v", ipRecorder.Name, ipRecorder.IPLists)
			logger.Error(errorInfo)
			return errors.New(errorInfo)
		}
	}

	ipPoolDetailName := ipRecorder.IPLists[0].Pool
	//2. 尝试释放IP(修改IPPoolDetail, 删除IPRecorder, 失败则重试)
	for i := 0; i < retry; i++ {
		var (
			ipPoolDetail *ipfixedv1alpha1.IPPoolDetail
		)
		//2.1 根据IPRecorder的IPLists[0].pool, 获取IPPoolDetail
		ipPoolDetail, err := ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Get(ipPoolDetailName, k8smetav1.GetOptions{})
		if err != nil {
			logger.Error("ip-fixed-ipam release ip: release unfixed IP, get IPPoolDetail error, will retry", zap.String("IPPoolDetail", ipPoolDetailName), zap.Int("retry", i+1), zap.Error(err))
			continue
		}

		//2.2 修改IPPoolDetail, 释放IP
		_, err = a.updateIPPoolDetail(ipPoolDetail, false, ipRecorderName, ipRecorder.IPLists[0].Index)
		if err != nil {
			//err != nil 表示之前获得的IPPoolDetail已过期(k8serrors.IsConflict()), 或其他错误. 需进行重试(跳转回1)
			if k8serrors.IsConflict(err) {
				logger.Warn("ip-fixed-ipam release ip: update IPPoolDetail failed, will retry", zap.String("IPPoolDetail", ipPoolDetailName), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1), zap.Error(err))
			} else {
				logger.Error("ip-fixed-ipam release ip: update IPPoolDetail error, will retry", zap.String("IPPoolDetail", ipPoolDetailName), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1), zap.Error(err))
			}
			continue
		} else {
			//修改IPPoolDetail成功后, 再删除IPRecorder.(同样具有重试机制)
			for i := 0; i < retry; i++ {
				err := ipfixedClient.IpfixedV1alpha1().IPRecorders().Delete(ipRecorderName, &k8smetav1.DeleteOptions{})
				if err != nil {
					logger.Error("ip-fixed-ipam release ip: delete IPRecorder failed, will retry", zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1), zap.Error(err))
					continue
				}
				logger.Info("ip-fixed-ipam release ip: update IPPoolDetail and delete IPRecorder success", zap.String("IPPoolDetail", ipPoolDetailName), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", i+1))
				return nil
			}
			return fmt.Errorf("ip-fixed-ipam release ip: delete IPRecorder: %s error and already update IPPoolDetail: %s", ipRecorderName, ipPoolDetailName)
		}
	}
	//如果更新IPPoolDetail超过重试次数均失败, 交给控制器释放.
	logger.Warn("ip-fixed-ipam release ip: try to update IPPoolDetail failed, will be returned and released IP by the controller", zap.String("IPPoolDetail", ipPoolDetailName), zap.String("IPRecorder", ipRecorderName), zap.Int("retry", retry))
	return nil
}

// 修改IPPoolDetail(allocations []*int & unallocated []int & recorders []string)
// isAssign=true表示IP分配, isAssign=false表示IP释放.
func (a *Allocator) updateIPPoolDetail(ipPoolDetail *ipfixedv1alpha1.IPPoolDetail, isAssign bool, ipRecorderName string, ipIndex int) (*ipfixedv1alpha1.IPPoolDetail, error) {
	var (
		ipfixedClient = a.ipfixedClient
	)
	if isAssign {
		//IP分配
		//1. 去除spec.unallocated中值为ipIndex的值.
		unallocated := ipPoolDetail.Spec.Unallocated
		for k, v := range unallocated {
			if v == ipIndex {
				unallocated = append(unallocated[:k], unallocated[k+1:]...)
				ipPoolDetail.Spec.Unallocated = unallocated
				break
			}
		}
		//2. 修改spec.allocations[ipIndex] = &ipIndex.
		ipPoolDetail.Spec.Allocations[ipIndex] = &ipIndex
		//3. 添加spec.recorders.
		recorders := ipPoolDetail.Spec.Recorders
		inFunc := inslice.InStringSliceMapKeyFunc(recorders)
		if !inFunc(ipRecorderName) {
			recorders = append(recorders, ipRecorderName)
			ipPoolDetail.Spec.Recorders = recorders
		}
	} else {
		//IP释放
		//1. 若spec.unallocated中不存在ipIndex的值则添加
		unallocated := ipPoolDetail.Spec.Unallocated
		inFunc := inslice.InIntSliceMapKeyFunc(unallocated)
		if !inFunc(ipIndex) {
			unallocated = append(unallocated, ipIndex)
			ipPoolDetail.Spec.Unallocated = unallocated
		}
		//2. 修改spec.allocations[ipIndex] = nil.
		ipPoolDetail.Spec.Allocations[ipIndex] = nil
		//3. 去除spec.recorders.
		recorders := ipPoolDetail.Spec.Recorders
		for k, v := range recorders {
			if v == ipRecorderName {
				recorders = append(recorders[:k], recorders[k+1:]...)
				ipPoolDetail.Spec.Recorders = recorders
				break
			}
		}
	}

	//4. 更新IPPoolDetail
	return ipfixedClient.IpfixedV1alpha1().IPPoolDetails().Update(ipPoolDetail)
}

// 创建IPRecorder
func (a *Allocator) createIPRecorder(ipPool *ipfixedv1alpha1.IPPool, podIPInfo *PodIPInfo, ipIndex int, ipStr string) (*ipfixedv1alpha1.IPRecorder, error) {
	var (
		ipfixedClient = a.ipfixedClient
	)
	// 构造IPLists.
	ipLists := []ipfixedv1alpha1.IPRecorderIPLists{
		{
			Pool:      ipPool.Name,
			IPAddress: ipStr,
			Gateway:   ipPool.Spec.Gateway,
			Index:     ipIndex,
			Resources: podIPInfo.irResources,
			Namespace: podIPInfo.irNamespace,
			Name:      podIPInfo.irName,
			Vlan:      ipPool.Spec.Vlan,
			Released:  false,
		},
	}

	ipRecorder := &ipfixedv1alpha1.IPRecorder{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: podIPInfo.ipRecorderName,
			Labels: map[string]string{
				ipPool.Name: IPRecorderLabelIPPoolValue, //hn-idm-cidr: ippool
			},
		},
		IPLists: ipLists,
	}

	return ipfixedClient.IpfixedV1alpha1().IPRecorders().Create(ipRecorder)
}

// 修改IPRecorder指定的IPLists[].released=true
func (a *Allocator) updateIPRecorderToReleasedIP(ipRecorder *ipfixedv1alpha1.IPRecorder, ipListsName string) (*ipfixedv1alpha1.IPRecorder, error) {
	var (
		ipfixedClient = a.ipfixedClient
	)
	for index, ipList := range ipRecorder.IPLists {
		if ipList.Name == ipListsName {
			ipList.Released = true
			ipRecorder.IPLists[index] = ipList
			break
		}
	}

	return ipfixedClient.IpfixedV1alpha1().IPRecorders().Update(ipRecorder)
}
