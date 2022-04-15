package iprecorder

import (
	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/v1alpha1"
	ipfixedipam "cmos.chinamobile.com/ip-fixed/pkg/ip-fixed-ipam"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/inslice"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const defaultRetry int = 10

type IPRecorderHandler struct {
	k8sClient client.Client
	context   context.Context
	log       logr.Logger
	// return successResult, cycle sync IPRecorder.
	successResult ctrl.Result
}

func NewIPRecorderHandler(client client.Client, context context.Context, log logr.Logger, successResult ctrl.Result) *IPRecorderHandler {
	return &IPRecorderHandler{
		k8sClient:     client,
		context:       context,
		log:           log,
		successResult: successResult,
	}
}

// check IPRecorder to release IP, sync IPRecorder and IPPoolDetail.
//TODO 适配Statefulset
func (handler *IPRecorderHandler) syncIPRecorder(ipRecorder *ipfixedv1alpha1.IPRecorder) (ctrl.Result, error) {
	var (
		log           = handler.log
		successResult = handler.successResult
	)
	if ipRecorder.IPLists == nil || len(ipRecorder.IPLists) == 0 {
		return ctrl.Result{}, fmt.Errorf("sync IPRecorder %s error, IPLists is nil or no value: %v", ipRecorder.Name, ipRecorder.IPLists)
	}
	//# 1. 根据IPRecorder的名称格式判断IP是否固定, 并获取使用IP的资源信息(resources/namespace/name)
	resourcesInfo := handler.getUseIPResourcesInfo(ipRecorder)

	//# 2. 检查使用IP的资源是否存在
	isExists, err := handler.checkResourcesExists(resourcesInfo)
	if err != nil {
		log.Error(err, "check resources exists error - requeue")
		return ctrl.Result{}, err
	}

	//## 2.1 若使用IP的资源存在, 本次sync结束
	if isExists {
		log.Info("resources using ip still exist, wait for next sync.", "resources", resourcesInfo.resources+"/"+resourcesInfo.namespace+"/"+resourcesInfo.name)
		goto success
	}
	//## 2.2 若使用IP的资源不存在, 根据是否固定IP做下一步处理(IP释放)
	if resourcesInfo.isFixedIP {
		//### 2.2.1 使用IP的资源不存在, 且为固定IP
		switch resourcesInfo.resources {
		// VM虚拟机固定IP
		case ipfixedipam.ResourcesVirtualMachine:
			//判断IP是否可释放, 即IPLists[].released是否为true, 决定是否在本次周期内释放
			isIPReleasable := isIPReleasable(ipRecorder)
			if !isIPReleasable {
				//若IP不可释放, 修改为IPLists[].released=true, 本次sync结束, 等待下次sync清理。
				_, err := handler.updateIPRecorderToReleasedIP(ipRecorder)
				if err != nil {
					log.Error(err, "update IPRecorder to released IP error")
					return ctrl.Result{}, err
				}
				log.Info("update IPRecorder to released IP success, wait for next sync.")
				goto success
			}
			//若IP可释放, 修改IPPoolDetail, 修改成功后再删除IPRecorder
			ipList := ipRecorder.IPLists[0]
			_, err := handler.updateIPPoolDetailToReleasedIP(resourcesInfo, ipList.Pool, ipRecorder.Name, ipList.Index)
			if err != nil {
				log.Error(err, "update IPPoolDetail to released fixed IP error - requeue", "IPPoolDetail", ipList.Pool)
				return ctrl.Result{}, err
			}

			_, err = handler.deleteIPRecorder(ipRecorder)
			if err != nil {
				log.Error(err, "delete IPRecorder error - requeue")
				return ctrl.Result{}, err
			}
			goto success
			//case ipfixedipam.ResourcesStatefulSet:
			//	fallthrough
			//default:
			//	return ctrl.Result{}, fmt.Errorf("sync IPRecorder error, unsupported use of ip resources: %s", resourcesInfo.resources)
		}
	} else {
		//### 2.2.2 使用IP的资源不存在, 且为非固定IP(非固定IP时IPAM已有释放逻辑, 此处当IPAM释放失败时才会执行)
		ipList := ipRecorder.IPLists[0]
		_, err := handler.updateIPPoolDetailToReleasedIP(resourcesInfo, ipList.Pool, ipRecorder.Name, ipList.Index)
		if err != nil {
			log.Error(err, "update IPPoolDetail to released unfixed IP error - requeue", "IPPoolDetail", ipList.Pool)
			return ctrl.Result{}, err
		}

		_, err = handler.deleteIPRecorder(ipRecorder)
		if err != nil {
			log.Error(err, "delete IPRecorder error - requeue")
			return ctrl.Result{}, err
		}
		goto success
	}
success:
	log.Info("------ Success sync IPRecorder ------")
	return successResult, nil
}

type ResourcesInfo struct {
	isFixedIP bool
	resources string
	namespace string
	name      string
}

// Determine whether the IP is fixed according to the name format of the IPRecorder, and obtain the resources info that use the IP.
func (handler *IPRecorderHandler) getUseIPResourcesInfo(ipRecorder *ipfixedv1alpha1.IPRecorder) *ResourcesInfo {
	irNameStrs := strings.Split(ipRecorder.Name, ipfixedipam.IPRecorderNameSeparator)
	//IPRecorder命名格式:
	// 非固定IP: k8s-pod-network.{containerid}
	// 固定IP: k8s-pod-network.{resources}.{namespaces}.{name}
	// vm固定IP时, IPLists[].resources等数据对应virtualmachines. sts固定IP时, 则对应为pods.
	if len(irNameStrs) >= 4 {
		return &ResourcesInfo{
			isFixedIP: true,
			resources: irNameStrs[1],
			namespace: irNameStrs[2],
			name:      irNameStrs[3],
		}
	} else {
		return &ResourcesInfo{
			isFixedIP: false,
			resources: ipRecorder.IPLists[0].Resources,
			namespace: ipRecorder.IPLists[0].Namespace,
			name:      ipRecorder.IPLists[0].Name,
		}
	}
}

// Check if the resource exists
func (handler *IPRecorderHandler) checkResourcesExists(info *ResourcesInfo) (bool, error) {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
		key       = types.NamespacedName{
			Namespace: info.namespace,
			Name:      info.name,
		}
		isExists = false
		err      error
	)
	switch info.resources {
	case ipfixedipam.ResourcesPod:
		err = k8sClient.Get(context, key, new(v1.Pod))
	case ipfixedipam.ResourcesVirtualMachine:
		err = k8sClient.Get(context, key, new(kubevirtv1.VirtualMachine))
	case ipfixedipam.ResourcesStatefulSet:
		fallthrough
	default:
		return isExists, fmt.Errorf("sync IPRecorder error, unsupported use of ip resources: %s", info.resources)
	}
	if err != nil {
		if k8serrors.IsNotFound(err) {
			isExists = false
			return isExists, nil
		}
		return isExists, fmt.Errorf("sync IPRecorder error, get using IP resource error: %s", err.Error())
	}
	isExists = true
	return isExists, nil
}

