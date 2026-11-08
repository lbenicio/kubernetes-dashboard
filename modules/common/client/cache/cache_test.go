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

package cache

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/dashboard/errors"
	"k8s.io/dashboard/types"
)

type cacheTestValue struct {
	Name string `json:"name"`
}

func testCacheKey(name string) Key {
	return NewKey(types.ResourceKindPod, "default", "", metav1.ListOptions{LabelSelector: "app=" + name})
}

func TestCacheGetSetDeleteAndClear(t *testing.T) {
	Clear[cacheTestValue]()
	t.Cleanup(func() { Clear[cacheTestValue]() })
	key := testCacheKey("first")

	missing, found, err := Get[cacheTestValue](key)
	if err != nil || found || missing == nil {
		t.Fatalf("Get() on empty cache = %#v, %t, %v", missing, found, err)
	}

	want := &cacheTestValue{Name: "cached"}
	if err := Set[*cacheTestValue](key, want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, found, err := Get[cacheTestValue](key)
	if err != nil || !found || got.Name != want.Name {
		t.Fatalf("Get() = %#v, %t, %v", got, found, err)
	}
	if !set("direct-key", want) {
		t.Fatal("set() rejected a cache entry")
	}

	if err := Delete[cacheTestValue](key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, found, err := Get[cacheTestValue](key); err != nil || found {
		t.Fatalf("Get() after Delete = found %t, err %v", found, err)
	}
	if err := Set[*cacheTestValue](key, want); err != nil {
		t.Fatal(err)
	}
	Clear[cacheTestValue]()
	if _, found, err := Get[cacheTestValue](key); err != nil || found {
		t.Fatalf("Get() after Clear = found %t, err %v", found, err)
	}
}

func TestSyncedLoadInitializesOnceAndReusesValue(t *testing.T) {
	Clear[cacheTestValue]()
	t.Cleanup(func() { Clear[cacheTestValue]() })
	key := testCacheKey("synced")
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		value *cacheTestValue
		err   error
	}
	results := make(chan result, 2)
	load := func() (*cacheTestValue, error) {
		calls.Add(1)
		close(entered)
		<-release
		return &cacheTestValue{Name: "loaded"}, nil
	}
	go func() {
		value, err := SyncedLoad(key, load)
		results <- result{value: value, err: err}
	}()
	<-entered
	go func() {
		value, err := SyncedLoad(key, load)
		results <- result{value: value, err: err}
	}()
	close(release)

	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil || got.value == nil || got.value.Name != "loaded" {
			t.Fatalf("SyncedLoad() result = %#v, %v", got.value, got.err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("load called %d times, want once", calls.Load())
	}

	loadError := context.Canceled
	if value, err := SyncedLoad(testCacheKey("failed"), func() (*cacheTestValue, error) { return nil, loadError }); !stderrors.Is(err, loadError) || value == nil {
		t.Fatalf("SyncedLoad() error result = %#v, %v", value, err)
	}
}

func TestDeferredLoadUpdatesAndSkipsConcurrentRefresh(t *testing.T) {
	Clear[cacheTestValue]()
	t.Cleanup(func() { Clear[cacheTestValue]() })
	key := testCacheKey("deferred")
	started := make(chan struct{}, 1)
	DeferredLoad[*cacheTestValue](key, func() (*cacheTestValue, error) {
		started <- struct{}{}
		return &cacheTestValue{Name: "fresh"}, nil
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("deferred loader did not run")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if value, found, err := Get[cacheTestValue](key); err == nil && found && value.Name == "fresh" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if value, found, err := Get[cacheTestValue](key); err != nil || !found || value.Name != "fresh" {
		t.Fatalf("cache after deferred load = %#v, %t, %v", value, found, err)
	}

	cacheKey, err := key.SHA()
	if err != nil {
		t.Fatal(err)
	}
	cacheLocks.Store(cacheKey, struct{}{})
	defer cacheLocks.Delete(cacheKey)
	var skipped atomic.Bool
	DeferredLoad[*cacheTestValue](key, func() (*cacheTestValue, error) {
		skipped.Store(true)
		return nil, nil
	})
	time.Sleep(10 * time.Millisecond)
	if skipped.Load() {
		t.Fatal("second deferred refresh ran while the key lock was held")
	}

	failedKey := testCacheKey("deferred-error")
	DeferredLoad[*cacheTestValue](failedKey, func() (*cacheTestValue, error) {
		return nil, context.Canceled
	})
	time.Sleep(10 * time.Millisecond)
	if _, found, err := Get[cacheTestValue](failedKey); err != nil || found {
		t.Fatalf("failed deferred load entered cache: found=%t err=%v", found, err)
	}
}

func TestDeferredLoadReleasesItsRefreshLock(t *testing.T) {
	setCacheArg(t, "cache-refresh-debounce", "0s")
	Clear[cacheTestValue]()
	t.Cleanup(func() { Clear[cacheTestValue]() })
	key := testCacheKey("release-lock")
	cacheKey, err := key.SHA()
	if err != nil {
		t.Fatal(err)
	}
	loaded := make(chan struct{})
	DeferredLoad[*cacheTestValue](key, func() (*cacheTestValue, error) {
		close(loaded)
		return &cacheTestValue{Name: "loaded"}, nil
	})
	select {
	case <-loaded:
	case <-time.After(time.Second):
		t.Fatal("deferred loader did not run")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, locked := cacheLocks.Load(cacheKey); !locked {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("deferred refresh lock was not released")
}

func TestKeyHashAndJSON(t *testing.T) {
	key := NewKey(types.ResourceKindPod, "default", "token", metav1.ListOptions{
		LabelSelector:   "app=dashboard",
		FieldSelector:   "status.phase=Running",
		ResourceVersion: "ignored-by-key",
	})
	sha, err := key.SHA()
	if err != nil || sha == "" {
		t.Fatalf("Key.SHA() = %q, %v", sha, err)
	}
	serialized, err := key.MarshalJSON()
	if err != nil {
		t.Fatalf("Key.MarshalJSON() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(serialized, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["Context"] != "" {
		t.Fatalf("single-cluster cache key context = %v", decoded["Context"])
	}
	if base, err := key.key.MarshalJSON(); err != nil {
		t.Fatal(err)
	} else {
		var fields map[string]any
		if err := json.Unmarshal(base, &fields); err != nil {
			t.Fatal(err)
		}
		if _, includesResourceVersion := fields["Opts"].(map[string]any)["ResourceVersion"]; includesResourceVersion {
			t.Fatal("cache identity should ignore resource version")
		}
	}
}

func TestKeyExchangesAndCachesClusterContext(t *testing.T) {
	serverCalls := atomic.Int32{}
	withDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer cluster-token" {
			return nil, fmt.Errorf("exchange Authorization = %q", got)
		}
		serverCalls.Add(1)
		return exchangeResponse(http.StatusOK, "cluster-context"), nil
	}))
	setCacheArg(t, "cluster-context-enabled", "true")
	setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")

	key := NewKey(types.ResourceKindPod, "default", "cluster-token", metav1.ListOptions{})
	first, err := key.SHA()
	if err != nil {
		t.Fatalf("Key.SHA() error = %v", err)
	}
	second, err := key.SHA()
	if err != nil || first != second || serverCalls.Load() != 1 {
		t.Fatalf("cached cluster key = %q, %q, %v, calls=%d", first, second, err, serverCalls.Load())
	}
}

func TestKeyExchangeMapsErrors(t *testing.T) {
	for _, tc := range []struct {
		name         string
		code         int
		unauthorized bool
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, unauthorized: true},
		{name: "forbidden", code: http.StatusForbidden, unauthorized: true},
		{name: "bad request", code: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				return exchangeResponse(tc.code, "exchange rejected"), nil
			}))
			setCacheArg(t, "cluster-context-enabled", "true")
			setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")
			_, err := NewKey(types.ResourceKindPod, "default", "error-"+tc.name, metav1.ListOptions{}).SHA()
			if err == nil || errors.IsUnauthorized(err) != tc.unauthorized {
				t.Fatalf("Key.SHA() error = %v, unauthorized=%t", err, errors.IsUnauthorized(err))
			}
		})
	}
}

func TestCacheOperationsReturnTokenExchangeErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(Key) error
	}{
		{
			name: "get",
			run: func(key Key) error {
				_, _, err := Get[cacheTestValue](key)
				return err
			},
		},
		{
			name: "set",
			run: func(key Key) error {
				return Set(key, &cacheTestValue{Name: "not cached"})
			},
		},
		{
			name: "delete",
			run:  func(key Key) error { return Delete[cacheTestValue](key) },
		},
		{
			name: "synced load",
			run: func(key Key) error {
				value, err := SyncedLoad(key, func() (*cacheTestValue, error) {
					t.Fatal("loader ran after token exchange failed")
					return nil, nil
				})
				if value == nil {
					t.Fatal("SyncedLoad() returned a nil value on error")
				}
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			}))
			setCacheArg(t, "cluster-context-enabled", "true")
			setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")

			if err := tc.run(NewKey(types.ResourceKindPod, "default", "transport-error-"+tc.name, metav1.ListOptions{})); !stderrors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cache operation error = %v, want token exchange transport error", err)
			}
		})
	}

	t.Run("deferred load", func(t *testing.T) {
		requested := make(chan struct{}, 1)
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			requested <- struct{}{}
			return nil, context.DeadlineExceeded
		}))
		setCacheArg(t, "cluster-context-enabled", "true")
		setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")
		var loaderCalled atomic.Bool
		DeferredLoad[*cacheTestValue](NewKey(types.ResourceKindPod, "default", "transport-error-deferred", metav1.ListOptions{}), func() (*cacheTestValue, error) {
			loaderCalled.Store(true)
			return &cacheTestValue{Name: "unexpected"}, nil
		})

		select {
		case <-requested:
		case <-time.After(time.Second):
			t.Fatal("DeferredLoad() did not attempt the token exchange")
		}
		time.Sleep(10 * time.Millisecond)
		if loaderCalled.Load() {
			t.Fatal("DeferredLoad() called loader after token exchange failed")
		}
	})
}

