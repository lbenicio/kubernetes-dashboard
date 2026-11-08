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

package client

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

type discoveryStub struct {
	discovery.DiscoveryInterface
	resources []*metav1.APIResourceList
	err       error
}

func (in discoveryStub) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, in.resources, in.err
}

func resetKindMapping(t *testing.T) {
	t.Helper()
	previous := kindToGroupVersionResource
	kindToGroupVersionResource = make(map[string]schema.GroupVersionResource)
	t.Cleanup(func() { kindToGroupVersionResource = previous })
}

func TestGroupVersionResourceFromKindUsesDiscoveryAndCache(t *testing.T) {
	resetKindMapping(t)
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod"},
				{Name: "pods/status", Kind: "Pod"},
			},
		},
		{
			GroupVersion: "widgets.example.test/v1",
			APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Group: "widgets.example.test"}},
		},
	}
	verber := &resourceVerber{discovery: discoveryStub{resources: resources}}

	for _, test := range []struct {
		kind string
		want schema.GroupVersionResource
	}{
		{kind: "pod", want: schema.GroupVersionResource{Version: "v1", Resource: "pods"}},
		{kind: "widgets.widgets.example.test", want: schema.GroupVersionResource{Group: "widgets.example.test", Version: "v1", Resource: "widgets"}},
	} {
		got, err := verber.groupVersionResourceFromKind(test.kind)
		if err != nil || got != test.want {
			t.Errorf("groupVersionResourceFromKind(%q) = %v, %v; want %v", test.kind, got, err, test.want)
		}
	}

	if _, err := verber.groupVersionResourceFromKind("missing"); err == nil || !strings.Contains(err.Error(), "could not find GVR") {
		t.Fatalf("missing kind error = %v", err)
	}
	verber.discovery = discoveryStub{err: context.DeadlineExceeded}
	if _, err := verber.groupVersionResourceFromKind("not-cached"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("discovery error = %v, want original error", err)
	}
	kindToGroupVersionResource["cached"] = testGVR
	if got, err := verber.groupVersionResourceFromKind("cached"); err != nil || got != testGVR {
		t.Fatalf("cached kind = %v, %v; want %v", got, err, testGVR)
	}
}

func TestBuildGroupVersionResourceCacheRejectsMalformedGroupVersion(t *testing.T) {
	resetKindMapping(t)
	verber := &resourceVerber{}
	err := verber.buildGroupVersionResourceCache([]*metav1.APIResourceList{{GroupVersion: "bad/group/version"}})
	if err == nil {
		t.Fatal("buildGroupVersionResourceCache() accepted malformed group version")
	}
}

func TestGroupVersionResourceFromKindReturnsDiscoveryCacheBuildErrors(t *testing.T) {
	resetKindMapping(t)
	verber := &resourceVerber{discovery: discoveryStub{resources: []*metav1.APIResourceList{{GroupVersion: "invalid/group/version"}}}}
	if _, err := verber.groupVersionResourceFromKind("pod"); err == nil {
		t.Fatal("groupVersionResourceFromKind() accepted malformed discovery metadata")
	}
}

