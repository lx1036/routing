package eviction

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strconv"
	"strings"
	"time"
)

const (
	unsupportedEvictionSignal = "unsupported eviction signal %v"
	// Reason is the reason reported back in status.
	Reason = "Evicted"
	// nodeLowMessageFmt is the message for evictions due to resource pressure.
	nodeLowMessageFmt = "The node was low on resource: %v. "
	// nodeConditionMessageFmt is the message for evictions due to resource pressure.
	nodeConditionMessageFmt = "The node had condition: %v. "
	// containerMessageFmt provides additional information for containers exceeding requests
	containerMessageFmt = "Container %s was using %s, which exceeds its request of %s. "
	// containerEphemeralStorageMessageFmt provides additional information for containers which have exceeded their ES limit
	containerEphemeralStorageMessageFmt = "Container %s exceeded its local ephemeral storage limit %q. "
	// podEphemeralStorageMessageFmt provides additional information for pods which have exceeded their ES limit
	podEphemeralStorageMessageFmt = "Pod ephemeral local storage usage exceeds the total limit of containers %s. "
	// emptyDirMessageFmt provides additional information for empty-dir volumes which have exceeded their size limit
	emptyDirMessageFmt = "Usage of EmptyDir volume %q exceeds the limit %q. "
	// inodes, number. internal to this module, used to account for local disk inode consumption.
	resourceInodes v1.ResourceName = "inodes"
	// resourcePids, number. internal to this module, used to account for local pid consumption.
	resourcePids v1.ResourceName = "pids"
	// OffendingContainersKey is the key in eviction event annotations for the list of container names which exceeded their requests
	OffendingContainersKey = "offending_containers"
	// OffendingContainersUsageKey is the key in eviction event annotations for the list of usage of containers which exceeded their requests
	OffendingContainersUsageKey = "offending_containers_usage"
	// StarvedResourceKey is the key for the starved resource in eviction event annotations
	StarvedResourceKey = "starved_resource"
)

const (
	// User visible keys for managing node allocatable enforcement on the node.
	NodeAllocatableEnforcementKey = "pods"
	SystemReservedEnforcementKey  = "system-reserved"
	KubeReservedEnforcementKey    = "kube-reserved"
	NodeAllocatableNoneKey        = "none"
)

var (
	// signalToNodeCondition maps a signal to the node condition to report if threshold is met.
	signalToNodeCondition map[Signal]v1.NodeConditionType
	// signalToResource maps a Signal to its associated Resource.
	signalToResource map[Signal]v1.ResourceName
)

func init() {
	// map eviction signals to node conditions
	signalToNodeCondition = map[Signal]v1.NodeConditionType{}
	signalToNodeCondition[SignalMemoryAvailable] = v1.NodeMemoryPressure
	signalToNodeCondition[SignalAllocatableMemoryAvailable] = v1.NodeMemoryPressure
	signalToNodeCondition[SignalImageFsAvailable] = v1.NodeDiskPressure
	signalToNodeCondition[SignalNodeFsAvailable] = v1.NodeDiskPressure
	signalToNodeCondition[SignalImageFsInodesFree] = v1.NodeDiskPressure
	signalToNodeCondition[SignalNodeFsInodesFree] = v1.NodeDiskPressure
	signalToNodeCondition[SignalPIDAvailable] = v1.NodePIDPressure

	// map signals to resources (and vice-versa)
	signalToResource = map[Signal]v1.ResourceName{}
	signalToResource[SignalMemoryAvailable] = v1.ResourceMemory
	signalToResource[SignalAllocatableMemoryAvailable] = v1.ResourceMemory
	signalToResource[SignalImageFsAvailable] = v1.ResourceEphemeralStorage
	signalToResource[SignalImageFsInodesFree] = resourceInodes
	signalToResource[SignalNodeFsAvailable] = v1.ResourceEphemeralStorage
	signalToResource[SignalNodeFsInodesFree] = resourceInodes
	signalToResource[SignalPIDAvailable] = resourcePids
}

// parseGracePeriods parses the grace period statements
func parseGracePeriods(statements map[string]string) (map[Signal]time.Duration, error) {
	if len(statements) == 0 {
		return nil, nil
	}
	results := map[Signal]time.Duration{}
	for signal, val := range statements {
		signal := Signal(signal)
		if !validSignal(signal) {
			return nil, fmt.Errorf(unsupportedEvictionSignal, signal)
		}
		gracePeriod, err := time.ParseDuration(val)
		if err != nil {
			return nil, err
		}
		if gracePeriod < 0 {
			return nil, fmt.Errorf("invalid eviction grace period specified: %v, must be a positive value", val)
		}
		results[signal] = gracePeriod
	}
	return results, nil
}

