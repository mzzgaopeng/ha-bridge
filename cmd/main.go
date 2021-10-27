/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	v2 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	ipfixedclientset "cmos.chinamobile.com/ip-fixed/generated/ipfixed/clientset/versioned"
	ipaminformers "cmos.chinamobile.com/ip-fixed/generated/ipfixed/informers/externalversions"
	"flag"
	"fmt"
	"ha-bridge/pkg/bond"
	"ha-bridge/pkg/failover"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	kubev1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"math/rand"
	"os"
	"os/signal"
	"time"
)

func main() {
	klog.Infoln("start habridge......")
	klog.InitFlags(nil)
	flag.Parse()
	failover.HOST_NAME = os.Getenv("HOST_NAME")
	klog.Infoln("get nodename ", failover.HOST_NAME)
	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()
	virtClientSet, err := kubecli.GetKubevirtClient()
	if err != nil {
		klog.Fatalf("cannot obtain KubeVirt client: %v\n", err)
	}
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		klog.Fatalf("cannot obtain kube client: %v\n", err)
	}
	ipfixedClient, err := ipfixedclientset.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("cannot obtain Ipam client: %v\n", err)
	}
	//ipfixedClient.IpfixedV1alpha1()

	klog.Infoln("create informer......")
	lw := cache.NewListWatchFromClient(virtClientSet.RestClient(), "virtualmachineinstances", k8sv1.NamespaceAll, fields.Everything())
	kubvirtInformer := cache.NewSharedIndexInformer(lw, &kubev1.VirtualMachineInstance{}, resyncPeriod(12*time.Hour), cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		"node": func(obj interface{}) (strings []string, e error) {
			return []string{obj.(*kubev1.VirtualMachineInstance).Status.NodeName}, nil
		},
	})
	failover.VirtInformer = kubvirtInformer
	ipfixedInformerFactory := ipaminformers.NewSharedInformerFactory(ipfixedClient, time.Second*30)
	ipamInformer := ipfixedInformerFactory.Ipfixed().V1alpha1().IPRecorders().Informer()
	ipamInformer.AddIndexers(cache.Indexers{"ipaddress": func(obj interface{}) (strings []string, e error) {
		return []string{obj.(*v2.IPRecorder).IPLists[0].IPAddress}, nil
	},
	})
	klog.Infoln("start  informer......")
	go kubvirtInformer.Run(stopCh)
	go ipamInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, ipamInformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for ipam caches to sync"))
		return
	}
	failover.IpamInformer = ipamInformer
	klog.Infoln("start netlink listener ......")
	bond.Start()

}

// resyncPeriod computes the time interval a shared informer waits before resyncing with the api server
func resyncPeriod(minResyncPeriod time.Duration) time.Duration {
	// #nosec no need for better randomness
	factor := rand.Float64() + 1
	return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
}

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
