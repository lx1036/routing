package openapi

import (
	"k8s-lx1036/wayne/backend/client"
	"k8s-lx1036/wayne/backend/client/api"
	"k8s-lx1036/wayne/backend/models"
	"k8s-lx1036/wayne/backend/models/response"
	"k8s-lx1036/wayne/backend/resources/pod"
	"net/http"
	"time"
)

type PodListParam struct {
	Namespace string `json:"namespace"`
	Name string `json:"name"`
	Type api.ResourceName `json:"type"`
}

type respPodInfoList struct {
	Body struct {
		response.ResponseBase
		RespListInfo []*respListInfo `json:"list"`
	}
}
type respListInfo struct {
	Cluster      string `json:"cluster,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	// Wayne namespace 名称
	Namespace string        `json:"namespace,omitempty"`
	Pods      []respPodInfo `json:"pods"`
}
type respPodInfo struct {
	Name      string    `json:"name,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	NodeName  string    `json:"nodeName,omitempty"`
	PodIp     string    `json:"podIp,omitempty"`
	State     string    `json:"state,omitempty"`
	StartTime time.Time `json:"startTime,omitempty"`
}

// swagger:route GET /get_pod_list pod PodListParam
//
// 用于根据资源类型获取所有机房Pod列表
//
// 返回 Pod 信息
// 需要绑定全局 apikey 使用。
//
//     Responses:
//       200: respPodInfoList
//       401: responseState
//       500: responseState
// @router /get_pod_list [get]
func (controller *OpenAPIController) GetPodList() {
	if !controller.CheckoutRoutePermission(GetPodListAction) {
		return
	}
	if controller.APIKey.Type != models.GlobalAPIKey {

	}

	podList := respPodInfoList{}
	podList.Body.Code = http.StatusOK

	params := PodListParam{
		Namespace: controller.GetString("namespace"),
		Name:      controller.GetString("name"),
		Type:      controller.GetString("type"),
	}

	var err error
	var namespace *models.Namespace
	if params.Namespace != "" {
		namespace, err = models.NamespaceModel.GetByName(params.Namespace)
		if err != nil {

		}
	}

	managers := client.Managers()
	managers.Range(func(key, value interface{}) bool {
		manager := value.(*client.ClusterManager)
		// if Name and Namespace empty,return all pods
		if params.Name == "" && params.Namespace == "" {
			// return all pods
		}

		podListResp, err := buildPodListResp(manager, params.Namespace, namespace.KubeNamespace, params.Name, params.Type)
		if err != nil {

		}
		if len(podListResp.Pods) > 0 {
			podList.Body.RespListInfo = append(podList.Body.RespListInfo, podListResp)
		}

		return true
	})

	controller.HandleResponse(podList.Body)
}

func buildPodListResp(manager *client.ClusterManager, namespace, kubeNamespace, resourceName string, resourceType api.ResourceName) (*respListInfo, error) {
	pods, err := pod.GetPodListByType(manager.KubeClient, kubeNamespace, resourceName, resourceType)
	if err != nil {
		return nil, err
	}

	listInfo := &respListInfo{
		Cluster:      manager.Cluster.Name,
		ResourceName: resourceName,
		Namespace:    namespace,
	}

	for _, kubePod := range pods {
		listInfo.Pods = append(listInfo.Pods, respPodInfo{
			Name:      kubePod.Name,
			Namespace: kubePod.Namespace,
			NodeName:  kubePod.Spec.NodeName,
			PodIp:     kubePod.Status.PodIP,
			State:     pod.GetPodStatus(kubePod),
			StartTime: kubePod.CreationTimestamp.Time,
		})
	}

	return listInfo, nil
}