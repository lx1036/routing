package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilpointer "k8s.io/utils/pointer"
	"strings"
)

// INFO: 这里定义的函数，也会被 `make defaults-gen` 命令自动生成到 zz_generated.defaults.go 文件中

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_CustomResourceDefinition(obj *CustomResourceDefinition) {
	SetDefaults_CustomResourceDefinitionSpec(&obj.Spec)
	if len(obj.Status.StoredVersions) == 0 {
		for _, v := range obj.Spec.Versions {
			if v.Storage {
				obj.Status.StoredVersions = append(obj.Status.StoredVersions, v.Name)
				break
			}
		}
	}
}

func SetDefaults_CustomResourceDefinitionSpec(obj *CustomResourceDefinitionSpec) {
	if len(obj.Names.Singular) == 0 {
		obj.Names.Singular = strings.ToLower(obj.Names.Kind)
	}
	if len(obj.Names.ListKind) == 0 && len(obj.Names.Kind) > 0 {
		obj.Names.ListKind = obj.Names.Kind + "List"
	}
	if obj.Conversion == nil {
		obj.Conversion = &CustomResourceConversion{
			Strategy: NoneConverter,
		}
	}
}

// SetDefaults_ServiceReference sets defaults for Webhook's ServiceReference
func SetDefaults_ServiceReference(obj *ServiceReference) {
	if obj.Port == nil {
		obj.Port = utilpointer.Int32Ptr(443)
	}
}
