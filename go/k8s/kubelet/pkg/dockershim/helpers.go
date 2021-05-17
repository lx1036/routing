package dockershim

import (
	"fmt"
	"strings"

	"k8s-lx1036/k8s/kubelet/pkg/types"

	dockertypes "github.com/docker/docker/api/types"
	dockerfilters "github.com/docker/docker/api/types/filters"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
)

const (
	annotationPrefix     = "annotation."
	securityOptSeparator = '='

	dockerNetNSFmt = "/proc/%v/ns/net"
)

var internalLabelKeys = []string{containerTypeLabelKey, containerLogPathLabelKey, sandboxIDLabelKey}

// dockerFilter wraps around dockerfilters.Args and provides methods to modify
// the filter easily.
type dockerFilter struct {
	args *dockerfilters.Args
}

func newDockerFilter(args *dockerfilters.Args) *dockerFilter {
	return &dockerFilter{args: args}
}

func (f *dockerFilter) Add(key, value string) {
	f.args.Add(key, value)
}

func (f *dockerFilter) AddLabel(key, value string) {
	f.Add("label", fmt.Sprintf("%s=%s", key, value))
}

/*
"Labels":{
	"annotation.io.kubernetes.container.hash":"ad01c60e",
	"annotation.io.kubernetes.container.restartCount":"0",
	"annotation.io.kubernetes.container.terminationMessagePath":"/dev/termination-log",
	"annotation.io.kubernetes.container.terminationMessagePolicy":"File",
	"annotation.io.kubernetes.pod.terminationGracePeriod":"30",
	"io.kubernetes.container.logpath":"/var/log/pods/default_cgroup1-75cb7bc8c5-vbzww_cf1f7aa0-acb5-48cf-a2e6-50d34d553d96/cgroup1-0/0.log",
	"io.kubernetes.container.name":"cgroup1-0",
	"io.kubernetes.docker.type":"container",
	"io.kubernetes.pod.name":"cgroup1-75cb7bc8c5-vbzww",
	"io.kubernetes.pod.namespace":"default",
	"io.kubernetes.pod.uid":"cf1f7aa0-acb5-48cf-a2e6-50d34d553d96",
	"io.kubernetes.sandbox.id":"af88d9ab332141c168be34dc49752787d0640f0877b181d94077e24fcf9de497"
}
*/
// makeLabels converts annotations to labels and merge them with the given
// labels. This is necessary because docker does not support annotations;
// we *fake* annotations using labels. Note that docker labels are not
// updatable.
func makeLabels(labels, annotations map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range labels {
		merged[k] = v
	}
	for k, v := range annotations {
		// Assume there won't be conflict.
		merged[fmt.Sprintf("%s%s", annotationPrefix, k)] = v
	}
	return merged
}

// extractLabels converts raw docker labels to the CRI labels and annotations.
// It also filters out internal labels used by this shim.
func extractLabels(input map[string]string) (map[string]string, map[string]string) {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	for k, v := range input {
		// Check if the key is used internally by the shim.
		internal := false
		for _, internalKey := range internalLabelKeys {
			if k == internalKey {
				internal = true
				break
			}
		}
		if internal {
			continue
		}

		// Delete the container name label for the sandbox. It is added in the shim,
		// should not be exposed via CRI.
		if k == types.KubernetesContainerNameLabel &&
			input[containerTypeLabelKey] == containerTypeLabelSandbox {
			continue
		}

		// Check if the label should be treated as an annotation.
		if strings.HasPrefix(k, annotationPrefix) {
			annotations[strings.TrimPrefix(k, annotationPrefix)] = v
			continue
		}
		labels[k] = v
	}
	return labels, annotations
}

func getNetworkNamespace(c *dockertypes.ContainerJSON) (string, error) {
	if c.State.Pid == 0 {
		// Docker reports pid 0 for an exited container.
		return "", fmt.Errorf("cannot find network namespace for the terminated container %q", c.ID)
	}

	return fmt.Sprintf(dockerNetNSFmt, c.State.Pid), nil
}

// generateEnvList converts KeyValue list to a list of strings, in the form of
// '<key>=<value>', which can be understood by docker.
func generateEnvList(envs []*runtimeapi.KeyValue) (result []string) {
	for _, env := range envs {
		result = append(result, fmt.Sprintf("%s=%s", env.Key, env.Value))
	}

	return
}

// generateMountBindings converts the mount list to a list of strings that
// can be understood by docker.
// '<HostPath>:<ContainerPath>[:options]', where 'options'
// is a comma-separated list of the following strings:
// 'ro', if the path is read only
// 'Z', if the volume requires SELinux relabeling
// propagation mode such as 'rslave'
func generateMountBindings(mounts []*runtimeapi.Mount) []string {
	result := make([]string, 0, len(mounts))
	for _, m := range mounts {
		bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
		var attrs []string
		if m.Readonly {
			attrs = append(attrs, "ro")
		}
		// Only request relabeling if the pod provides an SELinux context. If the pod
		// does not provide an SELinux context relabeling will label the volume with
		// the container's randomly allocated MCS label. This would restrict access
		// to the volume to the container which mounts it first.
		if m.SelinuxRelabel {
			attrs = append(attrs, "Z")
		}
		switch m.Propagation {
		case runtimeapi.MountPropagation_PROPAGATION_PRIVATE:
			// noop, private is default
		case runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			attrs = append(attrs, "rshared")
		case runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			attrs = append(attrs, "rslave")
		default:
			klog.Warningf("unknown propagation mode for hostPath %q", m.HostPath)
			// Falls back to "private"
		}

		if len(attrs) > 0 {
			bind = fmt.Sprintf("%s:%s", bind, strings.Join(attrs, ","))
		}
		result = append(result, bind)
	}

	return result
}
