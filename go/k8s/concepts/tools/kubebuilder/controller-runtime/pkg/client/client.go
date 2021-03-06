package client

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type Object interface {
	metav1.Object
	runtime.Object
}
type ObjectList interface {
	metav1.ListInterface
	runtime.Object
}

type Options struct {
	Scheme *runtime.Scheme

	Mapper meta.RESTMapper
}

// k8s client,直接和api-server通信
type client struct {
	typedClient        typedClient
	unstructuredClient unstructuredClient
	scheme             *runtime.Scheme
	mapper             meta.RESTMapper
}

// resetGroupVersionKind is a helper function to restore and preserve GroupVersionKind on an object.
// TODO(vincepri): Remove this function and its calls once controller-runtime dependencies are upgraded to 1.16?
func (c *client) resetGroupVersionKind(obj runtime.Object, gvk schema.GroupVersionKind) {
	if gvk != schema.EmptyObjectKind.GroupVersionKind() {
		if v, ok := obj.(schema.ObjectKind); ok {
			v.SetGroupVersionKind(gvk)
		}
	}
}
func (c *client) Get(ctx context.Context, key ObjectKey, obj runtime.Object) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Get(ctx, key, obj)
	}

	return c.typedClient.Get(ctx, key, obj)
}

func (c *client) List(ctx context.Context, obj runtime.Object, opts ...ListOption) error {
	_, ok := obj.(*unstructured.UnstructuredList)
	log.WithFields(log.Fields{
		"ok": ok,
	}).Debug("[client List]")
	if ok {
		return c.unstructuredClient.List(ctx, obj, opts...)
	}

	return c.typedClient.List(ctx, obj, opts...)
}

func (c *client) Create(ctx context.Context, obj runtime.Object, opts ...CreateOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Create(ctx, obj, opts...)
	}
	return c.typedClient.Create(ctx, obj, opts...)
}

func (c *client) Update(ctx context.Context, obj runtime.Object, opts ...UpdateOption) error {
	defer c.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())

	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Update(ctx, obj, opts...)
	}
	return c.typedClient.Update(ctx, obj, opts...)
}

func (c *client) Patch(ctx context.Context, obj runtime.Object, patch Patch, opts ...PatchOption) error {
	defer c.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())

	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Patch(ctx, obj, patch, opts...)
	}
	return c.typedClient.Patch(ctx, obj, patch, opts...)
}

func (c *client) Delete(ctx context.Context, obj runtime.Object, opts ...DeleteOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.Delete(ctx, obj, opts...)
	}

	return c.typedClient.Delete(ctx, obj, opts...)
}
func (c *client) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...DeleteAllOfOption) error {
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return c.unstructuredClient.DeleteAllOf(ctx, obj, opts...)
	}
	return c.typedClient.DeleteAllOf(ctx, obj, opts...)
}

// statusWriter is client.StatusWriter that writes status subresource
type statusWriter struct {
	client *client
}

func (c *client) Status() StatusWriter {
	return &statusWriter{client: c}
}
func (sw *statusWriter) Update(ctx context.Context, obj runtime.Object, opts ...UpdateOption) error {
	defer sw.client.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return sw.client.unstructuredClient.UpdateStatus(ctx, obj, opts...)
	}
	return sw.client.typedClient.UpdateStatus(ctx, obj, opts...)
}
func (sw *statusWriter) Patch(ctx context.Context, obj runtime.Object, patch Patch, opts ...PatchOption) error {
	defer sw.client.resetGroupVersionKind(obj, obj.GetObjectKind().GroupVersionKind())
	_, ok := obj.(*unstructured.Unstructured)
	if ok {
		return sw.client.unstructuredClient.PatchStatus(ctx, obj, patch, opts...)
	}
	return sw.client.typedClient.PatchStatus(ctx, obj, patch, opts...)
}

func (c *client) Scheme() *runtime.Scheme {
	panic("implement me")
}

func (c *client) RESTMapper() meta.RESTMapper {
	panic("implement me")
}

func New(config *rest.Config, options Options) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("must provide non-nil rest.Config to client.New")
	}
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	// Init a Mapper if none provided
	if options.Mapper == nil {
		var err error
		options.Mapper, err = NewDynamicRESTMapper(config)
		if err != nil {
			return nil, err
		}
	}

	clientcache := &clientCache{
		config:         config,
		scheme:         options.Scheme,
		mapper:         options.Mapper,
		codecs:         serializer.NewCodecFactory(options.Scheme),
		resourceByType: make(map[schema.GroupVersionKind]*resourceMeta),
	}

	c := &client{
		typedClient: typedClient{
			cache:      clientcache,
			paramCodec: runtime.NewParameterCodec(options.Scheme),
		},
		unstructuredClient: unstructuredClient{
			cache:      clientcache,
			paramCodec: noConversionParamCodec{},
		},
		scheme: options.Scheme,
		mapper: options.Mapper,
	}

	return c, nil
}
