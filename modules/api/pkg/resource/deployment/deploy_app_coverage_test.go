// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployment

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDeployAppCreatesConfiguredDeploymentAndService(t *testing.T) {
	description := "web frontend"
	pullSecret := "registry-secret"
	command := "/bin/server"
	commandArgs := "--port 8080 --verbose"
	cpu := resource.MustParse("250m")
	memory := resource.MustParse("64Mi")

	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "cluster IP", true: "load balancer"}[external], func(t *testing.T) {
			client := fake.NewSimpleClientset()
			spec := &AppDeploymentSpec{
				Name: "web", ContainerImage: "example/web:v1", Namespace: "team-a", Replicas: 3,
				RunAsPrivileged: true, IsExternal: external, Description: &description, ImagePullSecret: &pullSecret,
				ContainerCommand: &command, ContainerCommandArgs: &commandArgs,
				CpuRequirement: &cpu, MemoryRequirement: &memory,
				Variables:    []EnvironmentVariable{{Name: "ENV", Value: "test"}},
				Labels:       []Label{{Key: "app", Value: "old"}, {Key: "app", Value: "web"}, {Key: "team", Value: "platform"}},
				PortMappings: []PortMapping{{Port: 80, TargetPort: 8080, Protocol: corev1.ProtocolTCP}, {Port: 53, TargetPort: 5353, Protocol: corev1.ProtocolUDP}},
			}
			if err := DeployApp(spec, client); err != nil {
				t.Fatalf("DeployApp() error = %v", err)
			}

			actions := client.Actions()
			if len(actions) != 2 || actions[0].GetResource().Resource != "deployments" || actions[1].GetResource().Resource != "services" {
				t.Fatalf("client actions = %#v; want deployment then service creation", actions)
			}
			deployed := actions[0].(clienttesting.CreateAction).GetObject().(*apps.Deployment)
			if deployed.Namespace != "" { // Namespace is carried by the create action, not object metadata.
				t.Errorf("deployment object namespace = %q; want unset", deployed.Namespace)
			}
			if actions[0].GetNamespace() != "team-a" || deployed.Name != "web" || *deployed.Spec.Replicas != 3 {
				t.Errorf("deployment action/object = namespace %q, %#v", actions[0].GetNamespace(), deployed)
			}
			if deployed.Labels["app"] != "web" || deployed.Labels["team"] != "platform" || deployed.Annotations[DescriptionAnnotationKey] != description {
				t.Errorf("deployment metadata = %#v", deployed.ObjectMeta)
			}
			container := deployed.Spec.Template.Spec.Containers[0]
			if container.Image != "example/web:v1" || container.Command[0] != command || strings.Join(container.Args, " ") != commandArgs {
				t.Errorf("deployment container command/image = %#v", container)
			}
			cpuRequest := container.Resources.Requests[corev1.ResourceCPU]
			memoryRequest := container.Resources.Requests[corev1.ResourceMemory]
			if cpuRequest.Cmp(cpu) != 0 || memoryRequest.Cmp(memory) != 0 {
				t.Errorf("deployment resource requests = %#v", container.Resources.Requests)
			}
			if len(deployed.Spec.Template.Spec.ImagePullSecrets) != 1 || deployed.Spec.Template.Spec.ImagePullSecrets[0].Name != pullSecret || len(container.Env) != 1 || container.Env[0] != (corev1.EnvVar{Name: "ENV", Value: "test"}) {
				t.Errorf("deployment pod configuration = %#v", deployed.Spec.Template.Spec)
			}

			service := actions[1].(clienttesting.CreateAction).GetObject().(*corev1.Service)
			wantType := corev1.ServiceTypeClusterIP
			if external {
				wantType = corev1.ServiceTypeLoadBalancer
			}
			if actions[1].GetNamespace() != "team-a" || service.Spec.Type != wantType || service.Spec.Selector["app"] != "web" || len(service.Spec.Ports) != 2 {
				t.Errorf("service = %#v, namespace %q; want type %s and two ports", service, actions[1].GetNamespace(), wantType)
			}
			if service.Spec.Ports[0].Port != 80 || service.Spec.Ports[0].TargetPort.IntVal != 8080 || service.Spec.Ports[0].Protocol != corev1.ProtocolTCP || !strings.HasPrefix(service.Spec.Ports[0].Name, "tcp-80-8080-") {
				t.Errorf("first service port = %#v", service.Spec.Ports[0])
			}
			if service.Spec.Ports[1].Port != 53 || service.Spec.Ports[1].TargetPort.IntVal != 5353 || service.Spec.Ports[1].Protocol != corev1.ProtocolUDP || !strings.HasPrefix(service.Spec.Ports[1].Name, "udp-53-5353-") {
				t.Errorf("second service port = %#v", service.Spec.Ports[1])
			}
		})
	}
}

func TestDeployAppPropagatesCreateErrors(t *testing.T) {
	for _, test := range []struct {
		name        string
		resource    string
		wantActions int
	}{
		{name: "deployment create", resource: "deployments", wantActions: 1},
		{name: "service create", resource: "services", wantActions: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("create", test.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("creation rejected")
			})
			err := DeployApp(&AppDeploymentSpec{
				Name: "web", Namespace: "team-a",
				PortMappings: []PortMapping{{Port: 80, TargetPort: 8080, Protocol: corev1.ProtocolTCP}},
			}, client)
			if err == nil || err.Error() != "creation rejected" {
				t.Fatalf("DeployApp() error = %v; want creation rejection", err)
			}
			if got := len(client.Actions()); got != test.wantActions {
				t.Errorf("client actions = %d; want %d", got, test.wantActions)
			}
		})
	}
}

func TestGeneratePortMappingNameTruncatesLongPrefix(t *testing.T) {
	name := generatePortMappingName(PortMapping{Protocol: corev1.Protocol(strings.Repeat("x", 100))})
	if len(name) != 63 || !strings.HasPrefix(name, strings.Repeat("x", 58)) {
		t.Errorf("generatePortMappingName() = %q (length %d); want a 63-character bounded name", name, len(name))
	}
}

func TestGetLabelsMapUsesLastDuplicate(t *testing.T) {
	labels := getLabelsMap([]Label{{Key: "app", Value: "old"}, {Key: "app", Value: "new"}, {Key: "team", Value: "platform"}})
	want := map[string]string{"app": "new", "team": "platform"}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("getLabelsMap() = %#v; want %#v", labels, want)
	}
}
