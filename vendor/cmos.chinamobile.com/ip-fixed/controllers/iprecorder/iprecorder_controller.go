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

package iprecorder

import (
	"context"
	"flag"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/v1alpha1"
)

func init() {
	flag.Int64Var(&checkIPRecorderInterval, "check-iprecorder-interval", checkIPRecorderInterval, "Check the interval of IPRecorder, time unit second. reconcile.Result.RequeueAfter = checkIPRecorderInterval.")
}

var (
	checkIPRecorderInterval = int64(60)
)

// IPRecorderReconciler reconciles a IPRecorder object
type IPRecorderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=iprecorders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=iprecorders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippools,verbs=get;list;watch;
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippooldetails,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipfixed.cmos.chinamobile.com,resources=ippooldetails/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get

func (r *IPRecorderReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var log = r.Log.WithValues("iprecorderControl", req.NamespacedName)

	// Fetch the IPRecorder instance
	ipRecorder := new(ipfixedv1alpha1.IPRecorder)
	err := r.Client.Get(context.TODO(), req.NamespacedName, ipRecorder)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		log.Error(err, "reading the IPRecorder error - requeue", "key", req.NamespacedName)
		return ctrl.Result{}, err
	}

	var successResult = ctrl.Result{RequeueAfter: time.Duration(checkIPRecorderInterval) * time.Second, Requeue: true}
	var handler = NewIPRecorderHandler(r.Client, context.Background(), log, successResult)
	log.Info("------ Begin to sync IPRecorder ------")
	return handler.syncIPRecorder(ipRecorder)
}

func (r *IPRecorderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipfixedv1alpha1.IPRecorder{}).
		Complete(r)
}