// parseMinimumReclaims parses the minimum reclaim statements
func parseMinimumReclaims(statements map[string]string) (map[Signal]ThresholdValue, error) {
	if len(statements) == 0 {
		return nil, nil
	}
	results := map[Signal]ThresholdValue{}
	for signal, val := range statements {
		signal := Signal(signal)
		if !validSignal(signal) {
			return nil, fmt.Errorf(unsupportedEvictionSignal, signal)
		}
		if strings.HasSuffix(val, "%") {
			percentage, err := parsePercentage(val)
			if err != nil {
				return nil, err
			}
			if percentage <= 0 {
				return nil, fmt.Errorf("eviction percentage minimum reclaim %v must be positive: %s", signal, val)
			}
			results[signal] = ThresholdValue{
				Percentage: percentage,
			}
			continue
		}
		quantity, err := resource.ParseQuantity(val)
		if err != nil {
			return nil, err
		}
		if quantity.Sign() < 0 {
			return nil, fmt.Errorf("negative eviction minimum reclaim specified for %v", signal)
		}
		results[signal] = ThresholdValue{
			Quantity: &quantity,
		}
	}
	return results, nil
}

// isHardEvictionThreshold returns true if eviction should immediately occur
func isHardEvictionThreshold(threshold Threshold) bool {
	return threshold.GracePeriod == time.Duration(0)
}
func addAllocatableThresholds(thresholds []Threshold) []Threshold {
	additionalThresholds := []Threshold{}
	for _, threshold := range thresholds {
		if threshold.Signal == SignalMemoryAvailable && isHardEvictionThreshold(threshold) {
			// Copy the SignalMemoryAvailable to SignalAllocatableMemoryAvailable
			additionalThresholds = append(additionalThresholds, Threshold{
				Signal:     SignalAllocatableMemoryAvailable,
				Operator:   threshold.Operator,
				Value:      threshold.Value,
				MinReclaim: threshold.MinReclaim,
			})
		}
	}
	return append(append([]Threshold{}, thresholds...), additionalThresholds...)
}

// ParseThresholdConfig parses the flags for thresholds.
func ParseThresholdConfig(allocatableConfig []string, evictionHard, evictionSoft, evictionSoftGracePeriod, evictionMinimumReclaim map[string]string) ([]Threshold, error) {
	var results []Threshold
	hardThresholds, err := parseThresholdStatements(evictionHard)
	if err != nil {
		return nil, err
	}
	results = append(results, hardThresholds...)
	softThresholds, err := parseThresholdStatements(evictionSoft)
	if err != nil {
		return nil, err
	}
	gracePeriods, err := parseGracePeriods(evictionSoftGracePeriod)
	if err != nil {
		return nil, err
	}
	minReclaims, err := parseMinimumReclaims(evictionMinimumReclaim)
	if err != nil {
		return nil, err
	}
	for i := range softThresholds {
		signal := softThresholds[i].Signal
		period, found := gracePeriods[signal]
		if !found {
			return nil, fmt.Errorf("grace period must be specified for the soft eviction threshold %v", signal)
		}
		softThresholds[i].GracePeriod = period
	}
	results = append(results, softThresholds...)
	for i := range results {
		if minReclaim, ok := minReclaims[results[i].Signal]; ok {
			results[i].MinReclaim = &minReclaim
		}
	}
	for _, key := range allocatableConfig {
		if key == NodeAllocatableEnforcementKey {
			results = addAllocatableThresholds(results)
			break
		}
	}
	return results, nil
}

// parseThresholdStatements parses the input statements into a list of Threshold objects.
func parseThresholdStatements(statements map[string]string) ([]Threshold, error) {
	if len(statements) == 0 {
		return nil, nil
	}
	var results []Threshold
	for signal, val := range statements {
		result, err := parseThresholdStatement(Signal(signal), val)
		if err != nil {
			return nil, err
		}
		if result != nil {
			results = append(results, *result)
		}
	}
	return results, nil
}

// validSignal returns true if the signal is supported.
func validSignal(signal Signal) bool {
	_, found := signalToResource[signal]
	return found
}

// parsePercentage parses a string representing a percentage value
func parsePercentage(input string) (float32, error) {
	value, err := strconv.ParseFloat(strings.TrimRight(input, "%"), 32)
	if err != nil {
		return 0, err
	}
	return float32(value) / 100, nil
}

// parseThresholdStatement parses a threshold statement and returns a threshold,
// or nil if the threshold should be ignored.
func parseThresholdStatement(signal Signal, val string) (*Threshold, error) {
	if !validSignal(signal) {
		return nil, fmt.Errorf(unsupportedEvictionSignal, signal)
	}
	operator := OpForSignal[signal]
	if strings.HasSuffix(val, "%") {
		// ignore 0% and 100%
		if val == "0%" || val == "100%" {
			return nil, nil
		}
		percentage, err := parsePercentage(val)
		if err != nil {
			return nil, err
		}
		if percentage < 0 {
			return nil, fmt.Errorf("eviction percentage threshold %v must be >= 0%%: %s", signal, val)
		}
		if percentage > 100 {
			return nil, fmt.Errorf("eviction percentage threshold %v must be <= 100%%: %s", signal, val)
		}
		return &Threshold{
			Signal:   signal,
			Operator: operator,
			Value: ThresholdValue{
				Percentage: percentage,
			},
		}, nil
	}
	quantity, err := resource.ParseQuantity(val)
	if err != nil {
		return nil, err
	}
	if quantity.Sign() < 0 || quantity.IsZero() {
		return nil, fmt.Errorf("eviction threshold %v must be positive: %s", signal, &quantity)
	}
	return &Threshold{
		Signal:   signal,
		Operator: operator,
		Value: ThresholdValue{
			Quantity: &quantity,
		},
	}, nil
}
