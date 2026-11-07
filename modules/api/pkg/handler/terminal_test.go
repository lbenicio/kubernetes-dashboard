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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/igm/sockjs-go.v2/sockjs"
	"k8s.io/client-go/tools/remotecommand"
)

type testSockJSSession struct {
	received []string
	recvErr  error
	sent     []string
	sendErr  error
	closed   bool
	closeErr error
}

func (s *testSockJSSession) ID() string { return "test-session" }

func (s *testSockJSSession) Recv() (string, error) {
	if s.recvErr != nil {
		return "", s.recvErr
	}
	if len(s.received) == 0 {
		return "", errors.New("no message")
	}
	message := s.received[0]
	s.received = s.received[1:]
	return message, nil
}

func (s *testSockJSSession) Send(message string) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, message)
	return nil
}

func (s *testSockJSSession) Close(uint32, string) error {
	s.closed = true
	return s.closeErr
}

var _ sockjs.Session = (*testSockJSSession)(nil)

func TestTerminalSessionNext(t *testing.T) {
	sizeChan := make(chan remotecommand.TerminalSize, 1)
	sizeChan <- remotecommand.TerminalSize{Width: 90, Height: 30}
	got := (TerminalSession{sizeChan: sizeChan}).Next()
	if got == nil || got.Width != 90 || got.Height != 30 {
		t.Fatalf("Next() = %v; want 90x30", got)
	}

	sizeChan <- remotecommand.TerminalSize{}
	if got := (TerminalSession{sizeChan: sizeChan}).Next(); got != nil {
		t.Errorf("Next(zero size) = %v; want nil", got)
	}
}

func TestTerminalSessionRead(t *testing.T) {
	t.Run("stdin", func(t *testing.T) {
		session := &testSockJSSession{received: []string{`{"Op":"stdin","Data":"hello"}`}}
		terminal := TerminalSession{sockJSSession: session, sizeChan: make(chan remotecommand.TerminalSize, 1)}
		buffer := make([]byte, 3)
		n, err := terminal.Read(buffer)
		if err != nil || n != 3 || string(buffer) != "hel" {
			t.Fatalf("Read() = %q, %d, %v; want hel, 3, nil", buffer, n, err)
		}
	})

	t.Run("resize", func(t *testing.T) {
		session := &testSockJSSession{received: []string{`{"Op":"resize","Rows":24,"Cols":80}`}}
		sizeChan := make(chan remotecommand.TerminalSize, 1)
		terminal := TerminalSession{sockJSSession: session, sizeChan: sizeChan}
		if n, err := terminal.Read(make([]byte, 1)); n != 0 || err != nil {
			t.Fatalf("Read(resize) = %d, %v; want 0, nil", n, err)
		}
		if size := <-sizeChan; size.Width != 80 || size.Height != 24 {
			t.Errorf("resize = %+v; want 80x24", size)
		}
	})

	t.Run("unknown operation terminates input", func(t *testing.T) {
		session := &testSockJSSession{received: []string{`{"Op":"other"}`}}
		buffer := make([]byte, len(END_OF_TRANSMISSION))
		n, err := (TerminalSession{sockJSSession: session}).Read(buffer)
		if err == nil || n != 1 || string(buffer[:n]) != END_OF_TRANSMISSION {
			t.Fatalf("Read(unknown) = %q, %d, %v; want EOT and error", buffer, n, err)
		}
	})

	t.Run("receive error terminates input", func(t *testing.T) {
		session := &testSockJSSession{recvErr: errors.New("connection closed")}
		buffer := make([]byte, len(END_OF_TRANSMISSION))
		n, err := (TerminalSession{sockJSSession: session}).Read(buffer)
		if err == nil || n != 1 || string(buffer[:n]) != END_OF_TRANSMISSION {
			t.Fatalf("Read(error) = %q, %d, %v; want EOT and error", buffer, n, err)
		}
	})
}

