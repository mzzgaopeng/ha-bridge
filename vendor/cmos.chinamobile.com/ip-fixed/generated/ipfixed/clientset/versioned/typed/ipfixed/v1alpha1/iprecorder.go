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
// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"time"

	v1alpha1 "cmos.chinamobile.com/ip-fixed/api/ipfixed/v1alpha1"
	scheme "cmos.chinamobile.com/ip-fixed/generated/ipfixed/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// IPRecordersGetter has a method to return a IPRecorderInterface.
// A group's client should implement this interface.
type IPRecordersGetter interface {
	IPRecorders() IPRecorderInterface
}

// IPRecorderInterface has methods to work with IPRecorder resources.
type IPRecorderInterface interface {
	Create(*v1alpha1.IPRecorder) (*v1alpha1.IPRecorder, error)
	Update(*v1alpha1.IPRecorder) (*v1alpha1.IPRecorder, error)
	UpdateStatus(*v1alpha1.IPRecorder) (*v1alpha1.IPRecorder, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.IPRecorder, error)
	List(opts v1.ListOptions) (*v1alpha1.IPRecorderList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.IPRecorder, err error)
	IPRecorderExpansion
}

// iPRecorders implements IPRecorderInterface
type iPRecorders struct {
	client rest.Interface
}

// newIPRecorders returns a IPRecorders
func newIPRecorders(c *IpfixedV1alpha1Client) *iPRecorders {
	return &iPRecorders{
		client: c.RESTClient(),
	}
}

// Get takes name of the iPRecorder, and returns the corresponding iPRecorder object, and an error if there is any.
func (c *iPRecorders) Get(name string, options v1.GetOptions) (result *v1alpha1.IPRecorder, err error) {
	result = &v1alpha1.IPRecorder{}
	err = c.client.Get().
		Resource("iprecorders").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of IPRecorders that match those selectors.
func (c *iPRecorders) List(opts v1.ListOptions) (result *v1alpha1.IPRecorderList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.IPRecorderList{}
	err = c.client.Get().
		Resource("iprecorders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested iPRecorders.
func (c *iPRecorders) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("iprecorders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a iPRecorder and creates it.  Returns the server's representation of the iPRecorder, and an error, if there is any.
func (c *iPRecorders) Create(iPRecorder *v1alpha1.IPRecorder) (result *v1alpha1.IPRecorder, err error) {
	result = &v1alpha1.IPRecorder{}
	err = c.client.Post().
		Resource("iprecorders").
		Body(iPRecorder).
		Do().
		Into(result)
	return
}

// Update takes the representation of a iPRecorder and updates it. Returns the server's representation of the iPRecorder, and an error, if there is any.
func (c *iPRecorders) Update(iPRecorder *v1alpha1.IPRecorder) (result *v1alpha1.IPRecorder, err error) {
	result = &v1alpha1.IPRecorder{}
	err = c.client.Put().
		Resource("iprecorders").
		Name(iPRecorder.Name).
		Body(iPRecorder).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *iPRecorders) UpdateStatus(iPRecorder *v1alpha1.IPRecorder) (result *v1alpha1.IPRecorder, err error) {
	result = &v1alpha1.IPRecorder{}
	err = c.client.Put().
		Resource("iprecorders").
		Name(iPRecorder.Name).
		SubResource("status").
		Body(iPRecorder).
		Do().
		Into(result)
	return
}

// Delete takes name of the iPRecorder and deletes it. Returns an error if one occurs.
func (c *iPRecorders) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("iprecorders").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *iPRecorders) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("iprecorders").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched iPRecorder.
func (c *iPRecorders) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.IPRecorder, err error) {
	result = &v1alpha1.IPRecorder{}
	err = c.client.Patch(pt).
		Resource("iprecorders").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}