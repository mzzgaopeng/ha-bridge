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
// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	ipfixedv1alpha1 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	versioned "cmos.chinamobile.com/ip-fixed/generated/ipfixed/clientset/versioned"
	internalinterfaces "cmos.chinamobile.com/ip-fixed/generated/ipfixed/informers/externalversions/internalinterfaces"
	v1alpha1 "cmos.chinamobile.com/ip-fixed/generated/ipfixed/listers/ipfixed/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// IPRecorderInformer provides access to a shared informer and lister for
// IPRecorders.
type IPRecorderInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.IPRecorderLister
}

type iPRecorderInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewIPRecorderInformer constructs a new informer for IPRecorder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewIPRecorderInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredIPRecorderInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredIPRecorderInformer constructs a new informer for IPRecorder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredIPRecorderInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IpfixedV1alpha1().IPRecorders().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IpfixedV1alpha1().IPRecorders().Watch(options)
			},
		},
		&ipfixedv1alpha1.IPRecorder{},
		resyncPeriod,
		indexers,
	)
}

func (f *iPRecorderInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredIPRecorderInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *iPRecorderInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&ipfixedv1alpha1.IPRecorder{}, f.defaultInformer)
}

func (f *iPRecorderInformer) Lister() v1alpha1.IPRecorderLister {
	return v1alpha1.NewIPRecorderLister(f.Informer().GetIndexer())
}