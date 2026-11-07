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

package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
)

type testReadCloser struct {
	reader io.Reader
	err    error
	closed bool
}

func (r *testReadCloser) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.reader.Read(p)
}

func (r *testReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestHandleDownloadCopiesAndClosesStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := &testReadCloser{reader: strings.NewReader("download contents")}
	handleDownload(restful.NewResponse(recorder), stream)

	if got := recorder.Header().Get(restful.HEADER_ContentType); got != "text/plain" {
		t.Errorf("Content-Type = %q; want text/plain", got)
	}
	if recorder.Body.String() != "download contents" {
		t.Errorf("body = %q; want download contents", recorder.Body.String())
	}
	if !stream.closed {
		t.Error("download stream was not closed")
	}
}

func TestHandleDownloadReportsReadErrorAndClosesStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := &testReadCloser{err: errors.New("stream failed")}
	handleDownload(restful.NewResponse(recorder), stream)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !stream.closed {
		t.Error("download stream was not closed after read failure")
	}
}
