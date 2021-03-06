// *********************************************************************
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-scheduling/scheduler_queues.md
// *********************************************************************

package queue

import (
	"fmt"
	"sync"
	"time"

	framework "k8s-lx1036/k8s/scheduler/pkg/scheduler/framework"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/internal/heap"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/metrics"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/util"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// DefaultPodInitialBackoffDuration is the default value for the initial backoff duration
	// for unschedulable pods. To change the default podInitialBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultPodInitialBackoffDuration time.Duration = 1 * time.Second

	// DefaultPodMaxBackoffDuration is the default value for the max backoff duration
	// for unschedulable pods. To change the default podMaxBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultPodMaxBackoffDuration time.Duration = 10 * time.Second
)

// Events that trigger scheduler queue to change.
const (
	// Unknown event
	Unknown = "Unknown"
	// PodAdd is the event when a new pod is added to API server.
	PodAdd = "PodAdd"
	// NodeAdd is the event when a new node is added to the cluster.
	NodeAdd = "NodeAdd"
	// ScheduleAttemptFailure is the event when a schedule attempt fails.
	ScheduleAttemptFailure = "ScheduleAttemptFailure"
	// BackoffComplete is the event when a pod finishes backoff.
	BackoffComplete = "BackoffComplete"
	// UnschedulableTimeout is the event when a pod stays in unschedulable for longer than timeout.
	UnschedulableTimeout = "UnschedulableTimeout"
	// AssignedPodAdd is the event when a pod is added that causes pods with matching affinity terms
	// to be more schedulable.
	AssignedPodAdd = "AssignedPodAdd"
	// AssignedPodUpdate is the event when a pod is updated that causes pods with matching affinity
	// terms to be more schedulable.
	AssignedPodUpdate = "AssignedPodUpdate"
	// AssignedPodDelete is the event when a pod is deleted that causes pods with matching affinity
	// terms to be more schedulable.
	AssignedPodDelete = "AssignedPodDelete"
	// PvAdd is the event when a persistent volume is added in the cluster.
	PvAdd = "PvAdd"
	// PvUpdate is the event when a persistent volume is updated in the cluster.
	PvUpdate = "PvUpdate"
	// PvcAdd is the event when a persistent volume claim is added in the cluster.
	PvcAdd = "PvcAdd"
	// PvcUpdate is the event when a persistent volume claim is updated in the cluster.
	PvcUpdate = "PvcUpdate"
	// StorageClassAdd is the event when a StorageClass is added in the cluster.
	StorageClassAdd = "StorageClassAdd"
	// ServiceAdd is the event when a service is added in the cluster.
	ServiceAdd = "ServiceAdd"
	// ServiceUpdate is the event when a service is updated in the cluster.
	ServiceUpdate = "ServiceUpdate"
	// ServiceDelete is the event when a service is deleted in the cluster.
	ServiceDelete = "ServiceDelete"
	// CSINodeAdd is the event when a CSI node is added in the cluster.
	CSINodeAdd = "CSINodeAdd"
	// CSINodeUpdate is the event when a CSI node is updated in the cluster.
	CSINodeUpdate = "CSINodeUpdate"
	// NodeSpecUnschedulableChange is the event when unschedulable node spec is changed.
	NodeSpecUnschedulableChange = "NodeSpecUnschedulableChange"
	// NodeAllocatableChange is the event when node allocatable is changed.
	NodeAllocatableChange = "NodeAllocatableChange"
	// NodeLabelsChange is the event when node label is changed.
	NodeLabelChange = "NodeLabelChange"
	// NodeTaintsChange is the event when node taint is changed.
	NodeTaintChange = "NodeTaintChange"
	// NodeConditionChange is the event when node condition is changed.
	NodeConditionChange = "NodeConditionChange"
)