func TestGroupVersionResourceFromUnstructuredAndDeletePolicies(t *testing.T) {
	verber := &resourceVerber{}
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	want := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if got := verber.groupVersionResourceFromUnstructured(resource); got != want {
		t.Fatalf("groupVersionResourceFromUnstructured() = %v, want %v", got, want)
	}

	for _, test := range []struct {
		input string
		want  metav1.DeletionPropagation
	}{
		{input: string(metav1.DeletePropagationBackground), want: metav1.DeletePropagationBackground},
		{input: string(metav1.DeletePropagationForeground), want: metav1.DeletePropagationForeground},
		{input: string(metav1.DeletePropagationOrphan), want: metav1.DeletePropagationOrphan},
		{input: "unknown", want: metav1.DeletePropagationForeground},
	} {
		if got := verber.toDeletePropagationPolicy(test.input); got != test.want {
			t.Errorf("toDeletePropagationPolicy(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestResourceVerberDeleteAndGetUseResolvedResource(t *testing.T) {
	resetKindMapping(t)
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	kindToGroupVersionResource["pod"] = podGVR
	resource := testUnstructuredResource("v1", "Pod", "testing", "one")
	fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)
	verber := &resourceVerber{client: fakeClient}

	got, err := verber.Get("pod", "testing", "one")
	if err != nil || got.(*unstructured.Unstructured).GetName() != "one" {
		t.Fatalf("Get() = %v, %v", got, err)
	}
	if err := verber.Delete("pod", "testing", "one", string(metav1.DeletePropagationBackground), true); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	deleteActions := fakeClient.Actions()
	var deleteAction k8stesting.DeleteAction
	for _, action := range deleteActions {
		if candidate, ok := action.(k8stesting.DeleteAction); ok {
			deleteAction = candidate
		}
	}
	if deleteAction == nil {
		t.Fatal("Delete() did not issue a Kubernetes delete action")
	}
	options := deleteAction.GetDeleteOptions()
	if options.PropagationPolicy == nil || *options.PropagationPolicy != metav1.DeletePropagationBackground ||
		options.GracePeriodSeconds == nil || *options.GracePeriodSeconds != 1 {
		t.Fatalf("Delete() options = %#v", options)
	}
}

func TestResourceVerberUpdateUsesMergePatchForCustomResources(t *testing.T) {
	for _, test := range []struct {
		name     string
		apiVer   string
		kind     string
		wantType k8stypes.PatchType
	}{
		{name: "custom resource", apiVer: "widgets.example.test/v1", kind: "Widget", wantType: "application/merge-patch+json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			object := testUnstructuredResource(test.apiVer, test.kind, "testing", "one")
			object.Object["spec"] = map[string]any{"version": "one"}
			modified := object.DeepCopy()
			modified.Object["spec"] = map[string]any{"version": "two"}
			fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), object)
			verber := &resourceVerber{client: fakeClient}
			if err := verber.Update(modified); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			var patchAction k8stesting.PatchAction
			for _, action := range fakeClient.Actions() {
				if candidate, ok := action.(k8stesting.PatchAction); ok {
					patchAction = candidate
				}
			}
			if patchAction == nil || patchAction.GetPatchType() != test.wantType {
				t.Fatalf("Update() patch action = %#v, want patch type %q", patchAction, test.wantType)
			}
		})
	}
}

func TestResourceVerberUpdateReturnsPatchErrors(t *testing.T) {
	resource := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
	resource.Object["spec"] = map[string]any{"version": "old"}
	modified := resource.DeepCopy()
	modified.Object["spec"] = map[string]any{"version": "new"}
	fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)
	fakeClient.PrependReactor("patch", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("patch denied")
	})
	verber := &resourceVerber{client: fakeClient}
	if err := verber.Update(modified); err == nil || err.Error() != "patch denied" {
		t.Fatalf("Update() patch error = %v, want patch denied", err)
	}
}

func TestResourceVerberUpdateUsesStrategicMergePatchForBuiltInResources(t *testing.T) {
	object := testUnstructuredResource("v1", "Pod", "testing", "one")
	object.Object["spec"] = map[string]any{"containers": []any{map[string]any{"name": "web", "image": "nginx:old"}}}
	modified := object.DeepCopy()
	modified.Object["spec"] = map[string]any{"containers": []any{map[string]any{"name": "web", "image": "nginx:new"}}}
	fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), object)
	fakeClient.PrependReactor("patch", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, modified.DeepCopy(), nil
	})
	verber := &resourceVerber{client: fakeClient}
	if err := verber.Update(modified); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	var patchAction k8stesting.PatchAction
	for _, action := range fakeClient.Actions() {
		if candidate, ok := action.(k8stesting.PatchAction); ok {
			patchAction = candidate
		}
	}
	if patchAction == nil || patchAction.GetPatchType() != k8stypes.StrategicMergePatchType {
		t.Fatalf("Update() patch action = %#v, want strategic merge patch", patchAction)
	}
}

func TestResourceVerberUpdateReportsInvalidStrategicMergePatch(t *testing.T) {
	resource := testUnstructuredResource("v1", "Pod", "testing", "one")
	resource.Object["spec"] = map[string]any{"containers": []any{map[string]any{"name": "web", "image": "nginx:old"}}}
	modified := resource.DeepCopy()
	modified.Object["spec"] = map[string]any{"containers": "not-a-container-list"}
	fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)
	verber := &resourceVerber{client: fakeClient}
	if err := verber.Update(modified); err == nil || !strings.Contains(err.Error(), "api field") {
		t.Fatalf("Update() malformed strategic patch error = %v; want a Kubernetes strategic-merge schema error", err)
	}
}

