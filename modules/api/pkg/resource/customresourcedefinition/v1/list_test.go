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

package v1

import (
	"errors"
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/customresourcedefinition/types"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	commontypes "k8s.io/dashboard/types"
)

func TestGetCustomResourceDefinition(t *testing.T) {
	cases := []struct {
		expectedActions []string
		crdList         *apiextensionsv1.CustomResourceDefinitionList
		expected        *types.CustomResourceDefinitionList
	}{
		{
			[]string{"list"},
			&apiextensionsv1.CustomResourceDefinitionList{
				Items: []apiextensionsv1.CustomResourceDefinition{
					{
						ObjectMeta: metaV1.ObjectMeta{Name: "foos.samplecontroller.k8s.io"},
						Spec: apiextensionsv1.CustomResourceDefinitionSpec{
							Names: apiextensionsv1.CustomResourceDefinitionNames{
								Kind:   "Foo",
								Plural: "foos",
							},
							Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
								{
									Served: true,
									Name:   "v1alpha1",
								},
							},
						},
					},
				},
			},
			&types.CustomResourceDefinitionList{
				ListMeta: commontypes.ListMeta{TotalItems: 1},
				Items: []types.CustomResourceDefinition{
					{
						ObjectMeta:  commontypes.ObjectMeta{Name: "foos.samplecontroller.k8s.io"},
						TypeMeta:    commontypes.TypeMeta{Kind: commontypes.ResourceKindCustomResourceDefinition},
						Version:     "v1alpha1",
						Established: apiextensions.ConditionUnknown,
					},
				},
				Errors: []error{},
			},
		}, {
			[]string{"list"},
			&apiextensionsv1.CustomResourceDefinitionList{
				Items: []apiextensionsv1.CustomResourceDefinition{
					{
						ObjectMeta: metaV1.ObjectMeta{Name: "foos.samplecontroller.k8s.io"},
						Spec: apiextensionsv1.CustomResourceDefinitionSpec{
							Names: apiextensionsv1.CustomResourceDefinitionNames{
								Kind:   "Foo",
								Plural: "foos",
							},
							Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
								{
									Served: true,
									Name:   "v1alpha1",
								},
								{
									Served: false,
									Name:   "v1beta1",
								},
								{
									Served: true,
									Name:   "v1gamma1",
								},
								{
									Served: false,
									Name:   "v1zeta1",
								},
							},
						},
					},
				},
			},
			&types.CustomResourceDefinitionList{
				ListMeta: commontypes.ListMeta{TotalItems: 1},
				Items: []types.CustomResourceDefinition{
					{
						ObjectMeta:  commontypes.ObjectMeta{Name: "foos.samplecontroller.k8s.io"},
						TypeMeta:    commontypes.TypeMeta{Kind: commontypes.ResourceKindCustomResourceDefinition},
						Version:     "v1alpha1",
						Established: apiextensions.ConditionUnknown,
					},
				},
				Errors: []error{},
			},
		},
	}

	for _, c := range cases {
		fakeClient := fake.NewSimpleClientset(c.crdList)

		actual, _ := GetCustomResourceDefinitionList(fakeClient, dataselect.DefaultDataSelect)

		actions := fakeClient.Actions()
		if len(actions) != len(c.expectedActions) {
			t.Errorf("Unexpected actions: %v, expected %d actions got %d", actions,
				len(c.expectedActions), len(actions))
			continue
		}

		for i, verb := range c.expectedActions {
			if actions[i].GetVerb() != verb {
				t.Errorf("Unexpected action: %+v, expected %s",
					actions[i], verb)
			}
		}

		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("GetCustomResourceDefinitionList(client, nil) == \ngot: %#v, \nexpected %#v",
				actual, c.expected)
		}
	}
}

func TestGetCustomResourceDefinitionListRejectsAPIErrorAndSkipsUnservedDefinitions(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "customresourcedefinitions", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("list failed")
		})
		if _, err := GetCustomResourceDefinitionList(client, dataselect.DefaultDataSelect); err == nil {
			t.Fatal("GetCustomResourceDefinitionList() returned no error after a list failure")
		}
	})

	client := fake.NewSimpleClientset(&apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: "widgets.example.test"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group:    "example.test",
			Names:    apiextensionsv1.CustomResourceDefinitionNames{Plural: "widgets", Kind: "Widget"},
			Scope:    apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: false}},
		},
	})
	result, err := GetCustomResourceDefinitionList(client, dataselect.DefaultDataSelect)
	if err != nil {
		t.Fatalf("GetCustomResourceDefinitionList() error = %v", err)
	}
	if result.ListMeta.TotalItems != 0 || len(result.Items) != 0 {
		t.Fatalf("unserved CRD result = %#v; want empty", result)
	}
}
