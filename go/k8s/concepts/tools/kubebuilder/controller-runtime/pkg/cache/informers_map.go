package cache

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

// MapEntry contains the cached data for an Informer
type MapEntry struct {
	// Informer is the cached informer
	Informer cache.SharedIndexInformer

	// CacheReader wraps Informer and implements the CacheReader interface for a single type
	Reader CacheReader
}

type specificInformersMap struct {
	// Scheme maps runtime.Objects to GroupVersionKinds
	Scheme *runtime.Scheme

	// config is used to talk to the apiserver
	config *rest.Config

	// mapper maps GroupVersionKinds to Resources
	mapper meta.RESTMapper

	// informersByGVK is the cache of informers keyed by groupVersionKind
	informersByGVK map[schema.GroupVersionKind]*MapEntry

	// codecs is used to create a new REST client
	codecs serializer.CodecFactory

	// paramCodec is used by list and watch
	paramCodec runtime.ParameterCodec

	// stop is the stop channel to stop informers
	stop <-chan struct{}

	// resync is the base frequency the informers are resynced
	// a 10 percent jitter will be added to the resync period between informers
	// so that all informers will not send list requests simultaneously.
	resync time.Duration

	// mu guards access to the map
	mu sync.RWMutex

	// start is true if the informers have been started
	started bool

	// startWait is a channel that is closed after the
	// informer has been started.
	startWait chan struct{}

	// createClient knows how to create a client and a list object,
	// and allows for abstracting over the particulars of structured vs
	// unstructured objects.
	createListWatcher createListWatcherFunc

	// namespace is the namespace that all ListWatches are restricted to
	// default or empty string means all namespaces
	namespace string
}

type createListWatcherFunc func(gvk schema.GroupVersionKind, ip *specificInformersMap) (*cache.ListWatch, error)

// Get will create a new Informer and add it to the map of specificInformersMap if none exists.  Returns
// the Informer from the map.
func (ip *specificInformersMap) Get(ctx context.Context, gvk schema.GroupVersionKind, obj runtime.Object) (bool, *MapEntry, error) {
	// Return the informer if it is found
	i, started, ok := func() (*MapEntry, bool, bool) {
		ip.mu.RLock()
		defer ip.mu.RUnlock()
		i, ok := ip.informersByGVK[gvk]
		return i, ip.started, ok
	}()

	if !ok {
		var err error
		if i, started, err = ip.addInformerToMap(gvk, obj); err != nil {
			return started, nil, err
		}
	}

	if started && !i.Informer.HasSynced() {
		// Wait for it to sync before returning the Informer so that folks don't read from a stale cache.
		if !cache.WaitForCacheSync(ctx.Done(), i.Informer.HasSynced) {
			return started, nil, apierrors.NewTimeoutError(fmt.Sprintf("failed waiting for %T Informer to sync", obj), 0)
		}
	}

	return started, i, nil
}

// newListWatch returns a new ListWatch object that can be used to create a SharedIndexInformer.
func createStructuredListWatch(gvk schema.GroupVersionKind, ip *specificInformersMap) (*cache.ListWatch, error) {

}

func createUnstructuredListWatch(gvk schema.GroupVersionKind, ip *specificInformersMap) (*cache.ListWatch, error) {

}

func newSpecificInformersMap(config *rest.Config,
	scheme *runtime.Scheme,
	mapper meta.RESTMapper,
	resync time.Duration,
	namespace string,
	createListWatcher createListWatcherFunc) *specificInformersMap {
	return &specificInformersMap{
		Scheme:            scheme,
		config:            config,
		mapper:            mapper,
		informersByGVK:    make(map[schema.GroupVersionKind]*MapEntry),
		codecs:            serializer.NewCodecFactory(scheme),
		paramCodec:        runtime.NewParameterCodec(scheme),
		resync:            resync,
		startWait:         make(chan struct{}),
		createListWatcher: createListWatcher,
		namespace:         namespace,
	}
}
