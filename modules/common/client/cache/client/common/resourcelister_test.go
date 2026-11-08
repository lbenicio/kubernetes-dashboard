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

package common_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationapiv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	authorizationv1fake "k8s.io/client-go/kubernetes/typed/authorization/v1/fake"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/client/cache"
	"k8s.io/dashboard/client/cache/client/common"
)

// TestResource is a mock resource type for testing
type TestResource struct {
	Items []string
}

// MockLister is a mock implementation of ResourceListerInterface for testing
type MockLister struct {
	shouldFail bool
	returnVal  *TestResource
}

// List implements ResourceListerInterface
func (m *MockLister) List(_ context.Context, _ metav1.ListOptions) (*TestResource, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock lister error")
	}
	return m.returnVal, nil
}

// mockRequestWithHeaders creates a RequestGetter function for testing
func mockRequestWithHeaders(headers map[string]string) common.RequestGetter {
	return func() *http.Request {
		if headers == nil {
			return nil
		}
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req
	}
}

func TestNewCachedResourceLister(t *testing.T) {
	// Test with required parameters
	auth := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
	lister := common.NewCachedResourceLister[TestResource](
		auth,
		common.WithResourceKind[TestResource]("pods"),
		common.WithToken[TestResource]("test-token"),
	)

	// We can still check that the lister was created properly
	assert.NotNil(t, lister)

	// Test that it panics when token is not provided
	assert.Panics(t, func() {
		common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
		)
	})

	// Test that it panics when resource kind is not provided
	assert.Panics(t, func() {
		common.NewCachedResourceLister[TestResource](
			auth,
			common.WithToken[TestResource]("test-token"),
		)
	})
}

func TestCachedResourceListerPassesGroupAndVersionInAccessReview(t *testing.T) {
	defer cache.Clear[TestResource]()
	var gotGroup, gotVersion string
	auth := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
	auth.AddReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		gotGroup = review.Spec.ResourceAttributes.Group
		gotVersion = review.Spec.ResourceAttributes.Version
		review.Status.Allowed = true
		return true, review, nil
	})
	lister := common.NewCachedResourceLister[TestResource](
		auth,
		common.WithResourceKind[TestResource]("widgets"),
		common.WithToken[TestResource]("test-token-with-group-version"),
		common.WithGroup[TestResource]("widgets.example.test"),
		common.WithVersion[TestResource]("v1"),
	)
	mockLister := &MockLister{returnVal: &TestResource{Items: []string{"widget"}}}
	if _, err := lister.List(context.Background(), mockLister, metav1.ListOptions{}); err != nil {
		t.Fatalf("initial List() error = %v", err)
	}
	if _, err := lister.List(context.Background(), mockLister, metav1.ListOptions{}); err != nil {
		t.Fatalf("cached List() error = %v", err)
	}
	if gotGroup != "widgets.example.test" || gotVersion != "v1" {
		t.Fatalf("access review group/version = %q/%q", gotGroup, gotVersion)
	}
}

func TestCachedResourceListerReturnsAuthorizationReviewErrors(t *testing.T) {
	defer cache.Clear[TestResource]()
	ctx := context.Background()
	options := []common.Option[TestResource]{
		common.WithResourceKind[TestResource]("pods"),
		common.WithToken[TestResource]("cached-auth-review-error"),
		common.WithNamespace[TestResource]("default"),
	}

	allow := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
	allow.AddReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	loader := common.NewCachedResourceLister[TestResource](allow, options...)
	if _, err := loader.List(ctx, &MockLister{returnVal: &TestResource{Items: []string{"cached"}}}, metav1.ListOptions{}); err != nil {
		t.Fatalf("initial List() error = %v", err)
	}

	wantErr := fmt.Errorf("authorization review unavailable")
	denied := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
	denied.AddReactor("create", "selfsubjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	reader := common.NewCachedResourceLister[TestResource](denied, options...)
	if _, err := reader.List(ctx, &MockLister{returnVal: &TestResource{Items: []string{"must-not-load"}}}, metav1.ListOptions{}); !errors.Is(err, wantErr) {
		t.Fatalf("cached List() error = %v, want %v", err, wantErr)
	}
}

func TestCachedResourceListerReturnsTokenExchangeErrors(t *testing.T) {
	defer cache.Clear[TestResource]()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = resourceListerTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "502 Bad Gateway",
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("context exchange failed")),
			Request:    request,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	setResourceListerFlag(t, "cluster-context-enabled", "true")
	setResourceListerFlag(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")

	for _, test := range []struct {
		name string
		req  *http.Request
	}{
		{name: "cache read failure with no request", req: nil},
		{name: "cache invalidation failure", req: httptest.NewRequest(http.MethodGet, "http://dashboard.test", nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.req != nil {
				test.req.Header.Set("Cache-Control", "no-cache")
			}
			auth := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
			lister := common.NewCachedResourceLister[TestResource](
				auth,
				common.WithResourceKind[TestResource]("pods"),
				common.WithToken[TestResource]("exchange-error-"+test.name),
				common.WithRequestGetter[TestResource](func() *http.Request { return test.req }),
			)
			_, err := lister.List(context.Background(), &MockLister{}, metav1.ListOptions{})
			if err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
				t.Fatalf("List() error = %v; want token exchange failure", err)
			}
		})
	}
}