// SchedulingQueue is an interface for a queue to store pods waiting to be scheduled.
// The interface follows a pattern similar to cache.FIFO and cache.Heap and
// makes it easy to use those data structures as a SchedulingQueue.
type SchedulingQueue interface {
	framework.PodNominator
	Add(pod *v1.Pod) error
	// AddUnschedulableIfNotPresent adds an unschedulable pod back to scheduling queue.
	// The podSchedulingCycle represents the current scheduling cycle number which can be
	// returned by calling SchedulingCycle().
	AddUnschedulableIfNotPresent(pod *framework.QueuedPodInfo, podSchedulingCycle int64) error
	// SchedulingCycle returns the current number of scheduling cycle which is
	// cached by scheduling queue. Normally, incrementing this number whenever
	// a pod is popped (e.g. called Pop()) is enough.
	SchedulingCycle() int64
	// Pop removes the head of the queue and returns it. It blocks if the
	// queue is empty and waits until a new item is added to the queue.
	Pop() (*framework.QueuedPodInfo, error)
	Update(oldPod, newPod *v1.Pod) error
	Delete(pod *v1.Pod) error
	MoveAllToActiveOrBackoffQueue(event string)
	AssignedPodAdded(pod *v1.Pod)
	AssignedPodUpdated(pod *v1.Pod)
	PendingPods() []*v1.Pod
	// Close closes the SchedulingQueue so that the goroutine which is
	// waiting to pop items can exit gracefully.
	Close()
	// NumUnschedulablePods returns the number of unschedulable pods exist in the SchedulingQueue.
	NumUnschedulablePods() int
	// Run starts the goroutines managing the queue.
	Run()
}

// PriorityQueue implements a scheduling queue.
// The head of PriorityQueue is the highest priority pending pod. This structure
// has three sub queues. One sub-queue holds pods that are being considered for
// scheduling. This is called activeQ and is a Heap. Another queue holds
// pods that are already tried and are determined to be unschedulable. The latter
// is called unschedulableQ. The third queue holds pods that are moved from
// unschedulable queues and will be moved to active queue when backoff are completed.
type PriorityQueue struct {
	// PodNominator abstracts the operations to maintain nominated Pods.
	PodNominator framework.PodNominator

	stop  chan struct{}
	clock util.Clock

	// pod initial backoff duration.
	podInitialBackoffDuration time.Duration
	// pod maximum backoff duration.
	podMaxBackoffDuration time.Duration

	lock sync.RWMutex
	cond sync.Cond

	// activeQ is heap structure that scheduler actively looks at to find pods to
	// schedule. Head of heap is the highest priority pod.
	activeQ *heap.Heap
	// podBackoffQ is a heap ordered by backoff expiry. Pods which have completed backoff
	// are popped from this heap before the scheduler looks at activeQ
	podBackoffQ *heap.Heap
	// unschedulableQ holds pods that have been tried and determined unschedulable.
	unschedulableQ *UnschedulablePodsMap
	// schedulingCycle represents sequence number of scheduling cycle and is incremented
	// when a pod is popped.
	schedulingCycle int64
	// moveRequestCycle caches the sequence number of scheduling cycle when we
	// received a move request. Unscheduable pods in and before this scheduling
	// cycle will be put back to activeQueue if we were trying to schedule them
	// when we received move request.
	moveRequestCycle int64

	// closed indicates that the queue is closed.
	// It is mainly used to let Pop() exit its control loop while waiting for an item.
	closed bool
}

func (p *PriorityQueue) AddNominatedPod(pod *v1.Pod, nodeName string) {
	panic("implement me")
}

func (p *PriorityQueue) DeleteNominatedPodIfExists(pod *v1.Pod) {
	panic("implement me")
}

func (p *PriorityQueue) UpdateNominatedPod(oldPod, newPod *v1.Pod) {
	panic("implement me")
}

func (p *PriorityQueue) NominatedPodsForNode(nodeName string) []*v1.Pod {
	panic("implement me")
}

func (p *PriorityQueue) Update(oldPod, newPod *v1.Pod) error {
	panic("implement me")
}

func (p *PriorityQueue) Delete(pod *v1.Pod) error {
	panic("implement me")
}

func (p *PriorityQueue) AssignedPodUpdated(pod *v1.Pod) {
	panic("implement me")
}

func (p *PriorityQueue) PendingPods() []*v1.Pod {
	panic("implement me")
}