func TestResourceVerberUpdateReportsMarshalErrors(t *testing.T) {
	t.Run("latest resource", func(t *testing.T) {
		latest := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
		latest.Object["unencodable"] = func() {}
		modified := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
		verber := &resourceVerber{client: marshalErrorDynamicClient{latest: latest}}
		if err := verber.Update(modified); err == nil || !strings.Contains(err.Error(), "failed to marshal original data") {
			t.Fatalf("Update() latest marshal error = %v", err)
		}
	})

	t.Run("modified resource", func(t *testing.T) {
		latest := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
		modified := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
		modified.Object["unencodable"] = func() {}
		verber := &resourceVerber{client: marshalErrorDynamicClient{latest: latest}}
		if err := verber.Update(modified); err == nil || !strings.Contains(err.Error(), "failed to marshal modified data") {
			t.Fatalf("Update() modified marshal error = %v", err)
		}
	})
}

type marshalErrorDynamicClient struct {
	dynamic.Interface
	latest *unstructured.Unstructured
}

func (in marshalErrorDynamicClient) Resource(schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return marshalErrorResource{latest: in.latest}
}

type marshalErrorResource struct {
	dynamic.ResourceInterface
	latest *unstructured.Unstructured
}

func (in marshalErrorResource) Namespace(string) dynamic.ResourceInterface { return in }
func (in marshalErrorResource) Get(context.Context, string, metav1.GetOptions, ...string) (*unstructured.Unstructured, error) {
	return in.latest, nil
}

func TestResourceVerberReturnsDiscoveryFailuresForGetAndDelete(t *testing.T) {
	resetKindMapping(t)
	verber := &resourceVerber{
		client:    dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		discovery: discoveryStub{},
	}
	if _, err := verber.Get("missing", "testing", "one"); err == nil {
		t.Fatal("Get() succeeded for a kind absent from discovery")
	}
	if err := verber.Delete("missing", "testing", "one", "", false); err == nil {
		t.Fatal("Delete() succeeded for a kind absent from discovery")
	}
}

func TestResourceVerberUpdateSkipsUnchangedResourcesAndWrapsReadErrors(t *testing.T) {
	resource := testUnstructuredResource("widgets.example.test/v1", "Widget", "testing", "one")
	unchangedClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)
	unchangedVerber := &resourceVerber{client: unchangedClient}
	if err := unchangedVerber.Update(resource.DeepCopy()); err != nil {
		t.Fatalf("Update() on unchanged object error = %v", err)
	}
	for _, action := range unchangedClient.Actions() {
		if _, ok := action.(k8stesting.PatchAction); ok {
			t.Fatal("Update() patched an unchanged object")
		}
	}

	missingClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	missingVerber := &resourceVerber{client: missingClient}
	err := missingVerber.Update(resource.DeepCopy())
	if err == nil || !strings.Contains(err.Error(), "failed to get latest widgets version") {
		t.Fatalf("Update() missing object error = %v", err)
	}
}

func TestVerberClientRequiresConfigurationAndBuildsClients(t *testing.T) {
	previous := baseConfig
	baseConfig = nil
	t.Cleanup(func() { baseConfig = previous })
	anonymousFlag := pflag.Lookup("anonymous-admin")
	previousAnonymous := anonymousFlag.Value.String()
	if err := anonymousFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = anonymousFlag.Value.Set(previousAnonymous) })
	request := httptest.NewRequest("GET", "https://dashboard.example.test", nil)
	if _, err := VerberClient(request); err == nil {
		t.Fatal("VerberClient() succeeded without an authorization token")
	}

	baseConfig = &rest.Config{Host: "https://kubernetes.example.test"}
	SetAuthorizationHeader(request, "test-token")
	verber, err := VerberClient(request)
	if err != nil || verber == nil {
		t.Fatalf("VerberClient() = %v, %v", verber, err)
	}

	baseConfig = &rest.Config{Host: "%"}
	if _, err := VerberClient(request); err == nil {
		t.Fatal("VerberClient() accepted a malformed REST config")
	}
}

func testUnstructuredResource(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"namespace":       namespace,
			"name":            name,
			"resourceVersion": "1",
		},
	}}
}

var testGVR = schema.GroupVersionResource{Group: "widgets.example.test", Version: "v1", Resource: "widgets"}
