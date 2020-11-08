/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	"context"
	v1beta1 "podbuggertool/pkg/apis/podbuggertool/v1beta1"
	scheme "podbuggertool/pkg/generated/clientset/versioned/scheme"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PodbuggertoolsGetter has a method to return a PodbuggertoolInterface.
// A group's client should implement this interface.
type PodbuggertoolsGetter interface {
	Podbuggertools(namespace string) PodbuggertoolInterface
}

// PodbuggertoolInterface has methods to work with Podbuggertool resources.
type PodbuggertoolInterface interface {
	Create(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.CreateOptions) (*v1beta1.Podbuggertool, error)
	Update(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.UpdateOptions) (*v1beta1.Podbuggertool, error)
	UpdateStatus(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.UpdateOptions) (*v1beta1.Podbuggertool, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.Podbuggertool, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.PodbuggertoolList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Podbuggertool, err error)
	PodbuggertoolExpansion
}

// podbuggertools implements PodbuggertoolInterface
type podbuggertools struct {
	client rest.Interface
	ns     string
}

// newPodbuggertools returns a Podbuggertools
func newPodbuggertools(c *UalterV1beta1Client, namespace string) *podbuggertools {
	return &podbuggertools{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the podbuggertool, and returns the corresponding podbuggertool object, and an error if there is any.
func (c *podbuggertools) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Podbuggertool, err error) {
	result = &v1beta1.Podbuggertool{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podbuggertools").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Podbuggertools that match those selectors.
func (c *podbuggertools) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.PodbuggertoolList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.PodbuggertoolList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podbuggertools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested podbuggertools.
func (c *podbuggertools) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("podbuggertools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a podbuggertool and creates it.  Returns the server's representation of the podbuggertool, and an error, if there is any.
func (c *podbuggertools) Create(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.CreateOptions) (result *v1beta1.Podbuggertool, err error) {
	result = &v1beta1.Podbuggertool{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podbuggertools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podbuggertool).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a podbuggertool and updates it. Returns the server's representation of the podbuggertool, and an error, if there is any.
func (c *podbuggertools) Update(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.UpdateOptions) (result *v1beta1.Podbuggertool, err error) {
	result = &v1beta1.Podbuggertool{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podbuggertools").
		Name(podbuggertool.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podbuggertool).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *podbuggertools) UpdateStatus(ctx context.Context, podbuggertool *v1beta1.Podbuggertool, opts v1.UpdateOptions) (result *v1beta1.Podbuggertool, err error) {
	result = &v1beta1.Podbuggertool{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podbuggertools").
		Name(podbuggertool.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podbuggertool).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the podbuggertool and deletes it. Returns an error if one occurs.
func (c *podbuggertools) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podbuggertools").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *podbuggertools) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podbuggertools").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched podbuggertool.
func (c *podbuggertools) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Podbuggertool, err error) {
	result = &v1beta1.Podbuggertool{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("podbuggertools").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