func (p *PriorityQueue) Close() {
	panic("implement me")
}

func (p *PriorityQueue) NumUnschedulablePods() int {
	panic("implement me")
}

func (p *PriorityQueue) Run() {
	panic("implement me")
}

// newQueuedPodInfo builds a QueuedPodInfo object.
func (p *PriorityQueue) newQueuedPodInfo(pod *v1.Pod) *framework.QueuedPodInfo {
	now := p.clock.Now()
	return &framework.QueuedPodInfo{
		Pod:                     pod,
		Timestamp:               now,
		InitialAttemptTimestamp: now,
	}
}
func newQueuedPodInfoNoTimestamp(pod *v1.Pod) *framework.QueuedPodInfo {
	return &framework.QueuedPodInfo{
		Pod: pod,
	}
}

// add pod to activeQ
func (p *PriorityQueue) Add(pod *v1.Pod) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	podInfo := p.newQueuedPodInfo(pod)
	if err := p.activeQ.Add(podInfo); err != nil {
		klog.Errorf("Error adding pod %s/%s to the scheduling queue: %v", pod.Namespace, pod.Name, err)
		return err
	}
	if p.unschedulableQ.get(pod) != nil {
		klog.Errorf("Error: pod %s/%s is already in the unschedulable queue.", pod.Namespace, pod.Name)
		p.unschedulableQ.delete(pod)
	}
	// Delete pod from backoffQ if it is backing off
	if err := p.podBackoffQ.Delete(podInfo); err == nil {
		klog.Errorf("Error: pod %s/%s is already in the podBackoff queue.", pod.Namespace, pod.Name)
	}

	p.PodNominator.AddNominatedPod(pod, "")
	p.cond.Broadcast()

	return nil
}

func (p *PriorityQueue) AddUnschedulableIfNotPresent(pInfo *framework.QueuedPodInfo, podSchedulingCycle int64) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	pod := pInfo.Pod
	if p.unschedulableQ.get(pod) != nil {
		return fmt.Errorf("pod: %s/%s is already present in unschedulable queue", pod.Namespace, pod.Name)
	}

	// Refresh the timestamp since the pod is re-added.
	pInfo.Timestamp = p.clock.Now()
	if _, exists, _ := p.activeQ.Get(pInfo); exists {
		return fmt.Errorf("pod: %s/%s is already present in the active queue", pod.Namespace, pod.Name)
	}
	if _, exists, _ := p.podBackoffQ.Get(pInfo); exists {
		return fmt.Errorf("pod %s/%s is already present in the backoff queue", pod.Namespace, pod.Name)
	}

	// If a move request has been received, move it to the BackoffQ, otherwise move it to unschedulableQ.
	if p.moveRequestCycle >= podSchedulingCycle {
		if err := p.podBackoffQ.Add(pInfo); err != nil {
			return fmt.Errorf("error adding pod %v to the backoff queue: %v", pod.Name, err)
		}
	} else {
		p.unschedulableQ.addOrUpdate(pInfo)
	}

	p.PodNominator.AddNominatedPod(pod, "")
	return nil
}

const queueClosed = "scheduling queue is closed"

// 最大堆activeQ中pop一个pod出来，没有则一直block等待，同时p.schedulingCycle++
// Pop() 函数会阻塞，这点很重要！！！
func (p *PriorityQueue) Pop() (*framework.QueuedPodInfo, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for p.activeQ.Len() == 0 {
		// When the queue is empty, invocation of Pop() is blocked until new item is enqueued.
		// When Close() is called, the p.closed is set and the condition is broadcast,
		// which causes this loop to continue and return from the Pop().
		if p.closed {
			return nil, fmt.Errorf(queueClosed)
		}
		p.cond.Wait()
	}

	obj, err := p.activeQ.Pop()
	if err != nil {
		return nil, err
	}
	pInfo := obj.(*framework.QueuedPodInfo)
	pInfo.Attempts++
	p.schedulingCycle++
	return pInfo, err
}

