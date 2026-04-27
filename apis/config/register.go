package config

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	schedconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   schedconfig.GroupName,
	Version: runtime.APIVersionInternal,
}

var (
	localSchemeBuilder = &schedconfig.SchemeBuilder
	AddToScheme        = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &HappyEdgeArgs{})
	return nil
}

func init() {
	localSchemeBuilder.Register(addKnownTypes)
}
