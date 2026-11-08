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

package v1

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/customresourcedefinition/types"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

type crdTestRoundTripper func(*http.Request) (*http.Response, error)

func (in crdTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func servedTestCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.test"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.test", Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: "widgets", Kind: "Widget"},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: "v1", Served: true, Storage: true,
				Subresources: &apiextensionsv1.CustomResourceSubresources{
					Scale:  &apiextensionsv1.CustomResourceSubresourceScale{},
					Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
				},
			}},
		},
	}
}

func crdObjectTestClients(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition, responseBody string) (*apiextensionsfake.Clientset, *rest.Config, *[]string) {
	t.Helper()
	paths := make([]string, 0, 1)
	config := &rest.Config{
		Host: "https://kubernetes.test",
		Transport: crdTestRoundTripper(func(request *http.Request) (*http.Response, error) {
			paths = append(paths, request.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Request:    request,
			}, nil
		}),
	}
	return apiextensionsfake.NewSimpleClientset(crd), config, &paths
}

func TestGetCustomResourceObjectListUsesRESTClientAndMapsObjects(t *testing.T) {
	client, config, paths := crdObjectTestClients(t, servedTestCRD(), `{"apiVersion":"example.test/v1","kind":"WidgetList","items":[{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"first","namespace":"default"}},{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"second","namespace":"default"}}]}`)
	got, err := GetCustomResourceObjectList(client, config, common.NewSameNamespaceQuery("default"), dataselect.DefaultDataSelect, "widgets.example.test")
	if err != nil {
		t.Fatalf("GetCustomResourceObjectList() error = %v", err)
	}
	if got.ListMeta.TotalItems != 2 || len(got.Items) != 2 {
		t.Fatalf("list meta/items = %#v / %#v; want two objects", got.ListMeta, got.Items)
	}
	if got.Items[0].TypeMeta.Kind != "widgets.example.test" || !got.Items[0].TypeMeta.Scalable {
		t.Errorf("mapped object metadata = %#v; want CRD kind and scalable flag", got.Items[0].TypeMeta)
	}
	if len(*paths) != 1 || (*paths)[0] != "/apis/example.test/v1/namespaces/default/widgets" {
		t.Errorf("REST paths = %v; want namespaced custom resource list path", *paths)
	}
}

func TestGetCustomResourceObjectDetailUsesRESTClientAndMapsObject(t *testing.T) {
	client, config, paths := crdObjectTestClients(t, servedTestCRD(), `{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"first","namespace":"default"}}`)
	got, err := GetCustomResourceObjectDetail(client, common.NewSameNamespaceQuery("default"), config, "widgets.example.test", "first")
	if err != nil {
		t.Fatalf("GetCustomResourceObjectDetail() error = %v", err)
	}
	if got.ObjectMeta.Name != "first" || got.TypeMeta.Kind != "widgets.example.test" || !got.TypeMeta.Scalable {
		t.Errorf("object detail = %#v; want mapped CRD metadata", got.CustomResourceObject)
	}
	if len(*paths) != 1 || (*paths)[0] != "/apis/example.test/v1/namespaces/default/widgets/first" {
		t.Errorf("REST paths = %v; want namespaced custom resource detail path", *paths)
	}
}

func TestGetCustomResourceDefinitionDetailCollectsVersionsConditionsAndObjects(t *testing.T) {
	crd := servedTestCRD()
	crd.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue}}
	client, config, _ := crdObjectTestClients(t, crd, `{"apiVersion":"example.test/v1","kind":"WidgetList","items":[]}`)
	got, err := GetCustomResourceDefinitionDetail(client, config, crd.Name)
	if err != nil {
		t.Fatalf("GetCustomResourceDefinitionDetail() error = %v", err)
	}
	if len(got.Versions) != 1 || got.Versions[0].Name != "v1" || len(got.Conditions) != 1 || got.Conditions[0].Status != "True" {
		t.Errorf("detail versions/conditions = %#v / %#v", got.Versions, got.Conditions)
	}
	if len(got.Subresources) != 2 || got.Subresources[0] != "Scale" || got.Subresources[1] != "Status" || got.Objects.ListMeta.TotalItems != 0 {
		t.Errorf("detail subresources/objects = %#v / %#v", got.Subresources, got.Objects)
	}
}