// 该pod会把unschedulableQ中与其affinity匹配的pod放到activeQ中
// 这样可以使得两个亲和性pod优先被调度起来
func (p *PriorityQueue) AssignedPodAdded(pod *v1.Pod) {
	p.lock.Lock()
	p.movePodsToActiveOrBackoffQueue(p.getUnschedulablePodsWithMatchingAffinityTerm(pod), AssignedPodAdd)
	p.lock.Unlock()
}

// 从 unschedulableQ 中寻找pods，该pods需要match到输入的pod affinity
func (p *PriorityQueue) getUnschedulablePodsWithMatchingAffinityTerm(pod *v1.Pod) []*framework.QueuedPodInfo {
	var podsToMove []*framework.QueuedPodInfo
	for _, pInfo := range p.unschedulableQ.podInfoMap {
		up := pInfo.Pod
		terms := util.GetPodAffinityTerms(up.Spec.Affinity)
		for _, term := range terms {
			namespaces := util.GetNamespacesFromPodAffinityTerm(up, &term)
			selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
			if err != nil {
				klog.Errorf("Error getting label selectors for pod: %v.", up.Name)
			}
			if util.PodMatchesTermsNamespaceAndSelector(pod, namespaces, selector) {
				podsToMove = append(podsToMove, pInfo)
				break
			}
		}
	}

	return podsToMove
}

// 把 unschedulableQ 和 podBackoffQ 全部 move 到 activeQ
func (p *PriorityQueue) MoveAllToActiveOrBackoffQueue(event string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	unschedulablePods := make([]*framework.QueuedPodInfo, 0, len(p.unschedulableQ.podInfoMap))
	for _, pInfo := range p.unschedulableQ.podInfoMap {
		unschedulablePods = append(unschedulablePods, pInfo)
	}
	p.movePodsToActiveOrBackoffQueue(unschedulablePods, event)
}
func (p *PriorityQueue) movePodsToActiveOrBackoffQueue(podInfoList []*framework.QueuedPodInfo, event string) {
	for _, pInfo := range podInfoList {
		pod := pInfo.Pod
		if p.isPodBackingoff(pInfo) { // unschedulableQ -> podBackoffQ
			if err := p.podBackoffQ.Add(pInfo); err != nil {
				klog.Errorf("Error adding pod %v to the backoff queue: %v", pod.Name, err)
			} else {
				p.unschedulableQ.delete(pod)
			}
		} else { // unschedulableQ -> activeQ
			if err := p.activeQ.Add(pInfo); err != nil {
				klog.Errorf("Error adding pod %v to the scheduling queue: %v", pod.Name, err)
			} else {
				p.unschedulableQ.delete(pod)
			}
		}
	}

	p.moveRequestCycle = p.schedulingCycle
	p.cond.Broadcast()
}

// 判断是不是 podBackoff pod
func (p *PriorityQueue) isPodBackingoff(podInfo *framework.QueuedPodInfo) bool {
	return p.getBackoffTime(podInfo).After(p.clock.Now())
}

func podInfoKeyFunc(obj interface{}) (string, error) {
	return cache.MetaNamespaceKeyFunc(obj.(*framework.QueuedPodInfo).Pod)
}
func (p *PriorityQueue) podsCompareBackoffCompleted(podInfo1, podInfo2 interface{}) bool {
	pInfo1 := podInfo1.(*framework.QueuedPodInfo)
	pInfo2 := podInfo2.(*framework.QueuedPodInfo)
	bo1 := p.getBackoffTime(pInfo1)
	bo2 := p.getBackoffTime(pInfo2)
	return bo1.Before(bo2)
}

// getBackoffTime returns the time that podInfo completes backoff
func (p *PriorityQueue) getBackoffTime(podInfo *framework.QueuedPodInfo) time.Time {
	duration := p.calculateBackoffDuration(podInfo)
	backoffTime := podInfo.Timestamp.Add(duration)
	return backoffTime
}

// p.podInitialBackoffDuration 每次翻倍，次数不能超过podInfo.Attempts，也不能超过最大值
func (p *PriorityQueue) calculateBackoffDuration(podInfo *framework.QueuedPodInfo) time.Duration {
	duration := p.podInitialBackoffDuration
	for i := 1; i < podInfo.Attempts; i++ {
		duration = duration * 2
		if duration > p.podMaxBackoffDuration {
			return p.podMaxBackoffDuration
		}
	}
	return duration
}

