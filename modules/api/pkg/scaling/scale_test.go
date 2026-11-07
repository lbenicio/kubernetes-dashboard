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

package scaling

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type scaleRoundTripper struct {
	failGet    bool
	failUpdate bool
	updated    int32
}

func (in *scaleRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	status := http.StatusOK
	body := "{}"
	switch request.URL.Path {
	case "/apis":
		body = `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"apps","versions":[{"groupVersion":"apps/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apps/v1","version":"v1"}}]}`
	case "/apis/apps/v1":
		body = `{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"apps/v1","resources":[{"name":"deployments","singularName":"deployment","namespaced":true,"kind":"Deployment","verbs":["get","list"]},{"name":"deployments/scale","singularName":"","namespaced":true,"kind":"Scale","group":"autoscaling","version":"v1","verbs":["get","update","patch"]}]}`
	case "/apis/apps/v1/namespaces/default/deployments/web/scale":
		if request.Method == http.MethodPut {
			if in.failUpdate {
				status = http.StatusInternalServerError
				body = `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"scale update failed","reason":"InternalError","code":500}`
				break
			}
			var updated struct {
				Spec struct {
					Replicas int32 `json:"replicas"`
				} `json:"spec"`
			}
			if err := json.NewDecoder(request.Body).Decode(&updated); err != nil {
				return nil, err
			}
			in.updated = updated.Spec.Replicas
			body = `{"kind":"Scale","apiVersion":"autoscaling/v1","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":` + strconv.FormatInt(int64(in.updated), 10) + `},"status":{"replicas":2}}`
		} else if in.failGet {
			status = http.StatusInternalServerError
			body = `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"scale get failed","reason":"InternalError","code":500}`
		} else {
			body = `{"kind":"Scale","apiVersion":"autoscaling/v1","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":3},"status":{"replicas":2}}`
		}
	default:
		status = http.StatusNotFound
		body = `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"unexpected path ` + request.URL.Path + `","reason":"NotFound","code":404}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

func TestScaleOperationsReturnConfigurationErrors(t *testing.T) {
	badConfig := &rest.Config{Host: "%"}
	if _, err := GetReplicaCounts(badConfig, "deployments", "default", "web"); err == nil {
		t.Fatal("GetReplicaCounts() accepted an empty API server config")
	}
	if _, err := ScaleResource(badConfig, "deployments", "default", "web", "3"); err == nil {
		t.Fatal("ScaleResource() accepted an empty API server config")
	}
}

func TestGetGroupResourceSupportsCoreAndQualifiedResources(t *testing.T) {
	want := schema.GroupResource{Group: "apps", Resource: "deployments"}
	if got := getGroupResource("deployments"); got != want {
		t.Errorf("getGroupResource(core) = %v, want %v", got, want)
	}
	if got := getGroupResource("deployments.apps"); got != want {
		t.Errorf("getGroupResource(qualified) = %v, want %v", got, want)
	}
}

func TestScaleOperationsAgainstDiscoveryAndScaleAPI(t *testing.T) {
	transport := &scaleRoundTripper{}
	config := &rest.Config{Host: "https://kubernetes.example.test", Transport: transport}
	counts, err := GetReplicaCounts(config, "deployments", "default", "web")
	if err != nil || counts.DesiredReplicas != 3 || counts.ActualReplicas != 2 {
		t.Fatalf("GetReplicaCounts() = %#v, %v", counts, err)
	}

	counts, err = ScaleResource(config, "deployments", "default", "web", "4")
	if err != nil || counts.DesiredReplicas != 4 || counts.ActualReplicas != 2 || transport.updated != 4 {
		t.Fatalf("ScaleResource() = %#v, %v; update sent replicas %d", counts, err, transport.updated)
	}
	if _, err := ScaleResource(config, "deployments", "default", "web", "not-a-number"); err == nil {
		t.Fatal("ScaleResource() accepted a non-numeric replica count")
	}
}

func TestScaleOperationsReturnAPIErrors(t *testing.T) {
	getFailure := &rest.Config{Host: "https://kubernetes.example.test", Transport: &scaleRoundTripper{failGet: true}}
	if _, err := GetReplicaCounts(getFailure, "deployments", "default", "web"); err == nil || !strings.Contains(err.Error(), "scale get failed") {
		t.Fatalf("GetReplicaCounts() error = %v; want scale get failed", err)
	}
	if _, err := ScaleResource(getFailure, "deployments", "default", "web", "3"); err == nil || !strings.Contains(err.Error(), "scale get failed") {
		t.Fatalf("ScaleResource() error = %v; want scale get failed", err)
	}

	updateFailure := &rest.Config{Host: "https://kubernetes.example.test", Transport: &scaleRoundTripper{failUpdate: true}}
	if _, err := ScaleResource(updateFailure, "deployments", "default", "web", "3"); err == nil || !strings.Contains(err.Error(), "scale update failed") {
		t.Fatalf("ScaleResource() error = %v; want scale update failed", err)
	}
}
