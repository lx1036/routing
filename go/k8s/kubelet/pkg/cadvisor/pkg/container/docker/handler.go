package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	"k8s-lx1036/k8s/kubelet/pkg/cadvisor/pkg/container"
	"k8s-lx1036/k8s/kubelet/pkg/cadvisor/pkg/container/common"
	"k8s-lx1036/k8s/kubelet/pkg/cadvisor/pkg/container/libcontainer"
	"k8s-lx1036/k8s/kubelet/pkg/cadvisor/pkg/fs"
	"k8s-lx1036/k8s/kubelet/pkg/cadvisor/pkg/info/v1"

	dockercontainer "github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
)

const (
	// The read write layers exist here.
	aufsRWLayer     = "diff"
	overlayRWLayer  = "upper"
	overlay2RWLayer = "diff"

	// Path to the directory where docker stores log files if the json logging driver is enabled.
	pathToContainersDir = "containers"
)

type dockerContainerHandler struct {
	// machineInfoFactory provides v1.MachineInfo
	machineInfoFactory v1.MachineInfoFactory

	// Absolute path to the cgroup hierarchies of this container.
	// (e.g.: "cpu" -> "/sys/fs/cgroup/cpu/test")
	cgroupPaths map[string]string

	// the docker storage driver
	storageDriver    storageDriver
	fsInfo           fs.FsInfo
	rootfsStorageDir string

	// Time at which this container was created.
	creationTime time.Time

	// Metadata associated with the container.
	envs   map[string]string
	labels map[string]string

	// Image name used for this container.
	image string

	// The network mode of the container
	networkMode dockercontainer.NetworkMode

	// Filesystem handler.
	fsHandler common.FsHandler

	// The IP address of the container
	ipAddress string

	includedMetrics container.MetricSet

	// the devicemapper poolname
	poolName string

	// zfsParent is the parent for docker zfs
	zfsParent string

	// Reference to the container
	reference v1.ContainerReference

	libcontainerHandler *libcontainer.Handler
}

func (h *dockerContainerHandler) ContainerReference() (v1.ContainerReference, error) {
	panic("implement me")
}

func (h *dockerContainerHandler) GetSpec() (v1.ContainerSpec, error) {
	panic("implement me")
}

func (h *dockerContainerHandler) needNet() bool {
	if h.includedMetrics.Has(container.NetworkUsageMetrics) {
		return !h.networkMode.IsContainer()
	}
	return false
}