func (p *PriorityQueue) SchedulingCycle() int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.schedulingCycle
}

type priorityQueueOptions struct {
	clock                     util.Clock
	podInitialBackoffDuration time.Duration
	podMaxBackoffDuration     time.Duration
	podNominator              framework.PodNominator
}

type Option func(*priorityQueueOptions)

func WithClock(clock util.Clock) Option {
	return func(o *priorityQueueOptions) {
		o.clock = clock
	}
}

// WithPodInitialBackoffDuration sets pod initial backoff duration for PriorityQueue.
func WithPodInitialBackoffDuration(duration time.Duration) Option {
	return func(o *priorityQueueOptions) {
		o.podInitialBackoffDuration = duration
	}
}

// WithPodMaxBackoffDuration sets pod max backoff duration for PriorityQueue.
func WithPodMaxBackoffDuration(duration time.Duration) Option {
	return func(o *priorityQueueOptions) {
		o.podMaxBackoffDuration = duration
	}
}

// WithPodNominator sets pod nominator for PriorityQueue.
func WithPodNominator(pn framework.PodNominator) Option {
	return func(o *priorityQueueOptions) {
		o.podNominator = pn
	}
}

var defaultPriorityQueueOptions = priorityQueueOptions{
	clock:                     util.RealClock{},
	podInitialBackoffDuration: DefaultPodInitialBackoffDuration,
	podMaxBackoffDuration:     DefaultPodMaxBackoffDuration,
}

// NewSchedulingQueue initializes a priority queue as a new scheduling queue.
func NewSchedulingQueue(lessFn framework.LessFunc, opts ...Option) SchedulingQueue {
	return NewPriorityQueue(lessFn, opts...)
}

// NewPriorityQueue creates a PriorityQueue object.
func NewPriorityQueue(lessFn framework.LessFunc, opts ...Option) *PriorityQueue {
	options := defaultPriorityQueueOptions
	for _, opt := range opts {
		opt(&options)
	}

	comp := func(podInfo1, podInfo2 interface{}) bool {
		pInfo1 := podInfo1.(*framework.QueuedPodInfo)
		pInfo2 := podInfo2.(*framework.QueuedPodInfo)
		return lessFn(pInfo1, pInfo2)
	}

	if options.podNominator == nil {
		options.podNominator = NewPodNominator()
	}

	pq := &PriorityQueue{
		PodNominator:              options.podNominator,
		clock:                     options.clock,
		stop:                      make(chan struct{}),
		podInitialBackoffDuration: options.podInitialBackoffDuration,
		podMaxBackoffDuration:     options.podMaxBackoffDuration,
		activeQ:                   heap.New(podInfoKeyFunc, comp),
		unschedulableQ:            newUnschedulablePodsMap(metrics.NewUnschedulablePodsRecorder()),
		moveRequestCycle:          -1,
	}
	pq.cond.L = &pq.lock
	pq.podBackoffQ = heap.New(podInfoKeyFunc, pq.podsCompareBackoffCompleted)

	return pq
}

// MakeNextPodFunc returns a function to retrieve the next pod from a given
// scheduling queue
/*func MakeNextPodFunc(queue SchedulingQueue) func() *framework.QueuedPodInfo {
	return func() *framework.QueuedPodInfo {
		podInfo, err := queue.Pop()
		if err == nil {
			klog.Infof("About to try and schedule pod %v/%v", podInfo.Pod.Namespace, podInfo.Pod.Name)
			return podInfo
		}
		klog.Errorf("Error while retrieving next pod from scheduling queue: %v", err)
		return nil
	}
}*/

// UnschedulablePodsMap holds pods that cannot be scheduled. This data structure
// is used to implement unschedulableQ.
type UnschedulablePodsMap struct {
	// podInfoMap is a map key by a pod's full-name and the value is a pointer to the QueuedPodInfo.
	podInfoMap map[string]*framework.QueuedPodInfo
	keyFunc    func(*v1.Pod) string
	// metricRecorder updates the counter when elements of an unschedulablePodsMap
	// get added or removed, and it does nothing if it's nil
	metricRecorder metrics.MetricRecorder
}

