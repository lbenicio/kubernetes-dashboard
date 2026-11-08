// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pod

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/controller"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	"k8s.io/dashboard/api/pkg/resource/persistentvolumeclaim"
	"k8s.io/dashboard/types"
)

func TestGetPodDetail(t *testing.T) {
	cases := []struct {
		pod      *v1.PodList
		expected *PodDetail
	}{
		{
			pod: &v1.PodList{Items: []v1.Pod{{
				ObjectMeta: metaV1.ObjectMeta{
					Name: "test-pod", Namespace: "test-namespace",
					Labels: map[string]string{"app": "test"},
				}}}},
			expected: &PodDetail{
				TypeMeta: types.TypeMeta{Kind: types.ResourceKindPod},
				ObjectMeta: types.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Labels:    map[string]string{"app": "test"},
				},
				Controller:     &controller.ResourceOwner{},
				Containers:     []Container{},
				InitContainers: []Container{},
				EventList: common.EventList{
					Events: []common.Event{},
					Errors: []error{},
				},
				Metrics:                   []metricapi.Metric{},
				PersistentvolumeclaimList: persistentvolumeclaim.PersistentVolumeClaimList{},
				Errors:                    []error{},
			},
		},
	}

	for _, c := range cases {
		fakeClient := fake.NewSimpleClientset(c.pod)

		dataselect.DefaultDataSelectWithMetrics.MetricQuery = dataselect.NoMetrics
		actual, err := GetPodDetail(fakeClient, nil, "test-namespace", "test-pod")

		if err != nil {
			t.Errorf("GetPodDetail(%#v) == \ngot err %#v", c.pod, err)
		}
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("GetPodDetail(%#v) == \ngot %#v, \nexpected %#v", c.pod, actual, c.expected)
		}
	}
}

func TestGetPodDetailReturnsCriticalRetrievalErrors(t *testing.T) {
	getError := errors.New("pod lookup failed")
	listError := errors.New("resource list failed")
	tests := []struct {
		name     string
		verb     string
		resource string
		err      error
	}{
		{name: "pod get", verb: "get", resource: "pods", err: getError},
		{name: "controller pod list", verb: "list", resource: "pods", err: listError},
		{name: "configmap list", verb: "list", resource: "configmaps", err: listError},
		{name: "secret list", verb: "list", resource: "secrets", err: listError},
		{name: "event list", verb: "list", resource: "events", err: listError},
		{name: "persistent volume claim list", verb: "list", resource: "persistentvolumeclaims", err: listError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(&v1.Pod{
				ObjectMeta: metaV1.ObjectMeta{Name: "web", Namespace: "default"},
				Spec: v1.PodSpec{Volumes: []v1.Volume{{
					Name: "data",
					VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "data-claim",
					}},
				}}},
			})
			client.PrependReactor(test.verb, test.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				if test.verb == "get" {
					return true, nil, test.err
				}
				switch test.resource {
				case "pods":
					return true, &v1.PodList{}, test.err
				case "configmaps":
					return true, &v1.ConfigMapList{}, test.err
				case "secrets":
					return true, &v1.SecretList{}, test.err
				case "events":
					return true, &v1.EventList{}, test.err
				case "persistentvolumeclaims":
					return true, &v1.PersistentVolumeClaimList{}, test.err
				default:
					return false, nil, nil
				}
			})
			if result, err := GetPodDetail(client, nil, "default", "web"); err == nil || result != nil {
				t.Fatalf("GetPodDetail() = (%#v, %v), want a critical retrieval error", result, err)
			}
		})
	}
}

func TestEvalValueFrom(t *testing.T) {
	cases := []struct {
		src        *v1.EnvVarSource
		container  *v1.Container
		pod        *v1.Pod
		configMaps *v1.ConfigMapList
		secrets    *v1.SecretList
		expected   string
	}{
		{
			src: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "secret-env",
					},
					Key: "username",
				},
			},
			container:  nil,
			pod:        nil,
			configMaps: nil,
			secrets: &v1.SecretList{
				Items: []v1.Secret{
					{
						ObjectMeta: metaV1.ObjectMeta{
							Name: "secret-env",
						},
						Data: map[string][]byte{
							"username": []byte("top-secret"),
						},
					},
				},
			},
			expected: base64.StdEncoding.EncodeToString([]byte("top-secret")),
		},
		{
			src: &v1.EnvVarSource{
				ConfigMapKeyRef: &v1.ConfigMapKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "config-map-env",
					},
					Key: "username",
				},
			},
			container: nil,
			pod:       nil,
			configMaps: &v1.ConfigMapList{
				Items: []v1.ConfigMap{
					{
						ObjectMeta: metaV1.ObjectMeta{
							Name: "config-map-env",
						},
						Data: map[string]string{
							"username": "joey",
						},
					},
				},
			},
			secrets:  nil,
			expected: "joey",
		},
	}

	for _, c := range cases {
		dataselect.DefaultDataSelectWithMetrics.MetricQuery = dataselect.NoMetrics
		actual := evalValueFrom(c.src, c.container, c.pod, c.configMaps, c.secrets)
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("GetPodDetail(%#v, %#v, %#v, %#v, %#v) == \ngot %#v, \nexpected %#v",
				c.src, c.container, c.pod, c.configMaps, c.secrets, actual, c.expected)
		}
	}
}

func TestEvalEnvFrom(t *testing.T) {
	cases := []struct {
		container  v1.Container
		configMaps *v1.ConfigMapList
		secrets    *v1.SecretList
		expected   []EnvVar
	}{
		{
			container: v1.Container{
				Name:  "echoserver",
				Image: "registry.k8s.io/echoserver",
				EnvFrom: []v1.EnvFromSource{
					{
						SecretRef: &v1.SecretEnvSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "secret-env",
							},
						},
					}, {
						Prefix: "test_",
						ConfigMapRef: &v1.ConfigMapEnvSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "config-map-env",
							},
						},
					},
				},
			},
			configMaps: &v1.ConfigMapList{
				Items: []v1.ConfigMap{
					{
						ObjectMeta: metaV1.ObjectMeta{
							Name: "config-map-env",
						},
						Data: map[string]string{
							"username": "joey",
						},
					},
				},
			},
			secrets: &v1.SecretList{
				Items: []v1.Secret{
					{
						ObjectMeta: metaV1.ObjectMeta{
							Name: "secret-env",
						},
						Data: map[string][]byte{
							"username": []byte("top-secret"),
						},
					},
				},
			},
			expected: []EnvVar{
				{
					Name:  "username",
					Value: base64.StdEncoding.EncodeToString([]byte("top-secret")),
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "secret-env",
							},
							Key: "username",
						},
					},
				},
				{
					Name:  "test_username",
					Value: "joey",
					ValueFrom: &v1.EnvVarSource{
						ConfigMapKeyRef: &v1.ConfigMapKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "config-map-env",
							},
							Key: "username",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		dataselect.DefaultDataSelectWithMetrics.MetricQuery = dataselect.NoMetrics
		actual := evalEnvFrom(c.container, c.configMaps, c.secrets)
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("evalEnvFrom(%#v, %#v, %#v) == \ngot %#v, \nexpected %#v",
				c.container, c.configMaps, c.secrets, actual, c.expected)
		}
	}
}
