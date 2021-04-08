// +build !ignore_autogenerated

/*
Copyright 2021 The Kubernetes Authors.

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeschedulerPolicy) DeepCopyInto(out *DeschedulerPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Strategies != nil {
		in, out := &in.Strategies, &out.Strategies
		*out = make(StrategyList, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(string)
		**out = **in
	}
	if in.EvictLocalStoragePods != nil {
		in, out := &in.EvictLocalStoragePods, &out.EvictLocalStoragePods
		*out = new(bool)
		**out = **in
	}
	if in.EvictSystemCriticalPods != nil {
		in, out := &in.EvictSystemCriticalPods, &out.EvictSystemCriticalPods
		*out = new(bool)
		**out = **in
	}
	if in.IgnorePVCPods != nil {
		in, out := &in.IgnorePVCPods, &out.IgnorePVCPods
		*out = new(bool)
		**out = **in
	}
	if in.MaxNoOfPodsToEvictPerNode != nil {
		in, out := &in.MaxNoOfPodsToEvictPerNode, &out.MaxNoOfPodsToEvictPerNode
		*out = new(int)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeschedulerPolicy.
func (in *DeschedulerPolicy) DeepCopy() *DeschedulerPolicy {
	if in == nil {
		return nil
	}
	out := new(DeschedulerPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DeschedulerPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeschedulerStrategy) DeepCopyInto(out *DeschedulerStrategy) {
	*out = *in
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = new(StrategyParameters)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeschedulerStrategy.
func (in *DeschedulerStrategy) DeepCopy() *DeschedulerStrategy {
	if in == nil {
		return nil
	}
	out := new(DeschedulerStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Namespaces) DeepCopyInto(out *Namespaces) {
	*out = *in
	if in.Include != nil {
		in, out := &in.Include, &out.Include
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Exclude != nil {
		in, out := &in.Exclude, &out.Exclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Namespaces.
func (in *Namespaces) DeepCopy() *Namespaces {
	if in == nil {
		return nil
	}
	out := new(Namespaces)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeResourceUtilizationThresholds) DeepCopyInto(out *NodeResourceUtilizationThresholds) {
	*out = *in
	if in.Thresholds != nil {
		in, out := &in.Thresholds, &out.Thresholds
		*out = make(ResourceThresholds, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.TargetThresholds != nil {
		in, out := &in.TargetThresholds, &out.TargetThresholds
		*out = make(ResourceThresholds, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeResourceUtilizationThresholds.
func (in *NodeResourceUtilizationThresholds) DeepCopy() *NodeResourceUtilizationThresholds {
	if in == nil {
		return nil
	}
	out := new(NodeResourceUtilizationThresholds)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodLifeTime) DeepCopyInto(out *PodLifeTime) {
	*out = *in
	if in.MaxPodLifeTimeSeconds != nil {
		in, out := &in.MaxPodLifeTimeSeconds, &out.MaxPodLifeTimeSeconds
		*out = new(uint)
		**out = **in
	}
	if in.PodStatusPhases != nil {
		in, out := &in.PodStatusPhases, &out.PodStatusPhases
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodLifeTime.
func (in *PodLifeTime) DeepCopy() *PodLifeTime {
	if in == nil {
		return nil
	}
	out := new(PodLifeTime)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodsHavingTooManyRestarts) DeepCopyInto(out *PodsHavingTooManyRestarts) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodsHavingTooManyRestarts.
func (in *PodsHavingTooManyRestarts) DeepCopy() *PodsHavingTooManyRestarts {
	if in == nil {
		return nil
	}
	out := new(PodsHavingTooManyRestarts)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RemoveDuplicates) DeepCopyInto(out *RemoveDuplicates) {
	*out = *in
	if in.ExcludeOwnerKinds != nil {
		in, out := &in.ExcludeOwnerKinds, &out.ExcludeOwnerKinds
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RemoveDuplicates.
func (in *RemoveDuplicates) DeepCopy() *RemoveDuplicates {
	if in == nil {
		return nil
	}
	out := new(RemoveDuplicates)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ResourceThresholds) DeepCopyInto(out *ResourceThresholds) {
	{
		in := &in
		*out = make(ResourceThresholds, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
		return
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceThresholds.
func (in ResourceThresholds) DeepCopy() ResourceThresholds {
	if in == nil {
		return nil
	}
	out := new(ResourceThresholds)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in StrategyList) DeepCopyInto(out *StrategyList) {
	{
		in := &in
		*out = make(StrategyList, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
		return
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StrategyList.
func (in StrategyList) DeepCopy() StrategyList {
	if in == nil {
		return nil
	}
	out := new(StrategyList)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StrategyParameters) DeepCopyInto(out *StrategyParameters) {
	*out = *in
	if in.NodeResourceUtilizationThresholds != nil {
		in, out := &in.NodeResourceUtilizationThresholds, &out.NodeResourceUtilizationThresholds
		*out = new(NodeResourceUtilizationThresholds)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeAffinityType != nil {
		in, out := &in.NodeAffinityType, &out.NodeAffinityType
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PodsHavingTooManyRestarts != nil {
		in, out := &in.PodsHavingTooManyRestarts, &out.PodsHavingTooManyRestarts
		*out = new(PodsHavingTooManyRestarts)
		**out = **in
	}
	if in.PodLifeTime != nil {
		in, out := &in.PodLifeTime, &out.PodLifeTime
		*out = new(PodLifeTime)
		(*in).DeepCopyInto(*out)
	}
	if in.RemoveDuplicates != nil {
		in, out := &in.RemoveDuplicates, &out.RemoveDuplicates
		*out = new(RemoveDuplicates)
		(*in).DeepCopyInto(*out)
	}
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = new(Namespaces)
		(*in).DeepCopyInto(*out)
	}
	if in.ThresholdPriority != nil {
		in, out := &in.ThresholdPriority, &out.ThresholdPriority
		*out = new(int32)
		**out = **in
	}
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StrategyParameters.
func (in *StrategyParameters) DeepCopy() *StrategyParameters {
	if in == nil {
		return nil
	}
	out := new(StrategyParameters)
	in.DeepCopyInto(out)
	return out
}
