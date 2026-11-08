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

package container

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"k8s.io/dashboard/api/pkg/resource/logs"
)

type logTestRoundTripper func(*http.Request) (*http.Response, error)

func (in logTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func newLogTestClient(t *testing.T, logStatus int) kubernetes.Interface {
	t.Helper()
	client, err := kubernetes.NewForConfig(&rest.Config{
		Host: "https://kubernetes.test",
		Transport: logTestRoundTripper(func(r *http.Request) (*http.Response, error) {
			status := http.StatusOK
			contentType := "application/json"
			body := ""
			switch r.URL.Path {
			case "/api/v1/namespaces/ns/pods/pod":
				pod, err := json.Marshal(&corev1.Pod{
					TypeMeta:   metaV1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metaV1.ObjectMeta{Name: "pod", Namespace: "ns"},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}, {Name: "sidecar"}}},
				})
				if err != nil {
					return nil, err
				}
				body = string(pod)
			case "/api/v1/namespaces/ns/pods/pod/log":
				contentType = "text/plain"
				if r.URL.Query().Get("container") != "main" {
					status = http.StatusBadRequest
					body = "unexpected container"
				} else if logStatus != http.StatusOK {
					status = logStatus
					body = "log stream unavailable"
				} else {
					body = "2026-11-07T12:00:00Z started\n"
				}
			default:
				status = http.StatusNotFound
				body = "not found"
			}
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{contentType}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("create Kubernetes client: %v", err)
	}
	return client
}

func TestGetPodContainers(t *testing.T) {
	client := newLogTestClient(t, http.StatusOK)

	got, err := GetPodContainers(client, "ns", "pod")
	if err != nil {
		t.Fatalf("GetPodContainers() error = %v", err)
	}
	if strings.Join(got.Containers, ",") != "main,sidecar" {
		t.Errorf("containers = %v; want [main sidecar]", got.Containers)
	}
}

func TestGetLogDetailsUsesFirstContainerAndReadsStream(t *testing.T) {
	client := newLogTestClient(t, http.StatusOK)

	got, err := GetLogDetails(client, "ns", "pod", "", logs.AllSelection, false)
	if err != nil {
		t.Fatalf("GetLogDetails() error = %v", err)
	}
	if got.Info.PodName != "pod" || got.Info.ContainerName != "main" {
		t.Errorf("log info = %#v; want pod and default container main", got.Info)
	}
	if len(got.LogLines) != 1 || got.LogLines[0].Content != "started" {
		t.Errorf("log lines = %#v; want one started line", got.LogLines)
	}
}

func TestGetLogDetailsReturnsLogEndpointErrorAsLogContent(t *testing.T) {
	client := newLogTestClient(t, http.StatusInternalServerError)

	got, err := GetLogDetails(client, "ns", "pod", "main", logs.AllSelection, false)
	if err != nil {
		t.Fatalf("GetLogDetails() error = %v; expected an error line in the returned details", err)
	}
	if len(got.LogLines) != 1 || !strings.Contains(got.LogLines[0].Content, "log stream unavailable") {
		t.Errorf("log lines = %#v; want API error text", got.LogLines)
	}
}

func TestPodLogReadersReturnPodLookupErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	if _, err := GetPodContainers(client, "ns", "missing"); err == nil {
		t.Fatal("GetPodContainers() succeeded for a missing pod")
	}
	if _, err := GetLogDetails(client, "ns", "missing", "main", logs.AllSelection, false); err == nil {
		t.Fatal("GetLogDetails() succeeded for a missing pod")
	}
}

func TestGetLogFileStreamsAndClosesResponse(t *testing.T) {
	client := newLogTestClient(t, http.StatusOK)

	stream, err := GetLogFile(client, "ns", "pod", "main", &corev1.PodLogOptions{Previous: true, Timestamps: true})
	if err != nil {
		t.Fatalf("GetLogFile() error = %v", err)
	}
	defer stream.Close()
	content, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read log stream: %v", err)
	}
	if !strings.Contains(string(content), "started") {
		t.Errorf("stream = %q; want started line", content)
	}
}

func TestIsReadLimitReached(t *testing.T) {
	if !isReadLimitReached(byteReadLimit, 0, logs.Beginning) {
		t.Error("byte limit at beginning was not detected")
	}
	if !isReadLimitReached(0, lineReadLimit, logs.End) {
		t.Error("line limit at end was not detected")
	}
	if isReadLimitReached(byteReadLimit-1, lineReadLimit-1, logs.End) {
		t.Error("limit below thresholds was reported as reached")
	}
	if isReadLimitReached(byteReadLimit, 0, logs.End) {
		t.Error("byte threshold should not apply when reading from end")
	}
}
