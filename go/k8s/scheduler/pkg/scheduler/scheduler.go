package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"k8s-lx1036/k8s/scheduler/pkg/scheduler/apis/config"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/core"
	framework "k8s-lx1036/k8s/scheduler/pkg/scheduler/framework"
	frameworkplugins "k8s-lx1036/k8s/scheduler/pkg/scheduler/framework/plugins"
	frameworkruntime "k8s-lx1036/k8s/scheduler/pkg/scheduler/framework/runtime"
	internalcache "k8s-lx1036/k8s/scheduler/pkg/scheduler/internal/cache"
	internalqueue "k8s-lx1036/k8s/scheduler/pkg/scheduler/internal/queue"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/metrics"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/profile"
	"k8s-lx1036/k8s/scheduler/pkg/scheduler/util"

	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/apis/core/validation"
)

const (
	pluginMetricsSamplePercent = 10
)

// Scheduler watches for new unscheduled pods. It attempts to find
// nodes that they fit on and writes bindings back to the api server.
type Scheduler struct {
	// It is expected that changes made via SchedulerCache will be observed
	// by NodeLister and Algorithm.
	SchedulerCache internalcache.Cache

	// INFO: 类似 iptables，把各个hooks串起来
	Algorithm core.ScheduleAlgorithm

	// NextPod should be a function that blocks until the next pod
	// is available. We don't use a channel for this, because scheduling
	// a pod may take some amount of time and we don't want pods to get
	// stale while they sit in a channel.
	NextPod func() *framework.QueuedPodInfo

	// Error is called if there is an error. It is passed the pod in
	// question, and the error
	Error func(*framework.QueuedPodInfo, error)

	// Close this to shut down the scheduler.
	StopEverything <-chan struct{}

	// PriorityQueue holds pods to be scheduled
	PriorityQueue internalqueue.PriorityQueue

	// Profiles are the scheduling profiles.
	Profiles profile.Map

	scheduledPodsHasSynced func() bool

	client clientset.Interface

	profiles                 []config.KubeSchedulerProfile
	podInitialBackoffSeconds int64
	podMaxBackoffSeconds     int64
	//recorderFactory profile.RecorderFactory
	informerFactory informers.SharedInformerFactory
	podInformer     coreinformers.PodInformer

	schedulerCache internalcache.Cache
	// Disable pod preemption or not.
	disablePreemption bool
	// Always check all predicates even if the middle of one predicate fails.
	alwaysCheckAllPredicates bool
	// percentageOfNodesToScore specifies percentage of all nodes to score in each scheduling cycle.
	percentageOfNodesToScore int32
	registry                 frameworkruntime.Registry
	nodeInfoSnapshot         *internalcache.Snapshot
	extenders                []config.Extender
	frameworkCapturer        FrameworkCapturer

	// SchedulingQueue holds pods to be scheduled
	SchedulingQueue internalqueue.SchedulingQueue
}

// FrameworkCapturer is used for registering a notify function in building framework.
type FrameworkCapturer func(config.KubeSchedulerProfile)