func TestKeyExchangeRejectsUnreadableResponseBody(t *testing.T) {
	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       errorReadCloser{},
		}, nil
	}))
	setCacheArg(t, "cluster-context-enabled", "true")
	setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")

	if _, err := NewKey(types.ResourceKindPod, "default", "unreadable-body", metav1.ListOptions{}).SHA(); !stderrors.Is(err, errExchangeBodyRead) {
		t.Fatalf("Key.SHA() error = %v, want response body read error", err)
	}
}

func TestKeyExchangeStillReturnsContextWhenResponseCloseFails(t *testing.T) {
	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       &closeErrorReadCloser{},
		}, nil
	}))
	setCacheArg(t, "cluster-context-enabled", "true")
	setCacheArg(t, "token-exchange-endpoint", "http://token.exchange.invalid/login")

	if got, err := NewKey(types.ResourceKindPod, "default", "close-error-body", metav1.ListOptions{}).SHA(); err != nil || got == "" {
		t.Fatalf("Key.SHA() = %q, %v; want context hash despite close error", got, err)
	}
}

var errExchangeBodyRead = stderrors.New("exchange response body read failed")

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errExchangeBodyRead }
func (errorReadCloser) Close() error             { return nil }

type closeErrorReadCloser struct {
	read bool
}

func (in *closeErrorReadCloser) Read(buffer []byte) (int, error) {
	if in.read {
		return 0, io.EOF
	}
	in.read = true
	return copy(buffer, "context"), nil
}

func (closeErrorReadCloser) Close() error { return stderrors.New("close failed") }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func exchangeResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func withDefaultTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func setCacheArg(t *testing.T, name, value string) {
	t.Helper()
	flag := pflag.Lookup(name)
	old := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(old) })
	if err := flag.Value.Set(value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}
