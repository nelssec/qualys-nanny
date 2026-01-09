/*
Copyright 2026.

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

package platform

import (
	"context"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	isOpenShift     *bool
	detectionMutex  sync.RWMutex
	dynamicClient   dynamic.Interface
	kubeClient      kubernetes.Interface
	clientInitMutex sync.Mutex
)

var SCCGVR = schema.GroupVersionResource{
	Group:    "security.openshift.io",
	Version:  "v1",
	Resource: "securitycontextconstraints",
}

func IsOpenShift(ctx context.Context) bool {
	detectionMutex.RLock()
	if isOpenShift != nil {
		result := *isOpenShift
		detectionMutex.RUnlock()
		return result
	}
	detectionMutex.RUnlock()

	detectionMutex.Lock()
	defer detectionMutex.Unlock()

	if isOpenShift != nil {
		return *isOpenShift
	}

	result := detectOpenShift(ctx)
	isOpenShift = &result
	return result
}

func detectOpenShift(ctx context.Context) bool {
	client, err := getDynamicClient()
	if err != nil {
		return false
	}

	_, err = client.Resource(SCCGVR).List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

func getDynamicClient() (dynamic.Interface, error) {
	clientInitMutex.Lock()
	defer clientInitMutex.Unlock()

	if dynamicClient != nil {
		return dynamicClient, nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	dynamicClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return dynamicClient, nil
}

func ResetCache() {
	detectionMutex.Lock()
	defer detectionMutex.Unlock()
	isOpenShift = nil
}

type ContainerRuntime string

const (
	RuntimeContainerd ContainerRuntime = "containerd"
	RuntimeCRIO       ContainerRuntime = "cri-o"
	RuntimeDocker     ContainerRuntime = "docker"
	RuntimeUnknown    ContainerRuntime = "unknown"
)

func DetectContainerRuntime(nodeContainerRuntimeVersion string) ContainerRuntime {
	if len(nodeContainerRuntimeVersion) == 0 {
		return RuntimeUnknown
	}

	for i, c := range nodeContainerRuntimeVersion {
		if c == ':' {
			runtimeName := nodeContainerRuntimeVersion[:i]
			switch runtimeName {
			case "containerd":
				return RuntimeContainerd
			case "cri-o":
				return RuntimeCRIO
			case "docker":
				return RuntimeDocker
			default:
				return RuntimeUnknown
			}
		}
	}

	return RuntimeUnknown
}

type OSType string

const (
	OSTypeCoreOS  OSType = "coreos"
	OSTypeRHEL    OSType = "rhel"
	OSTypeDebian  OSType = "debian"
	OSTypeUnknown OSType = "unknown"
)

type NodeOSInfo struct {
	OSType           OSType
	OSImage          string
	KernelVersion    string
	ContainerRuntime ContainerRuntime
	Architecture     string
}

func DetectNodeOS(node *corev1.Node) NodeOSInfo {
	info := NodeOSInfo{
		OSType:           OSTypeUnknown,
		OSImage:          node.Status.NodeInfo.OSImage,
		KernelVersion:    node.Status.NodeInfo.KernelVersion,
		ContainerRuntime: DetectContainerRuntime(node.Status.NodeInfo.ContainerRuntimeVersion),
		Architecture:     node.Status.NodeInfo.Architecture,
	}

	osImage := strings.ToLower(node.Status.NodeInfo.OSImage)

	if strings.Contains(osImage, "coreos") ||
		strings.Contains(osImage, "rhcos") ||
		strings.Contains(osImage, "fedora coreos") ||
		strings.Contains(osImage, "flatcar") {
		info.OSType = OSTypeCoreOS
		return info
	}

	if strings.Contains(osImage, "red hat enterprise linux") ||
		strings.Contains(osImage, "rhel") ||
		strings.Contains(osImage, "centos") ||
		strings.Contains(osImage, "rocky") ||
		strings.Contains(osImage, "alma") ||
		strings.Contains(osImage, "oracle linux") ||
		strings.Contains(osImage, "amazon linux") {
		info.OSType = OSTypeRHEL
		return info
	}

	if strings.Contains(osImage, "debian") ||
		strings.Contains(osImage, "ubuntu") {
		info.OSType = OSTypeDebian
		return info
	}

	return info
}

type ClusterOSProfile struct {
	HasCoreOSNodes bool
	HasRHELNodes   bool
	HasDebianNodes bool
	HasMixedOS     bool
	PrimaryOS      OSType
	NodeCount      map[OSType]int
}

func DetectClusterOSProfile(ctx context.Context) (*ClusterOSProfile, error) {
	client, err := getKubeClient()
	if err != nil {
		return nil, err
	}

	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	profile := &ClusterOSProfile{
		NodeCount: make(map[OSType]int),
	}

	for i := range nodes.Items {
		nodeInfo := DetectNodeOS(&nodes.Items[i])
		profile.NodeCount[nodeInfo.OSType]++
	}

	maxCount := 0
	osTypes := 0
	for osType, count := range profile.NodeCount {
		if count > 0 {
			osTypes++
			if count > maxCount {
				maxCount = count
				profile.PrimaryOS = osType
			}
			switch osType {
			case OSTypeCoreOS:
				profile.HasCoreOSNodes = true
			case OSTypeRHEL:
				profile.HasRHELNodes = true
			case OSTypeDebian:
				profile.HasDebianNodes = true
			}
		}
	}

	profile.HasMixedOS = osTypes > 1

	return profile, nil
}

func IsCoreOSNode(node *corev1.Node) bool {
	return DetectNodeOS(node).OSType == OSTypeCoreOS
}

func getKubeClient() (kubernetes.Interface, error) {
	clientInitMutex.Lock()
	defer clientInitMutex.Unlock()

	if kubeClient != nil {
		return kubeClient, nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return kubeClient, nil
}