func newUnschedulablePodsMap(metricRecorder metrics.MetricRecorder) *UnschedulablePodsMap {
	return &UnschedulablePodsMap{
		podInfoMap:     make(map[string]*framework.QueuedPodInfo),
		keyFunc:        util.GetPodFullName,
		metricRecorder: metricRecorder,
	}
}

func (u *UnschedulablePodsMap) addOrUpdate(pInfo *framework.QueuedPodInfo) {
	u.podInfoMap[u.keyFunc(pInfo.Pod)] = pInfo
}

func (u *UnschedulablePodsMap) get(pod *v1.Pod) *framework.QueuedPodInfo {
	podKey := u.keyFunc(pod)
	if pInfo, exists := u.podInfoMap[podKey]; exists {
		return pInfo
	}
	return nil
}

func (u *UnschedulablePodsMap) delete(pod *v1.Pod) {
	podID := u.keyFunc(pod)
	delete(u.podInfoMap, podID)
}

// nominatedPodMap is a structure that stores pods nominated to run on nodes.
// It exists because nominatedNodeName of pod objects stored in the structure
// may be different than what scheduler has here. We should be able to find pods
// by their UID and update/delete them.
type nominatedPodMap struct {
	// nominatedPods is a map keyed by a node name and the value is a list of
	// pods which are nominated to run on the node. These are pods which can be in
	// the activeQ or unschedulableQ.
	nominatedPods map[string][]*v1.Pod
	// nominatedPodToNode is map keyed by a Pod UID to the node name where it is
	// nominated.
	nominatedPodToNode map[ktypes.UID]string

	sync.RWMutex
}

func (npm *nominatedPodMap) AddNominatedPod(pod *v1.Pod, nodeName string) {
	npm.Lock()
	npm.add(pod, nodeName)
	npm.Unlock()
}
func (npm *nominatedPodMap) add(pod *v1.Pod, nodeName string) {
	// always delete the pod if it already exist, to ensure we never store more than
	// one instance of the pod.
	npm.delete(pod)

	nnn := nodeName
	if len(nnn) == 0 {
		nnn = pod.Status.NominatedNodeName
		if len(nnn) == 0 {
			return
		}
	}
	npm.nominatedPodToNode[pod.UID] = nnn
	for _, np := range npm.nominatedPods[nnn] {
		if np.UID == pod.UID {
			klog.V(4).Infof("Pod %v/%v already exists in the nominated map!", pod.Namespace, pod.Name)
			return
		}
	}
	npm.nominatedPods[nnn] = append(npm.nominatedPods[nnn], pod)
}
func (npm *nominatedPodMap) delete(p *v1.Pod) {
	nnn, ok := npm.nominatedPodToNode[p.UID]
	if !ok {
		return
	}
	for i, np := range npm.nominatedPods[nnn] {
		if np.UID == p.UID {
			npm.nominatedPods[nnn] = append(npm.nominatedPods[nnn][:i], npm.nominatedPods[nnn][i+1:]...)
			if len(npm.nominatedPods[nnn]) == 0 {
				delete(npm.nominatedPods, nnn)
			}
			break
		}
	}
	delete(npm.nominatedPodToNode, p.UID)
}

func (n *nominatedPodMap) DeleteNominatedPodIfExists(pod *v1.Pod) {
	panic("implement me")
}

func (n *nominatedPodMap) UpdateNominatedPod(oldPod, newPod *v1.Pod) {
	panic("implement me")
}

func (n *nominatedPodMap) NominatedPodsForNode(nodeName string) []*v1.Pod {
	panic("implement me")
}

// NewPodNominator creates a nominatedPodMap as a backing of framework.PodNominator.
func NewPodNominator() *nominatedPodMap {
	return &nominatedPodMap{
		nominatedPods:      make(map[string][]*v1.Pod),
		nominatedPodToNode: make(map[ktypes.UID]string),
	}
}
