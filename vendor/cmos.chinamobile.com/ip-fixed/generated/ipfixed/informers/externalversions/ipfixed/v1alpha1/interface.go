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
	internalinterfaces "cmos.chinamobile.com/ip-fixed/generated/ipfixed/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// IPPools returns a IPPoolInformer.
	IPPools() IPPoolInformer
	// IPPoolDetails returns a IPPoolDetailInformer.
	IPPoolDetails() IPPoolDetailInformer
	// IPRecorders returns a IPRecorderInformer.
	IPRecorders() IPRecorderInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// IPPools returns a IPPoolInformer.
func (v *version) IPPools() IPPoolInformer {
	return &iPPoolInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// IPPoolDetails returns a IPPoolDetailInformer.
func (v *version) IPPoolDetails() IPPoolDetailInformer {
	return &iPPoolDetailInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// IPRecorders returns a IPRecorderInformer.
func (v *version) IPRecorders() IPRecorderInformer {
	return &iPRecorderInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
