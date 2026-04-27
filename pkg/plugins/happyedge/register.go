package happyedge

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	schedschemev1 "k8s.io/kube-scheduler/config/v1"
	schedconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   schedconfig.GroupName,
	Version: "v1",
}

var (
	localSchemeBuilder = &schedschemev1.SchemeBuilder
	AddToScheme        = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &HappyEdgeArgs{})
	return nil
}

func init() {
	localSchemeBuilder.Register(addKnownTypes)
}
