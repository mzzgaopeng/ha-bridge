package ippool

import (
	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/v1alpha1"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/inslice"
	"cmos.chinamobile.com/ip-fixed/pkg/utils/ipcidr"
	"context"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPPoolHandler struct {
	k8sClient client.Client
	context   context.Context
	log       logr.Logger
}

func NewIPPoolHandler(client client.Client, context context.Context, log logr.Logger) *IPPoolHandler {
	return &IPPoolHandler{
		k8sClient: client,
		context:   context,
		log:       log,
	}
}

// sync IPPool and IPPoolDetail
func (handler *IPPoolHandler) syncIPPool(ipPool *ipfixedv1alpha1.IPPool) (ctrl.Result, error) {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
		log       = handler.log
	)

	ipPoolDetail := new(ipfixedv1alpha1.IPPoolDetail)
	key := types.NamespacedName{
		Namespace: ipPool.Namespace,
		Name:      ipPool.Name,
	}
	//Get IPPoolDetail corresponding to IPPool. If not found, create IPPoolDetail based on IPPool, else update IPPoolDetail based on IPPool.
	err := k8sClient.Get(context, key, ipPoolDetail)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			//# 1. IPPoolDetail not found, create IPPoolDetail based on IPPool.spec.
			//## 1.1 create IPPoolDetail.
			ipPoolDetail, err = handler.createIPPoolDetail(ipPool)
			if err != nil {
				log.Error(err, "create IPPoolDetail error - requeue")
				return ctrl.Result{}, err
			}
			log.Info("create IPPoolDetail success")
			//## 1.2 update IPPool.status.
			err = handler.updateIPPoolStatus(ipPool, ipPoolDetail)
			if err != nil {
				log.Error(err, "update IPPool.status error - requeue")
				return ctrl.Result{}, err
			}
			log.Info("------ Success sync IPPool, create IPPoolDetail and update IPPool.status ------")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "reading the IPPoolDetail error - requeue")
			return ctrl.Result{}, err
		}
	}

	//# 2. IPPoolDetail can be found, update IPPoolDetail based on IPPool.spec.
	//## 2.1 TODO update IPPoolDetail.
	//ipPoolDetail, err = handler.updateIPPoolDetail(ipPool, ipPoolDetail)
	//if err != nil {
	//	log.Error(err, "update IPPoolDetail error - requeue")
	//	return ctrl.Result{}, err
	//}
	//log.Info("update IPPoolDetail success")
	//## 2.2 update IPPool.status.
	err = handler.updateIPPoolStatus(ipPool, ipPoolDetail)
	if err != nil {
		log.Error(err, "update IPPool.status error - requeue")
		return ctrl.Result{}, err
	}
	log.Info("------ Success sync IPPool, update IPPool.status ------")
	return ctrl.Result{}, nil
}

func (handler *IPPoolHandler) createIPPoolDetail(ipPool *ipfixedv1alpha1.IPPool) (*ipfixedv1alpha1.IPPoolDetail, error) {
	var (
		log        = handler.log
		k8sClient  = handler.k8sClient
		context    = handler.context
		cidrStr    = ipPool.Spec.Cidr
		excludeIPs = make([]string, len(ipPool.Spec.ExcludeIPs))
	)

	cidr, err := ipcidr.NewCIDR(cidrStr)
	if err != nil {
		log.Error(err, "parse CIDR error", "CIDR", cidrStr)
		return nil, err
	}

	copy(excludeIPs, ipPool.Spec.ExcludeIPs)
	unallocated := []int{}
	allocations := make([]*int, cidr.GetAvailableIPCount())
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

	ipPoolDetail := initializeIPPoolDetail(ipPool, allocations, unallocated)
	err = k8sClient.Create(context, ipPoolDetail)
	if err != nil {
		return nil, err
	}
	return ipPoolDetail, nil
}

func initializeIPPoolDetail(ipPool *ipfixedv1alpha1.IPPool, allocations []*int, unallocated []int) *ipfixedv1alpha1.IPPoolDetail {
	return &ipfixedv1alpha1.IPPoolDetail{
		ObjectMeta: metav1.ObjectMeta{
			Name: ipPool.Name,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         ipPool.APIVersion,
					Kind:               ipPool.Kind,
					Name:               ipPool.Name,
					UID:                ipPool.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{false}[0],
				},
			},
		},
		Spec: ipfixedv1alpha1.IPPoolDetailSpec{
			Cidr:        ipPool.Spec.Cidr,
			Vlan:        ipPool.Spec.Vlan,
			Allocations: allocations,
			Unallocated: unallocated,
			Recorders:   []string{},
		},
	}
}

func (handler *IPPoolHandler) updateIPPoolStatus(ipPool *ipfixedv1alpha1.IPPool, ipPoolDetail *ipfixedv1alpha1.IPPoolDetail) error {
	var (
		k8sClient = handler.k8sClient
		context   = handler.context
		// The IP address index of the IPPool. null indicates that the ip corresponding to the index is not occupied. If it is not null, it means that the ip has been used, such as 0 means 192.168.2.1 has been used.
		allocations = ipPoolDetail.Spec.Allocations
		// Indicates the unallocated IP index.
		unallocated = ipPoolDetail.Spec.Unallocated

		// The number of exclude IPs in the IP pool 表示该IP池内不可用的IP数量
		excludeIPCount = len(ipPool.Spec.ExcludeIPs)
		// The number of IPs available in the IPPool 表示该IP段内当前可用IP数量
		available = len(unallocated)
		// The number of IPs used in the IPPool 表示该IP段内已经使用的IP数量(全部可用 + 2 - 可分配的 - 不可用)
		// excludeIPCount中应不包含cidr表示的网络地址段中的网络号与广播地址, 即第一个地址与最后一个地址.
		using = len(allocations) + 2 - available - excludeIPCount
	)

	ipPoolCopy := ipPool.DeepCopy()
	ipPoolCopy.Status.Available = available
	ipPoolCopy.Status.ExcludeIPCount = excludeIPCount
	ipPoolCopy.Status.Using = using
	err := k8sClient.Status().Update(context, ipPoolCopy)
	if err != nil {
		return err
	}
	return nil
}

//TODO IPPool创建出来之后暂不允许修改
// 允许修改需要考虑: 1.vlan的修改同步IPPoolDetail/IPRecorder 2.excludeIPs的修改, 可能新增的禁止使用IP已被使用.
func (handler *IPPoolHandler) updateIPPoolDetail(ipPool *ipfixedv1alpha1.IPPool, ipPoolDetail *ipfixedv1alpha1.IPPoolDetail) (*ipfixedv1alpha1.IPPoolDetail, error) {
	var (
		k8sClient  = handler.k8sClient
		context    = handler.context
		excludeIPs = make([]string, len(ipPool.Spec.ExcludeIPs))
	)
	copy(excludeIPs, ipPool.Spec.ExcludeIPs)
	ipPoolDetailCopy := ipPoolDetail.DeepCopy()

	//1. sync spec.vlan
	//ipPoolDetailCopy.Spec.Vlan = ipPool.Spec.Vlan
	//TODO 待完善: 协调IPPoolDetail的allocations与unallocated.完善思路: 遍历IPPoolDetail.recorders.
	//2. sync spec.allocations and spec.unallocated

	//3. update IPPoolDetail
	err := k8sClient.Update(context, ipPoolDetailCopy)
	if err != nil {
		return nil, err
	}
	return ipPoolDetailCopy, nil
}
