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
	if value, err := SyncedLoad(testCacheKey("failed"), func() (*cacheTestValue, error) { return nil, loadError }); err != loadError || value == nil {
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