func TestCustomResourceObjectReadsReturnCRDAndRESTErrors(t *testing.T) {
	crd := servedTestCRD()
	client, _, _ := crdObjectTestClients(t, crd, `{}`)
	wantErr := errors.New("custom resource request failed")
	client.PrependReactor("get", "customresourcedefinitions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetCustomResourceObjectList(client, nil, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect, crd.Name); !errors.Is(err, wantErr) {
		t.Fatalf("GetCustomResourceObjectList() CRD error = %v, want %v", err, wantErr)
	}
	if _, err := GetCustomResourceObjectDetail(client, common.NewSameNamespaceQuery("default"), nil, crd.Name, "sample"); !errors.Is(err, wantErr) {
		t.Fatalf("GetCustomResourceObjectDetail() CRD error = %v, want %v", err, wantErr)
	}

	for _, test := range []struct {
		name   string
		config *rest.Config
	}{
		{name: "list REST client configuration", config: &rest.Config{Host: "%"}},
		{name: "detail REST client configuration", config: &rest.Config{Host: "%"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _, _ := crdObjectTestClients(t, crd, `{}`)
			var err error
			if strings.Contains(test.name, "list") {
				_, err = GetCustomResourceObjectList(client, test.config, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect, crd.Name)
			} else {
				_, err = GetCustomResourceObjectDetail(client, common.NewSameNamespaceQuery("default"), test.config, crd.Name, "sample")
			}
			if err == nil {
				t.Fatal("custom resource read unexpectedly succeeded with an invalid REST config")
			}
		})
	}

	for _, test := range []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "list HTTP failure", statusCode: http.StatusInternalServerError, body: `{"kind":"Status","status":"Failure","message":"unavailable","code":500}`},
		{name: "list malformed response", statusCode: http.StatusOK, body: `{`},
		{name: "detail HTTP failure", statusCode: http.StatusInternalServerError, body: `{"kind":"Status","status":"Failure","message":"unavailable","code":500}`},
		{name: "detail malformed response", statusCode: http.StatusOK, body: `{`},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := apiextensionsfake.NewSimpleClientset(crd)
			config := &rest.Config{
				Host: "https://kubernetes.test",
				Transport: crdTestRoundTripper(func(request *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: test.statusCode, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(test.body)), Request: request}, nil
				}),
			}
			var err error
			if strings.HasPrefix(test.name, "list") {
				_, err = GetCustomResourceObjectList(client, config, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect, crd.Name)
			} else {
				_, err = GetCustomResourceObjectDetail(client, common.NewSameNamespaceQuery("default"), config, crd.Name, "sample")
			}
			if err == nil {
				t.Fatal("custom resource read unexpectedly succeeded with an invalid response")
			}
		})
	}
}

func TestGetCustomResourceDefinitionDetailReturnsCRDAndObjectErrors(t *testing.T) {
	crd := servedTestCRD()
	wantErr := errors.New("custom resource definition lookup failed")
	client, config, _ := crdObjectTestClients(t, crd, `{"apiVersion":"example.test/v1","kind":"WidgetList","items":[]}`)
	client.PrependReactor("get", "customresourcedefinitions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetCustomResourceDefinitionDetail(client, config, crd.Name); !errors.Is(err, wantErr) {
		t.Fatalf("GetCustomResourceDefinitionDetail() error = %v; want %v", err, wantErr)
	}

	client, _, _ = crdObjectTestClients(t, crd, `{}`)
	if _, err := GetCustomResourceDefinitionDetail(client, &rest.Config{}, crd.Name); err == nil {
		t.Fatal("GetCustomResourceDefinitionDetail() succeeded when the custom resource REST client failed")
	}
}

func TestCustomResourceObjectHelpersRejectMissingAndUnservedCRDs(t *testing.T) {
	missingClient := apiextensionsfake.NewSimpleClientset()
	if _, err := GetCustomResourceObjectList(missingClient, &rest.Config{}, common.NewNamespaceQuery(nil), dataselect.DefaultDataSelect, "missing.example.test"); err == nil {
		t.Error("GetCustomResourceObjectList(missing CRD) error = nil")
	}
	if _, err := GetCustomResourceObjectDetail(missingClient, common.NewNamespaceQuery(nil), &rest.Config{}, "missing.example.test", "one"); err == nil {
		t.Error("GetCustomResourceObjectDetail(missing CRD) error = nil")
	}

	unserved := servedTestCRD()
	unserved.Spec.Versions[0].Served = false
	client := apiextensionsfake.NewSimpleClientset(unserved)
	if _, err := GetCustomResourceObjectList(client, &rest.Config{}, common.NewNamespaceQuery(nil), dataselect.DefaultDataSelect, unserved.Name); err == nil {
		t.Error("GetCustomResourceObjectList(unserved CRD) error = nil")
	}
}

func TestCustomResourceObjectMappingWithoutScaleSubresource(t *testing.T) {
	crd := servedTestCRD()
	crd.Spec.Versions[0].Subresources = nil
	object := types.CustomResourceObject{}
	toCRDObject(&object, crd)
	if string(object.TypeMeta.Kind) != crd.Name || object.TypeMeta.Scalable {
		t.Errorf("mapped type metadata = %#v; want kind %q and not scalable", object.TypeMeta, crd.Name)
	}
}
