/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ippool

import (
	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/v1alpha1"
	"context"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPPoolReconciler reconciles a IPPool object
type IPPoolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippooldetails,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippooldetails/status,verbs=get;update;patch

//Reconcile reads IPPool object,
//when IPPoolDetails with the same name as IPPool does not exist, create according to IPPool.spec,
//and generate IPPool.status according to IPPoolDetails.spec.
func (r *IPPoolReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var log = r.Log.WithValues("ippoolControl", req.NamespacedName)

	// Fetch the IPPool instance
	ipPool := new(ipfixedv1alpha1.IPPool)
	err := r.Client.Get(context.TODO(), req.NamespacedName, ipPool)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		log.Error(err, "reading the IPPool error - requeue", "key", req.NamespacedName)
		return ctrl.Result{}, err
	}

	var handler = NewIPPoolHandler(r.Client, context.Background(), log)
	log.Info("------ Begin to sync IPPool ------")
	return handler.syncIPPool(ipPool)
}

func (r *IPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipfixedv1alpha1.IPPool{}).
		Owns(&ipfixedv1alpha1.IPPoolDetail{}).
		Complete(r)
}
