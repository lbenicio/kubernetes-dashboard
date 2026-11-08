// Copyright 2017 The Kubernetes Authors.
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
	for _, test := range []struct {
		name        string
		external    bool
		serviceType corev1.ServiceType
	}{
		{name: "cluster IP", serviceType: corev1.ServiceTypeClusterIP},
		{name: "load balancer", external: true, serviceType: corev1.ServiceTypeLoadBalancer},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertDeployAppConfiguration(t, test.external, test.serviceType)
		})
	}
}

func assertDeployAppConfiguration(t *testing.T, external bool, serviceType corev1.ServiceType) {
	t.Helper()
	description := "web frontend"
	pullSecret := "registry-secret"
	command := "/bin/server"
	commandArgs := "--port 8080 --verbose"
	cpu := resource.MustParse("250m")
	memory := resource.MustParse("64Mi")

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
	if len(actions) != 2 {
		t.Fatalf("client actions = %#v; want deployment then service creation", actions)
	}
	assertCreateActionResource(t, actions[0], "deployments")
	assertCreateActionResource(t, actions[1], "services")
	deploymentAction := actions[0].(clienttesting.CreateAction)
	serviceAction := actions[1].(clienttesting.CreateAction)
	assertConfiguredDeployment(t, deploymentAction, description, pullSecret, command, commandArgs, cpu, memory)
	assertConfiguredService(t, serviceAction, serviceType)
}

func assertCreateActionResource(t *testing.T, action clienttesting.Action, resource string) {
	t.Helper()
	if got := action.GetResource().Resource; got != resource {
		t.Fatalf("created resource = %q; want %q", got, resource)
	}
}

func assertConfiguredDeployment(
	t *testing.T,
	action clienttesting.CreateAction,
	description, pullSecret, command, commandArgs string,
	cpu, memory resource.Quantity,
) {
	t.Helper()
	deployed := action.GetObject().(*apps.Deployment)
	if deployed.Namespace != "" { // Namespace is carried by the create action, not object metadata.
		t.Errorf("deployment object namespace = %q; want unset", deployed.Namespace)
	}
	if action.GetNamespace() != "team-a" || deployed.Name != "web" || *deployed.Spec.Replicas != 3 {
		t.Errorf("deployment action/object = namespace %q, %#v", action.GetNamespace(), deployed)
	}
	assertDeploymentMetadata(t, deployed, description)
	container := deployed.Spec.Template.Spec.Containers[0]
	assertContainerCommand(t, container, command, commandArgs)
	assertContainerResources(t, container, cpu, memory)
	assertContainerSecretsAndEnvironment(t, deployed, container, pullSecret)
}

func assertDeploymentMetadata(t *testing.T, deployed *apps.Deployment, description string) {
	t.Helper()
	if deployed.Labels["app"] != "web" || deployed.Labels["team"] != "platform" {
		t.Errorf("deployment labels = %#v", deployed.Labels)
	}
	if got := deployed.Annotations[DescriptionAnnotationKey]; got != description {
		t.Errorf("deployment description annotation = %q; want %q", got, description)
	}
}

func assertContainerCommand(t *testing.T, container corev1.Container, command, commandArgs string) {
	t.Helper()
	if container.Image != "example/web:v1" || container.Command[0] != command || strings.Join(container.Args, " ") != commandArgs {
		t.Errorf("deployment container command/image = %#v", container)
	}
}

func assertContainerResources(t *testing.T, container corev1.Container, cpu, memory resource.Quantity) {
	t.Helper()
	cpuRequest := container.Resources.Requests[corev1.ResourceCPU]
	memoryRequest := container.Resources.Requests[corev1.ResourceMemory]
	if cpuRequest.Cmp(cpu) != 0 || memoryRequest.Cmp(memory) != 0 {
		t.Errorf("deployment resource requests = %#v", container.Resources.Requests)
	}
}

func assertContainerSecretsAndEnvironment(t *testing.T, deployed *apps.Deployment, container corev1.Container, pullSecret string) {
	t.Helper()
	imagePullSecrets := deployed.Spec.Template.Spec.ImagePullSecrets
	if len(imagePullSecrets) != 1 || imagePullSecrets[0].Name != pullSecret {
		t.Errorf("deployment image pull secrets = %#v", imagePullSecrets)
	}
	if len(container.Env) != 1 || container.Env[0] != (corev1.EnvVar{Name: "ENV", Value: "test"}) {
		t.Errorf("deployment container environment = %#v", container.Env)
	}
}

func assertConfiguredService(t *testing.T, action clienttesting.CreateAction, serviceType corev1.ServiceType) {
	t.Helper()
	service := action.GetObject().(*corev1.Service)
	if action.GetNamespace() != "team-a" {
		t.Errorf("service namespace = %q; want team-a", action.GetNamespace())
	}
	if service.Spec.Type != serviceType {
		t.Errorf("service type = %q; want %q", service.Spec.Type, serviceType)
	}
	if service.Spec.Selector["app"] != "web" {
		t.Errorf("service selector = %#v; want app=web", service.Spec.Selector)
	}
	if len(service.Spec.Ports) != 2 {
		t.Fatalf("service ports = %#v; want two ports", service.Spec.Ports)
	}
	assertServicePort(t, "first", service.Spec.Ports[0], 80, 8080, corev1.ProtocolTCP, "tcp-80-8080-")
	assertServicePort(t, "second", service.Spec.Ports[1], 53, 5353, corev1.ProtocolUDP, "udp-53-5353-")
}

func assertServicePort(t *testing.T, name string, port corev1.ServicePort, portNumber, targetPort int32, protocol corev1.Protocol, namePrefix string) {
	t.Helper()
	if port.Port != portNumber || port.TargetPort.IntVal != targetPort || port.Protocol != protocol || !strings.HasPrefix(port.Name, namePrefix) {
		t.Errorf("%s service port = %#v", name, port)
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
