// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package logs

import "testing"

func TestToLogLinesSplitsTimestampedAndPlainText(t *testing.T) {
	got := ToLogLines("2026-11-07T12:00:00Z started\nplain error message\n\n2026-11-07T12:00:01Z ready")
	if len(got) != 3 {
		t.Fatalf("ToLogLines() returned %d lines: %#v", len(got), got)
	}
	if got[0].Timestamp != "2026-11-07T12:00:00Z" || got[0].Content != "started" {
		t.Errorf("timestamped line = %#v", got[0])
	}
	if got[1].Timestamp != "0" || got[1].Content != "plain error message" {
		t.Errorf("plain error line = %#v", got[1])
	}
	if got[2].Content != "ready" {
		t.Errorf("last line = %#v", got[2])
	}
	if empty := ToLogLines(""); len(empty) != 0 {
		t.Errorf("ToLogLines(empty) = %#v", empty)
	}
}

func TestGetLineIndexHandlesSentinelsDuplicatesAndMissingIDs(t *testing.T) {
	lines := LogLines{
		{Timestamp: "2026-11-07T00:00:00Z"},
		{Timestamp: "2026-11-07T00:00:01Z"},
		{Timestamp: "2026-11-07T00:00:02Z"},
		{Timestamp: "2026-11-07T00:00:02Z"},
		{Timestamp: "2026-11-07T00:00:02Z"},
		{Timestamp: "2026-11-07T00:00:03Z"},
	}
	cases := []struct {
		name string
		id   *LogLineId
		want int
	}{
		{name: "nil means newest", id: nil, want: 5},
		{name: "newest sentinel", id: &NewestLogLineId, want: 5},
		{name: "oldest sentinel", id: &OldestLogLineId, want: 0},
		{name: "empty timestamp means newest", id: &LogLineId{}, want: 5},
		{name: "unique line", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:01Z", LineNum: 1}, want: 1},
		{name: "duplicate from beginning", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:02Z", LineNum: 2}, want: 3},
		{name: "duplicate from end", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:02Z", LineNum: -1}, want: 4},
		{name: "duplicate first from end", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:02Z", LineNum: -3}, want: 2},
		{name: "invalid duplicate offset", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:02Z", LineNum: 0}, want: LineIndexNotFound},
		{name: "missing timestamp", id: &LogLineId{LogTimestamp: "2026-11-07T00:00:04Z", LineNum: 1}, want: LineIndexNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lines.getLineIndex(tc.id); got != tc.want {
				t.Fatalf("getLineIndex() = %d, want %d", got, tc.want)
			}
		})
	}
	if got := (LogLines{}).getLineIndex(nil); got != LineIndexNotFound {
		t.Errorf("getLineIndex(empty) = %d, want %d", got, LineIndexNotFound)
	}
}

func TestCreateLogLineIDUsesStableDuplicateOffsets(t *testing.T) {
	lines := LogLines{
		{Timestamp: "one"},
		{Timestamp: "two"},
		{Timestamp: "two"},
		{Timestamp: "three"},
	}
	if got := lines.createLogLineId(1); got.LogTimestamp != "two" || got.LineNum != -2 {
		t.Errorf("createLogLineId(1) = %#v", got)
	}
	if got := lines.createLogLineId(2); got.LogTimestamp != "two" || got.LineNum != -1 {
		t.Errorf("createLogLineId(2) = %#v", got)
	}
	if got := lines.createLogLineId(3); got.LogTimestamp != "three" || got.LineNum != 1 {
		t.Errorf("createLogLineId(last) = %#v", got)
	}
}

func TestSelectLogsReturnsPagesAndBoundaryMetadata(t *testing.T) {
	lines := LogLines{
		{Timestamp: "01", Content: "one"},
		{Timestamp: "02", Content: "two"},
		{Timestamp: "03", Content: "three"},
		{Timestamp: "04", Content: "four"},
		{Timestamp: "05", Content: "five"},
		{Timestamp: "06", Content: "six"},
	}
	tests := []struct {
		name      string
		selection Selection
		wantFrom  string
		wantTo    string
		wantCount int
		wantLast  bool
	}{
		{name: "first page", selection: Selection{ReferencePoint: OldestLogLineId, OffsetFrom: 0, OffsetTo: 2, LogFilePosition: Beginning}, wantFrom: "01", wantTo: "02", wantCount: 2},
		{name: "newest page shifted to loaded end", selection: Selection{ReferencePoint: NewestLogLineId, OffsetFrom: -2, OffsetTo: 2, LogFilePosition: Beginning}, wantFrom: "03", wantTo: "06", wantCount: 4, wantLast: true},
		{name: "page before oldest", selection: Selection{ReferencePoint: OldestLogLineId, OffsetFrom: -2, OffsetTo: 1, LogFilePosition: End}, wantFrom: "01", wantTo: "03", wantCount: 3, wantLast: true},
		{name: "page after newest", selection: Selection{ReferencePoint: NewestLogLineId, OffsetFrom: -1, OffsetTo: 2, LogFilePosition: Beginning}, wantFrom: "04", wantTo: "06", wantCount: 3, wantLast: true},
		{name: "larger than loaded window", selection: Selection{ReferencePoint: NewestLogLineId, OffsetFrom: -20, OffsetTo: 20}, wantFrom: "01", wantTo: "06", wantCount: 6, wantLast: true},
		{name: "zero-width page", selection: Selection{ReferencePoint: NewestLogLineId, OffsetFrom: 0, OffsetTo: 0}, wantCount: 0},
		{name: "missing reference", selection: Selection{ReferencePoint: LogLineId{LogTimestamp: "missing", LineNum: 1}, OffsetFrom: -1, OffsetTo: 1}, wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			selected, from, to, _, last := lines.SelectLogs(&tc.selection)
			if len(selected) != tc.wantCount || from != LogTimestamp(tc.wantFrom) || to != LogTimestamp(tc.wantTo) || last != tc.wantLast {
				t.Fatalf("SelectLogs() = %v, %q..%q, last=%t; want %d, %q..%q, last=%t", selected, from, to, last, tc.wantCount, tc.wantFrom, tc.wantTo, tc.wantLast)
			}
		})
	}
	if selected, from, to, selection, last := (LogLines{}).SelectLogs(&Selection{ReferencePoint: NewestLogLineId, OffsetFrom: -1, OffsetTo: 1}); len(selected) != 0 || from != "" || to != "" || selection != (Selection{}) || last {
		t.Errorf("SelectLogs(empty) = %#v, %q, %q, %#v, %t", selected, from, to, selection, last)
	}
}