////////////////////// PriorityQueue ////////////////////////////
func (scheduler *Scheduler) addPodToCache(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Errorf("cannot convert to *v1.Pod: %v", obj)
		return
	}
	klog.Infof("add event for scheduled pod %s/%s ", pod.Namespace, pod.Name)

	// 存入scheduler的cache
	if err := scheduler.SchedulerCache.AddPod(pod); err != nil {
		klog.Errorf("scheduler cache AddPod failed: %v", err)
	}

	// 存入PriorityQueue
	scheduler.PriorityQueue.AssignedPodAdded(pod)
}
func (scheduler *Scheduler) updatePodInCache(oldObj, newObj interface{}) {

}
func (scheduler *Scheduler) deletePodFromCache(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Errorf("cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("cannot convert to *v1.Pod: %v", t)
		return
	}
	klog.Infof("delete event for scheduled pod %s/%s ", pod.Namespace, pod.Name)
	// NOTE: Updates must be written to scheduler cache before invalidating
	// equivalence cache, because we could snapshot equivalence cache after the
	// invalidation and then snapshot the cache itself. If the cache is
	// snapshotted before updates are written, we would update equivalence
	// cache with stale information which is based on snapshot of old cache.
	if err := scheduler.SchedulerCache.RemovePod(pod); err != nil {
		klog.Errorf("scheduler cache RemovePod failed: %v", err)
	}

	scheduler.PriorityQueue.MoveAllToActiveOrBackoffQueue(internalqueue.AssignedPodDelete)
}
func (scheduler *Scheduler) addPodToSchedulingQueue(obj interface{}) {
	pod := obj.(*v1.Pod)
	klog.V(3).Infof("add event for unscheduled pod %s/%s", pod.Namespace, pod.Name)
	if err := scheduler.PriorityQueue.Add(pod); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to queue %T: %v", obj, err))
	}
}
func (scheduler *Scheduler) updatePodInSchedulingQueue(oldObj, newObj interface{}) {
	pod := newObj.(*v1.Pod)
	if scheduler.skipPodUpdate(pod) {
		return
	}
	if err := scheduler.PriorityQueue.Update(oldObj.(*v1.Pod), pod); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to update %T: %v", newObj, err))
	}
}
func (scheduler *Scheduler) deletePodFromSchedulingQueue(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = obj.(*v1.Pod)
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod in %T", obj, scheduler))
			return
		}
	default:
		utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", scheduler, obj))
		return
	}
	klog.V(3).Infof("delete event for unscheduled pod %s/%s", pod.Namespace, pod.Name)
	if err := scheduler.PriorityQueue.Delete(pod); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to dequeue %T: %v", obj, err))
	}
	prof, err := scheduler.profileForPod(pod)
	if err != nil {
		// This shouldn't happen, because we only accept for scheduling the pods
		// which specify a scheduler name that matches one of the profiles.
		klog.Error(err)
		return
	}
	prof.Framework.RejectWaitingPod(pod.UID)
}
func (scheduler *Scheduler) addNodeToCache(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		klog.Errorf("cannot convert to *v1.Node: %v", obj)
		return
	}

	if err := scheduler.SchedulerCache.AddNode(node); err != nil {
		klog.Errorf("scheduler cache AddNode failed: %v", err)
	}

	klog.V(3).Infof("add event for node %q", node.Name)
	scheduler.PriorityQueue.MoveAllToActiveOrBackoffQueue(internalqueue.NodeAdd)
}
func (scheduler *Scheduler) updateNodeInCache(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*v1.Node)
	if !ok {
		klog.Errorf("cannot convert oldObj to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*v1.Node)
	if !ok {
		klog.Errorf("cannot convert newObj to *v1.Node: %v", newObj)
		return
	}

	if err := scheduler.SchedulerCache.UpdateNode(oldNode, newNode); err != nil {
		klog.Errorf("scheduler cache UpdateNode failed: %v", err)
	}

	// Only activate unschedulable pods if the node became more schedulable.
	// We skip the node property comparison when there is no unschedulable pods in the queue
	// to save processing cycles. We still trigger a move to active queue to cover the case
	// that a pod being processed by the scheduler is determined unschedulable. We want this
	// pod to be reevaluated when a change in the cluster happens.
	if scheduler.PriorityQueue.NumUnschedulablePods() == 0 {
		scheduler.PriorityQueue.MoveAllToActiveOrBackoffQueue(internalqueue.Unknown)
	} else if event := nodeSchedulingPropertiesChange(newNode, oldNode); event != "" {
		scheduler.PriorityQueue.MoveAllToActiveOrBackoffQueue(event)
	}
}
func (scheduler *Scheduler) deleteNodeFromCache(obj interface{}) {
	var node *v1.Node
	switch t := obj.(type) {
	case *v1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*v1.Node)
		if !ok {
			klog.Errorf("cannot convert to *v1.Node: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("cannot convert to *v1.Node: %v", t)
		return
	}
	klog.V(3).Infof("delete event for node %q", node.Name)
	// NOTE: Updates must be written to scheduler cache before invalidating
	// equivalence cache, because we could snapshot equivalence cache after the
	// invalidation and then snapshot the cache itself. If the cache is
	// snapshotted before updates are written, we would update equivalence
	// cache with stale information which is based on snapshot of old cache.
	if err := scheduler.SchedulerCache.RemoveNode(node); err != nil {
		klog.Errorf("scheduler cache RemoveNode failed: %v", err)
	}
}

////////////////////// PriorityQueue ////////////////////////////

// Run begins watching and scheduling.
// It waits for cache to be synced, then starts scheduling and blocked until the context is done.
func (scheduler *Scheduler) Run(ctx context.Context) {
	if !cache.WaitForCacheSync(ctx.Done(), scheduler.scheduledPodsHasSynced) {
		return
	}
	scheduler.PriorityQueue.Run()
	wait.UntilWithContext(ctx, scheduler.scheduleOne, 0)
	scheduler.PriorityQueue.Close()
}

// scheduleOne does the entire scheduling workflow for a single pod.
// It is serialized on the scheduling algorithm's host fitting.
func (scheduler *Scheduler) scheduleOne(ctx context.Context) {
	podInfo := scheduler.NextPod()
	// pod could be nil when schedulerQueue is closed
	if podInfo == nil || podInfo.Pod == nil {
		return
	}
	pod := podInfo.Pod
	prof, err := scheduler.profileForPod(pod)
	if err != nil {
		// This shouldn't happen, because we only accept for scheduling the pods
		// which specify a scheduler name that matches one of the profiles.
		klog.Error(err)
		return
	}
	if scheduler.skipPodSchedule(prof, pod) {
		return
	}

	klog.Infof("Attempting to schedule pod: %v/%v", pod.Namespace, pod.Name)

	// INFO: 由 schedule algo 来串起来并实际执行各个plugins
	//start := time.Now()
	state := framework.NewCycleState()
	// INFO: 这里逻辑只有10%概率记录 plugin metrics
	state.SetRecordPluginMetrics(rand.Intn(100) < pluginMetricsSamplePercent)
	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	scheduleResult, err := scheduler.Algorithm.Schedule(schedulingCycleCtx, prof, state, pod)
	if err != nil {
		nominatedNode := ""
		// INFO: 如果pod调度失败，则调用 PostFilter plugin 进行抢占
		if fitError, ok := err.(*core.FitError); ok {
			if !prof.HasPostFilterPlugins() {
				klog.V(3).Infof("No PostFilter plugins are registered, so no preemption will be performed.")
			} else {
				// INFO: PostFilter plugin 其实就是 defaultpreemption.Name plugin，运行 preemption plugin
				result, status := prof.RunPostFilterPlugins(ctx, state, pod, fitError.FilteredNodesStatuses)
				if status.Code() == framework.Error {
					klog.Errorf("Status after running PostFilter plugins for pod %v/%v: %v", pod.Namespace, pod.Name, status)
				} else {
					klog.V(5).Infof("Status after running PostFilter plugins for pod %v/%v: %v", pod.Namespace, pod.Name, status)
				}
				if status.IsSuccess() && result != nil {
					// INFO: 如果抢占成功，则去更新 pod.Status.NominatedNodeName，但是这次调度周期不会立刻更新 pod.Spec.nodeName，
					// 等待下次调度周期去调度。同时，下次调度周期时 pod.Spec.nodeName 未必就是 pod.Status.NominatedNodeName 这个 node
					// 可以去看 k8s.io/api/core/v1/types.go::NominatedNodeName 字段定义描述
					nominatedNode = result.NominatedNodeName
				}
			}
			// metrics
		} else if err == core.ErrNoNodesAvailable {

		} else {
			klog.ErrorS(err, "Error selecting node for pod", "pod", klog.KObj(pod))
		}

		// INFO: 更新 pod.Status.NominatedNodeName，以及更新 pod.Status.Conditions 便于展示信息
		scheduler.recordSchedulingFailure(prof, podInfo, err, v1.PodReasonUnschedulable, nominatedNode)

		return
	}

	// Run "permit" plugins.
	runPermitStatus := prof.RunPermitPlugins(schedulingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
	if runPermitStatus.Code() != framework.Wait && !runPermitStatus.IsSuccess() {

	}

	// 启动goroutine执行bind操作
	go func() {
		waitOnPermitStatus := prof.WaitOnPermit(bindingCycleCtx, assumedPod)
		if !waitOnPermitStatus.IsSuccess() {

		}
		// Run "prebind" plugins.
		preBindStatus := prof.RunPreBindPlugins(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		if !preBindStatus.IsSuccess() {

		}

		err := scheduler.bind(bindingCycleCtx, prof, assumedPod, scheduleResult.SuggestedHost, state)
		if err != nil {

		} else {

			// Run "postbind" plugins.
			prof.RunPostBindPlugins(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		}
	}()

}

// recordSchedulingFailure records an event for the pod that indicates the
// pod has failed to schedule. Also, update the pod condition and nominated node name if set.
func (sched *Scheduler) recordSchedulingFailure(prof *profile.Profile, podInfo *framework.QueuedPodInfo,
	err error, reason string, nominatedNode string) {
	sched.Error(podInfo, err)

	// Update the scheduling queue with the nominated pod information. Without
	// this, there would be a race condition between the next scheduling cycle
	// and the time the scheduler receives a Pod Update for the nominated pod.
	// Here we check for nil only for tests.
	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.AddNominatedPod(podInfo.Pod, nominatedNode)
	}

	pod := podInfo.Pod
	msg := truncateMessage(err.Error())
	prof.Recorder.Eventf(pod, nil, v1.EventTypeWarning, "FailedScheduling", "Scheduling", msg)
	if err := updatePod(sched.client, pod, &v1.PodCondition{
		Type:    v1.PodScheduled,
		Status:  v1.ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	}, nominatedNode); err != nil {
		klog.Errorf("Error updating pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
}

// truncateMessage truncates a message if it hits the NoteLengthLimit.
func truncateMessage(message string) string {
	max := validation.NoteLengthLimit
	if len(message) <= max {
		return message
	}
	suffix := " ..."
	return message[:max-len(suffix)] + suffix
}

func updatePod(client clientset.Interface, pod *v1.Pod, condition *v1.PodCondition, nominatedNode string) error {
	klog.V(3).Infof("Updating pod condition for %s/%s to (%s==%s, Reason=%s)",
		pod.Namespace, pod.Name, condition.Type, condition.Status, condition.Reason)
	podCopy := pod.DeepCopy()
	// NominatedNodeName is updated only if we are trying to set it, and the value is
	// different from the existing one.
	if !podutil.UpdatePodCondition(&podCopy.Status, condition) &&
		(len(nominatedNode) == 0 || pod.Status.NominatedNodeName == nominatedNode) {
		return nil
	}
	if nominatedNode != "" {
		podCopy.Status.NominatedNodeName = nominatedNode
	}

	return util.PatchPod(client, pod, podCopy)
}

////////////////////// Run ////////////////////////////

func (scheduler *Scheduler) profileForPod(pod *v1.Pod) (*profile.Profile, error) {
	prof, ok := scheduler.Profiles[pod.Spec.SchedulerName]
	if !ok {
		return nil, fmt.Errorf("profile not found for scheduler name %q", pod.Spec.SchedulerName)
	}
	return prof, nil
}

// skipPodSchedule returns true if we could skip scheduling the pod for specified cases.
func (scheduler *Scheduler) skipPodSchedule(prof *profile.Profile, pod *v1.Pod) bool {
	// ...
	return false
	// 存入PriorityQueue
	scheduler.PriorityQueue.AssignedPodAdded(pod)
}

// responsibleForPod returns true if the pod has asked to be scheduled by the given scheduler.
func responsibleForPod(pod *v1.Pod, profiles profile.Map) bool {
	return profiles.HandlesSchedulerName(pod.Spec.SchedulerName)
}

type schedulerOptions struct {
	schedulerAlgorithmSource config.SchedulerAlgorithmSource
	percentageOfNodesToScore int32
	podInitialBackoffSeconds int64
	podMaxBackoffSeconds     int64
	// Contains out-of-tree plugins to be merged with the in-tree registry.
	frameworkOutOfTreeRegistry frameworkruntime.Registry
	profiles                   []config.KubeSchedulerProfile
	extenders                  []config.Extender
	frameworkCapturer          FrameworkCapturer
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

func defaultAlgorithmSourceProviderName() *string {
	provider := config.SchedulerDefaultProviderName
	return &provider
}

var defaultSchedulerOptions = schedulerOptions{
	profiles: []config.KubeSchedulerProfile{
		// Profiles' default plugins are set from the algorithm provider.
		{SchedulerName: v1.DefaultSchedulerName},
	},
	schedulerAlgorithmSource: config.SchedulerAlgorithmSource{
		Provider: defaultAlgorithmSourceProviderName(),
	},
	percentageOfNodesToScore: config.DefaultPercentageOfNodesToScore,
	podInitialBackoffSeconds: int64(internalqueue.DefaultPodInitialBackoffDuration.Seconds()),
	podMaxBackoffSeconds:     int64(internalqueue.DefaultPodMaxBackoffDuration.Seconds()),
}

func WithProfiles(p ...config.KubeSchedulerProfile) Option {
	return func(o *schedulerOptions) {
		o.profiles = p
	}
}

func WithPercentageOfNodesToScore(percentageOfNodesToScore int32) Option {
	return func(o *schedulerOptions) {
		o.percentageOfNodesToScore = percentageOfNodesToScore
	}
}

func WithFrameworkOutOfTreeRegistry(registry frameworkruntime.Registry) Option {
	return func(o *schedulerOptions) {
		o.frameworkOutOfTreeRegistry = registry
	}
}

func WithPodInitialBackoffSeconds(podInitialBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.podInitialBackoffSeconds = podInitialBackoffSeconds
	}
}

func WithPodMaxBackoffSeconds(podMaxBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.podMaxBackoffSeconds = podMaxBackoffSeconds
	}
}

// New returns a Scheduler
func New(client clientset.Interface, informerFactory informers.SharedInformerFactory, podInformer coreinformers.PodInformer,
	recorderFactory profile.RecorderFactory, stopCh <-chan struct{}, opts ...Option) (*Scheduler, error) {
	stopEverything := stopCh
	if stopEverything == nil {
		stopEverything = wait.NeverStop
	}

	options := defaultSchedulerOptions
	for _, opt := range opts {
		opt(&options)
	}

	// INFO: scheduler提供了一套扩展机制 scheduler-framework，用来可以合并 out-of-tree registry plugins
	registry := frameworkplugins.NewInTreeRegistry()
	if err := registry.Merge(options.frameworkOutOfTreeRegistry); err != nil {
		return nil, err
	}

	schedulerCache := internalcache.New(30*time.Second, stopEverything)
	snapshot := internalcache.NewEmptySnapshot()

	configurator := &Configurator{
		client:                   client,
		recorderFactory:          recorderFactory,
		informerFactory:          informerFactory,
		podInformer:              podInformer,
		schedulerCache:           schedulerCache,
		StopEverything:           stopEverything,
		percentageOfNodesToScore: options.percentageOfNodesToScore,
		podInitialBackoffSeconds: options.podInitialBackoffSeconds,
		podMaxBackoffSeconds:     options.podMaxBackoffSeconds,
		profiles:                 append([]config.KubeSchedulerProfile(nil), options.profiles...),
		registry:                 registry,
		nodeInfoSnapshot:         snapshot,
		extenders:                options.extenders,
		frameworkCapturer:        options.frameworkCapturer,
	}

	metrics.Register()

	var sched *Scheduler
	source := options.schedulerAlgorithmSource
	switch {
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.createFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		sched = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}

	// Additional tweaks to the config produced by the configurator.
	sched.StopEverything = stopEverything
	sched.client = client
	sched.scheduledPodsHasSynced = podInformer.Informer().HasSynced

	addAllEventHandlers(sched, informerFactory, podInformer)

	return sched, nil
}