func TestTerminalSessionWriteAndToast(t *testing.T) {
	session := &testSockJSSession{}
	terminal := TerminalSession{sockJSSession: session}
	if n, err := terminal.Write([]byte("output")); err != nil || n != len("output") {
		t.Fatalf("Write() = %d, %v; want %d, nil", n, err, len("output"))
	}
	if err := terminal.Toast("notice"); err != nil {
		t.Fatalf("Toast() error = %v", err)
	}
	if len(session.sent) != 2 {
		t.Fatalf("sent %d messages; want 2", len(session.sent))
	}
	var output, toast TerminalMessage
	if err := json.Unmarshal([]byte(session.sent[0]), &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(session.sent[1]), &toast); err != nil {
		t.Fatal(err)
	}
	if output.Op != "stdout" || output.Data != "output" || toast.Op != "toast" || toast.Data != "notice" {
		t.Errorf("messages = %#v, %#v", output, toast)
	}

	session.sendErr = errors.New("send failed")
	if _, err := terminal.Write([]byte("x")); err == nil {
		t.Error("Write() error = nil; want send error")
	}
	if err := terminal.Toast("x"); err == nil {
		t.Error("Toast() error = nil; want send error")
	}
}

func TestSessionMapGetSetClose(t *testing.T) {
	session := &testSockJSSession{}
	sizeChan := make(chan remotecommand.TerminalSize)
	terminal := TerminalSession{id: "one", sockJSSession: session, sizeChan: sizeChan}
	store := SessionMap{Sessions: make(map[string]TerminalSession)}
	store.Set("one", terminal)
	if got := store.Get("one"); got.id != "one" {
		t.Fatalf("Get(one) = %#v", got)
	}
	store.Close("one", 1, "finished")
	if !session.closed {
		t.Error("Close() did not close SockJS session")
	}
	if _, ok := <-sizeChan; ok {
		t.Error("Close() did not close terminal size channel")
	}
	if got := store.Get("one"); got.id != "" {
		t.Errorf("Get(one) after Close() = %#v; want empty session", got)
	}
}

func TestHandleTerminalSession(t *testing.T) {
	tests := []struct {
		name    string
		session *testSockJSSession
	}{
		{name: "receive error", session: &testSockJSSession{recvErr: errors.New("closed")}},
		{name: "malformed message", session: &testSockJSSession{received: []string{"{"}}},
		{name: "wrong operation", session: &testSockJSSession{received: []string{`{"Op":"stdin"}`}}},
		{name: "unknown terminal", session: &testSockJSSession{received: []string{`{"Op":"bind","SessionID":"missing"}`}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { handleTerminalSession(tc.session) })
	}

	id := "coverage-bind-session"
	bound := make(chan struct{}, 1)
	terminalSessions.Set(id, TerminalSession{id: id, bound: bound, sizeChan: make(chan remotecommand.TerminalSize, 1)})
	defer func() {
		terminalSessions.Lock.Lock()
		delete(terminalSessions.Sessions, id)
		terminalSessions.Lock.Unlock()
	}()

	sockSession := &testSockJSSession{received: []string{fmt.Sprintf(`{"Op":"bind","SessionID":%q}`, id)}}
	handleTerminalSession(sockSession)
	if got := terminalSessions.Get(id); got.sockJSSession != sockSession {
		t.Error("handleTerminalSession() did not attach the SockJS session")
	}
	if len(bound) != 1 {
		t.Error("handleTerminalSession() did not signal the waiting terminal")
	}
}

func TestTerminalSessionIDAndShellValidation(t *testing.T) {
	id, err := genTerminalSessionId()
	if err != nil {
		t.Fatalf("genTerminalSessionId() error = %v", err)
	}
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Errorf("terminal session ID %q is not 32 lowercase hexadecimal characters", id)
	}
	for _, shell := range []string{"bash", "sh", "powershell", "cmd"} {
		if !isValidShell([]string{"bash", "sh", "powershell", "cmd"}, shell) {
			t.Errorf("isValidShell(%q) = false", shell)
		}
	}
	if isValidShell([]string{"bash", "sh"}, "zsh") {
		t.Error("isValidShell(zsh) = true; want false")
	}
}
