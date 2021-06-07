package recommender

import (
	"k8s-lx1036/k8s/monitor/vpa/recommender/pkg/client/clientset/versioned"
	v1 "k8s-lx1036/k8s/monitor/vpa/recommender/pkg/client/clientset/versioned/typed/autoscaling.k9s.io/v1"
	"k8s-lx1036/k8s/monitor/vpa/recommender/pkg/input"
	"k8s-lx1036/k8s/monitor/vpa/recommender/pkg/types"
	"k8s.io/client-go/rest"
	"time"

	"k8s.io/klog/v2"
)

type Recommender struct {
	clusterState *types.ClusterState

	clusterStateFeeder input.ClusterStateFeeder

	vpaClient v1.AutoscalingV1Interface

	podResourceRecommender PodResourceRecommender
}

func (r *Recommender) RunOnce() {

	r.clusterStateFeeder.LoadVPAs()

	r.clusterStateFeeder.LoadPods()

	r.clusterStateFeeder.LoadRealTimeMetrics()

	r.UpdateVPAs()

}

// Updates VPA CRD objects' statuses.
func (r *Recommender) UpdateVPAs() {
	for _, observedVpa := range r.clusterState.ObservedVpas {
		key := types.VpaID{
			Namespace: observedVpa.Namespace,
			VpaName:   observedVpa.Name,
		}
		vpa, found := r.clusterState.Vpas[key]
		if !found {
			continue
		}

		resources := r.podResourceRecommender.GetRecommendedPodResources(types.GetContainerNameToAggregateStateMap(vpa))
		vpa.UpdateRecommendation(getCappedRecommendation(vpa.ID, resources, observedVpa.Spec.ResourcePolicy))

		hasMatchingPods := vpa.PodCount > 0
		vpa.UpdateConditions(hasMatchingPods)

		_, err := types.UpdateVpaStatusIfNeeded(
			r.vpaClient.VerticalPodAutoscalers(vpa.ID.Namespace), vpa.ID.VpaName, vpa.AsStatus(), &observedVpa.Status)
		if err != nil {
			klog.Errorf(
				"Cannot update VPA %v object. Reason: %+v", vpa.ID.VpaName, err)
		}
	}
}

func NewRecommender(config *rest.Config, checkpointsGCInterval time.Duration, useCheckpoints bool, namespace string) *Recommender {

	return &Recommender{
		clusterState:       nil,
		clusterStateFeeder: input.ClusterStateFeeder{},
		vpaClient:          versioned.NewForConfigOrDie(config).AutoscalingV1(),
	}

}

// getCappedRecommendation creates a recommendation based on recommended pod
// resources, setting the UncappedTarget to the calculated recommended target
// and if necessary, capping the Target, LowerBound and UpperBound according
// to the ResourcePolicy.
func getCappedRecommendation(vpaID types.VpaID, resources logic.RecommendedPodResources,
	policy *v1.PodResourcePolicy) *v1.RecommendedPodResources {

}
