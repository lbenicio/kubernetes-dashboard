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

package node

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestNewDrainHelperDefaultsAndOverrides(t *testing.T) {
	defaults := newHelper(context.Background(), nil, nil)
	if !defaults.Force || defaults.Timeout != 2*time.Minute || defaults.GracePeriodSeconds != -1 ||
		!defaults.IgnoreAllDaemonSets || !defaults.DeleteEmptyDirData {
		t.Errorf("default drain helper settings = %#v", defaults)
	}

	force, ignoreDaemonSets, deleteEmptyDir := false, false, false
	timeout := 30 * time.Second
	gracePeriod := 15
	helper := newHelper(context.Background(), nil, &NodeDrainSpec{
		Force: &force, Timeout: &timeout, GracePeriodSeconds: &gracePeriod,
		IgnoreAllDaemonSets: &ignoreDaemonSets, DeleteEmptyDirData: &deleteEmptyDir,
	})
	if helper.Force || helper.Timeout != timeout || helper.GracePeriodSeconds != gracePeriod ||
		helper.IgnoreAllDaemonSets != ignoreDaemonSets || helper.DeleteEmptyDirData != deleteEmptyDir {
		t.Errorf("overridden drain helper settings = %#v", helper)
	}
}

func TestDrainNode(t *testing.T) {
	t.Run("missing node", func(t *testing.T) {
		if err := DrainNode(fake.NewClientset(), "missing", nil); err == nil {
			t.Fatal("DrainNode() succeeded for a missing node")
		}
	})

	t.Run("node with no pods", func(t *testing.T) {
		client := fake.NewClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
		if err := DrainNode(client, "worker", nil); err != nil {
			t.Fatalf("DrainNode() error = %v", err)
		}
	})
}

func TestDrainNodeReportsCordonAndPodListingFailures(t *testing.T) {
	t.Run("cordon failure", func(t *testing.T) {
		client := fake.NewClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
		client.PrependReactor("patch", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("node update failed")
		})
		if err := DrainNode(client, "worker", nil); err == nil || !strings.Contains(err.Error(), "error cordoning node") {
			t.Fatalf("DrainNode() error = %v; want a cordoning error", err)
		}
	})

	t.Run("pod listing failure", func(t *testing.T) {
		client := fake.NewClientset(&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec:       corev1.NodeSpec{Unschedulable: true},
		})
		client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod list failed")
		})
		if err := DrainNode(client, "worker", nil); err == nil || !strings.Contains(err.Error(), "error draining node") {
			t.Fatalf("DrainNode() error = %v; want a draining error", err)
		}
	})
}