// 判断IP是否可释放
func isIPReleasable(ipRecorder *ipfixedv1alpha1.IPRecorder) bool {
	isReleasable := true
	for _, v := range ipRecorder.IPLists {
		if v.Released == false {
			isReleasable = false
			break
		}
	}
	return isReleasable
}

// 固定IP时占用IP的resources已不存在, 修改IPRecorder的IPLists[].released=true, 等待下次sync清理.
func (handler *IPRecorderHandler) updateIPRecorderToReleasedIP(ipRecorder *ipfixedv1alpha1.IPRecorder) (*ipfixedv1alpha1.IPRecorder, error) {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
	)
	ipRecorderCopy := ipRecorder.DeepCopy()
	for index, ipList := range ipRecorderCopy.IPLists {
		ipList.Released = true
		ipRecorderCopy.IPLists[index] = ipList
	}
	err := k8sClient.Update(context, ipRecorderCopy)
	if err != nil {
		return nil, err
	}
	return ipRecorderCopy, nil
}

func (handler *IPRecorderHandler) updateIPPoolDetailToReleasedIP(info *ResourcesInfo, ipPoolDetailName string, ipRecorderName string, ipIndex int) (*ipfixedv1alpha1.IPPoolDetail, error) {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
		log       = handler.log
		key       = types.NamespacedName{
			Namespace: "",
			Name:      ipPoolDetailName,
		}
		retry = defaultRetry
	)

	for i := 0; i < retry; i++ {
		//更新IPPoolDetail释放IP前再次检查占用IP资源是否存在, 尽量避免立即创建同名资源的情况(中间间隔一次sync周期).
		isExists, err := handler.checkResourcesExists(info)
		if err != nil {
			log.Info("update IPPoolDetail to released IP, check resources exists error, will retry", "IPPoolDetail", ipPoolDetailName, "retry", i+1, "err", err.Error())
			continue
		}
		if isExists {
			return nil, fmt.Errorf("stop update IPPoolDetail %s to released IP, use ip resources %s is exists, retry: %d", ipPoolDetailName, info.resources+"/"+info.namespace+"/"+info.name, i+1)
		}
		ipPoolDetail := new(ipfixedv1alpha1.IPPoolDetail)
		err = k8sClient.Get(context, key, ipPoolDetail)
		if err != nil {
			log.Info("update IPPoolDetail to released IP, but get IPPoolDetail error, will retry", "IPPoolDetail", ipPoolDetailName, "retry", i+1, "err", err.Error())
			continue
		}
		ipPoolDetailCopy := ipPoolDetail.DeepCopy()
		//1. 若spec.unallocated中不存在ipIndex的值则添加
		unallocated := ipPoolDetailCopy.Spec.Unallocated
		inFunc := inslice.InIntSliceMapKeyFunc(unallocated)
		if !inFunc(ipIndex) {
			unallocated = append(unallocated, ipIndex)
			ipPoolDetailCopy.Spec.Unallocated = unallocated
		}
		//2. 修改spec.allocations[ipIndex] = nil.
		ipPoolDetailCopy.Spec.Allocations[ipIndex] = nil
		//3. 去除spec.recorders.
		recorders := ipPoolDetailCopy.Spec.Recorders
		isRecorderExist := false
		for k, v := range recorders {
			if v == ipRecorderName {
				recorders = append(recorders[:k], recorders[k+1:]...)
				ipPoolDetailCopy.Spec.Recorders = recorders
				isRecorderExist = true
				break
			}
		}
		if !isRecorderExist {
			//若IPRecorder不存在于spec.recorders中, 表明已修改IPPoolDetail, 但还未删除IPRecorder, 删除IPRecorder即可, 不再修改IPPoolDetail(避免还未删除IPRecorder, 已释放的IP被重新分配给其他容器时再次被释放).
			log.Info("IPRecorder not in IPPoolDetail.spec.recorders, IPRecorder will be deleted and IPPoolDetail will not be modified", "IPPoolDetail", ipPoolDetailName, "IPIndex", ipIndex, "retry", i+1)
			return ipPoolDetailCopy, nil
		}

		err = k8sClient.Update(context, ipPoolDetailCopy)
		if err != nil {
			//err != nil 表示之前获得的IPPoolDetail已过期(k8serrors.IsConflict()), 或其他错误需进行重试
			log.Info("update IPPoolDetail to released IP failed, will retry", "IPPoolDetail", ipPoolDetailName, "retry", i+1, "err", err.Error())
			continue
		}
		log.Info("update IPPoolDetail to released IP success", "IPPoolDetail", ipPoolDetailName, "IPIndex", ipIndex, "retry", i+1)
		return ipPoolDetailCopy, nil
	}
	//如果更新IPPoolDetail超过重试次数均失败, 等待下次sync重试.
	return nil, fmt.Errorf("try %d times update IPPoolDetail %s to released IP failed, wait for next sync", retry, ipPoolDetailName)
}

func (handler *IPRecorderHandler) deleteIPRecorder(ipRecorder *ipfixedv1alpha1.IPRecorder) (*ipfixedv1alpha1.IPRecorder, error) {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
		log       = handler.log
		retry     = defaultRetry
	)

	for i := 0; i < retry; i++ {
		err := k8sClient.Delete(context, ipRecorder)
		if err != nil {
			log.Info("delete IPRecorder failed, will retry", "retry", i+1, "err", err.Error())
			continue
		}
		log.Info("delete IPRecorder success", "retry", i+1)
		return ipRecorder, err
	}
	return nil, fmt.Errorf("try %d times delete IPRecorder %s failed, wait for next sync", retry, ipRecorder.Name)
}
