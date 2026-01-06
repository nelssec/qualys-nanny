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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	isOpenShift     *bool
	detectionMutex  sync.RWMutex
	dynamicClient   dynamic.Interface
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