type resourceListerTransport func(*http.Request) (*http.Response, error)

func (transport resourceListerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func setResourceListerFlag(t *testing.T, name, value string) {
	t.Helper()
	flag := pflag.Lookup(name)
	previous := flag.Value.String()
	if err := flag.Value.Set(value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
}

func TestCachedResourceLister_List(t *testing.T) {
	ctx := context.Background()
	auth := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}

	// Setup fake authorization that allows access
	auth.AddReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ssar := action.(k8stesting.CreateAction).GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		ssar.Status.Allowed = true
		return true, ssar, nil
	})

	t.Run("Initial cache load", func(t *testing.T) {
		defer cache.Clear[TestResource]()

		// Create a mock lister that returns test data
		mockLister := &MockLister{
			returnVal: &TestResource{
				Items: []string{"item1", "item2"},
			},
		}

		// Create the cached resource lister
		lister := common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token"),
			common.WithNamespace[TestResource]("default"),
		)

		// Call List which should load data from the mock lister
		result, err := lister.List(ctx, mockLister, metav1.ListOptions{})

		// Verify the result
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, []string{"item1", "item2"}, result.Items)
	})

	t.Run("Cache invalidation with no-cache header", func(t *testing.T) {
		defer cache.Clear[TestResource]()

		// First populate the cache
		mockLister := &MockLister{
			returnVal: &TestResource{
				Items: []string{"cached-item"},
			},
		}

		lister := common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token-3"),
			common.WithNamespace[TestResource]("default"),
			common.WithRequestGetter[TestResource](mockRequestWithHeaders(map[string]string{})),
		)

		// Initial call to populate the cache
		result, err := lister.List(ctx, mockLister, metav1.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, []string{"cached-item"}, result.Items)

		// Change the mock lister to return different data and set no-cache header
		mockLister.returnVal = &TestResource{
			Items: []string{"fresh-item"},
		}
		lister = common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token-3"),
			common.WithNamespace[TestResource]("default"),
			common.WithRequestGetter[TestResource](mockRequestWithHeaders(map[string]string{
				"Cache-Control": "no-cache",
			})),
		)

		// Call should invalidate cache and get fresh data
		result, err = lister.List(ctx, mockLister, metav1.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, []string{"fresh-item"}, result.Items)
	})

	t.Run("Authorization denied", func(t *testing.T) {
		defer cache.Clear[TestResource]()

		// Setup fake authorization that denies access
		authDeny := &authorizationv1fake.FakeAuthorizationV1{Fake: &k8stesting.Fake{}}
		authDeny.AddReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
			ssar := action.(k8stesting.CreateAction).GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
			ssar.Status.Allowed = false
			ssar.Status.Reason = "unauthorized"
			return true, ssar, nil
		})

		// Create a mock lister
		mockLister := &MockLister{
			returnVal: &TestResource{
				Items: []string{"secure-item"},
			},
		}

		lister := common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token-3"),
			common.WithNamespace[TestResource]("default"),
		)

		// Initial call to populate the cache
		result, err := lister.List(ctx, mockLister, metav1.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, []string{"secure-item"}, result.Items)

		// Create the cached resource lister with denying auth
		lister = common.NewCachedResourceLister[TestResource](
			authDeny,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token-4"),
			common.WithNamespace[TestResource]("default"),
		)

		// Call should be denied due to authorization
		_, err = lister.List(ctx, mockLister, metav1.ListOptions{})
		require.Error(t, err)
		// Check if it contains the unauthorized message
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("Lister error", func(t *testing.T) {
		defer cache.Clear[TestResource]()

		// Create a failing mock lister
		mockLister := &MockLister{
			shouldFail: true,
		}

		// Create the cached resource lister
		lister := common.NewCachedResourceLister[TestResource](
			auth,
			common.WithResourceKind[TestResource]("pods"),
			common.WithToken[TestResource]("test-token-6"),
			common.WithNamespace[TestResource]("default"),
		)

		// Call should return the error from the lister
		_, err := lister.List(ctx, mockLister, metav1.ListOptions{})
		require.Error(t, err)
		assert.Equal(t, "mock lister error", err.Error())
	})
}