// TODO: Get from libcontainer API instead of cgroup manager when we don't have to support older Dockers.
func (h *dockerContainerHandler) GetStats() (*v1.ContainerStats, error) {
	stats, err := h.libcontainerHandler.GetStats()
	if err != nil {
		return stats, err
	}

	// Clean up stats for containers that don't have their own network - this
	// includes containers running in Kubernetes pods that use the network of the
	// infrastructure container. This stops metrics being reported multiple times
	// for each container in a pod.
	if !h.needNet() {
		//stats.Network = v1.NetworkStats{}
	}

	// Get filesystem stats.
	err = h.getFsStats(stats)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func (h *dockerContainerHandler) getFsStats(stats *v1.ContainerStats) error {
	mi, err := h.machineInfoFactory.GetMachineInfo()
	if err != nil {
		return err
	}

	if h.includedMetrics.Has(container.DiskIOMetrics) {
		//common.AssignDeviceNamesToDiskStats((*common.MachineInfoNamer)(mi), &stats.DiskIo)
	}

	if !h.includedMetrics.Has(container.DiskUsageMetrics) {
		return nil
	}
	var device string
	switch h.storageDriver {
	case devicemapperStorageDriver:
		// Device has to be the pool name to correlate with the device name as
		// set in the machine info filesystems.
		device = h.poolName
	case aufsStorageDriver, overlayStorageDriver, overlay2StorageDriver, vfsStorageDriver:
		deviceInfo, err := h.fsInfo.GetDirFsDevice(h.rootfsStorageDir)
		if err != nil {
			return fmt.Errorf("unable to determine device info for dir: %v: %v", h.rootfsStorageDir, err)
		}
		device = deviceInfo.Device
	case zfsStorageDriver:
		device = h.zfsParent
	default:
		return nil
	}

	var (
		limit  uint64
		fsType string
	)

	// Docker does not impose any filesystem limits for containers. So use capacity as limit.
	for _, filesystem := range mi.Filesystems {
		if filesystem.Device == device {
			limit = filesystem.Capacity
			fsType = filesystem.Type
			break
		}
	}

	fsStat := v1.FsStats{Device: device, Type: fsType, Limit: limit}
	usage := h.fsHandler.Usage()
	fsStat.BaseUsage = usage.BaseUsageBytes
	fsStat.Usage = usage.TotalUsageBytes
	fsStat.Inodes = usage.InodeUsage

	stats.Filesystem = append(stats.Filesystem, fsStat)

	return nil
}

func (h *dockerContainerHandler) ListContainers(listType container.ListType) ([]v1.ContainerReference, error) {
	// No-op for Docker driver.
	return []v1.ContainerReference{}, nil
}

func (h *dockerContainerHandler) ListProcesses(listType container.ListType) ([]int, error) {
	panic("implement me")
}

func (h *dockerContainerHandler) GetCgroupPath(resource string) (string, error) {
	panic("implement me")
}

func (h *dockerContainerHandler) GetContainerLabels() map[string]string {
	panic("implement me")
}

func (h *dockerContainerHandler) GetContainerIPAddress() string {
	panic("implement me")
}

func (h *dockerContainerHandler) Exists() bool {
	panic("implement me")
}

func (h *dockerContainerHandler) Cleanup() {
	panic("implement me")
}

func (h *dockerContainerHandler) Start() {
	panic("implement me")
}

func (h *dockerContainerHandler) Type() container.ContainerType {
	panic("implement me")
}

func getRwLayerID(containerID, storageDir string, sd storageDriver, dockerVersion []int) (string, error) {
	const (
		// Docker version >=1.10.0 have a randomized ID for the root fs of a container.
		randomizedRWLayerMinorVersion = 10
		rwLayerIDFile                 = "mount-id"
	)
	if (dockerVersion[0] <= 1) && (dockerVersion[1] < randomizedRWLayerMinorVersion) {
		return containerID, nil
	}

	bytes, err := ioutil.ReadFile(path.Join(storageDir, "image", string(sd), "layerdb", "mounts", containerID, rwLayerIDFile))
	if err != nil {
		return "", fmt.Errorf("failed to identify the read-write layer ID for container %q. - %v", containerID, err)
	}
	return string(bytes), err
}

// newDockerContainerHandler returns a new container.ContainerHandler
func newDockerContainerHandler(
	client *docker.Client,
	name string,
	machineInfoFactory v1.MachineInfoFactory,
	fsInfo fs.FsInfo,
	storageDriver storageDriver,
	storageDir string,
	cgroupSubsystems *libcontainer.CgroupSubsystems,
	inHostNamespace bool,
	metadataEnvs []string,
	dockerVersion []int,
	includedMetrics container.MetricSet,
	thinPoolName string,
) (container.ContainerHandler, error) {
	// Create the cgroup paths.
	cgroupPaths := common.MakeCgroupPaths(cgroupSubsystems.MountPoints, name)

	// Generate the equivalent cgroup manager for this container.
	cgroupManager, err := libcontainer.NewCgroupManager(name, cgroupPaths)
	if err != nil {
		return nil, err
	}

	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
		storageDir = path.Join(rootFs, storageDir)
	}

	id := ContainerNameToDockerId(name)
	// Add the Containers dir where the log files are stored.
	// FIXME: Give `otherStorageDir` a more descriptive name.
	//otherStorageDir := path.Join(storageDir, pathToContainersDir, id)
	rwLayerID, err := getRwLayerID(id, storageDir, storageDriver, dockerVersion)
	if err != nil {
		return nil, err
	}

	// Determine the rootfs storage dir OR the pool name to determine the device.
	// For devicemapper, we only need the thin pool name, and that is passed in to this call
	var (
		rootfsStorageDir string
		//zfsFilesystem    string
		zfsParent string
	)
	switch storageDriver {
	case overlay2StorageDriver:
		rootfsStorageDir = path.Join(storageDir, string(storageDriver), rwLayerID, overlay2RWLayer)
	}

	// We assume that if Inspect fails then the container is not known to docker.
	ctnr, err := client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %q: %v", id, err)
	}

	// TODO: extract object mother method
	handler := &dockerContainerHandler{
		machineInfoFactory: machineInfoFactory,
		cgroupPaths:        cgroupPaths,
		fsInfo:             fsInfo,
		storageDriver:      storageDriver,
		poolName:           thinPoolName,
		rootfsStorageDir:   rootfsStorageDir,
		envs:               make(map[string]string),
		labels:             ctnr.Config.Labels,
		includedMetrics:    includedMetrics,
		zfsParent:          zfsParent,
	}
	// Timestamp returned by Docker is in time.RFC3339Nano format.
	handler.creationTime, err = time.Parse(time.RFC3339Nano, ctnr.Created)
	if err != nil {
		// This should not happen, report the error just in case
		return nil, fmt.Errorf("failed to parse the create timestamp %q for container %q: %v", ctnr.Created, id, err)
	}
	handler.libcontainerHandler = libcontainer.NewHandler(cgroupManager, rootFs, ctnr.State.Pid, includedMetrics)

	// Add the name and bare ID as aliases of the container.
	handler.reference = v1.ContainerReference{
		Id:        id,
		Name:      name,
		Aliases:   []string{strings.TrimPrefix(ctnr.Name, "/"), id},
		Namespace: DockerNamespace,
	}
	handler.image = ctnr.Config.Image
	handler.networkMode = ctnr.HostConfig.NetworkMode
	// Only adds restartcount label if it's greater than 0
	if ctnr.RestartCount > 0 {
		handler.labels["restartcount"] = strconv.Itoa(ctnr.RestartCount)
	}

	// Obtain the IP address for the container.
	// If the NetworkMode starts with 'container:' then we need to use the IP address of the container specified.
	// This happens in cases such as kubernetes where the containers doesn't have an IP address itself and we need to use the pod's address
	ipAddress := ctnr.NetworkSettings.IPAddress
	networkMode := string(ctnr.HostConfig.NetworkMode)
	if ipAddress == "" && strings.HasPrefix(networkMode, "container:") {
		containerID := strings.TrimPrefix(networkMode, "container:")
		c, err := client.ContainerInspect(context.Background(), containerID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container %q: %v", id, err)
		}
		ipAddress = c.NetworkSettings.IPAddress
	}

	handler.ipAddress = ipAddress

	// INFO: ignore disk usage

	// split env vars to get metadata map.
	for _, exposedEnv := range metadataEnvs {
		if exposedEnv == "" {
			// if no dockerEnvWhitelist provided, len(metadataEnvs) == 1, metadataEnvs[0] == ""
			continue
		}

		for _, envVar := range ctnr.Config.Env {
			if envVar != "" {
				splits := strings.SplitN(envVar, "=", 2)
				if len(splits) == 2 && strings.HasPrefix(splits[0], exposedEnv) {
					handler.envs[strings.ToLower(splits[0])] = splits[1]
				}
			}
		}
	}

	return handler, nil
}
