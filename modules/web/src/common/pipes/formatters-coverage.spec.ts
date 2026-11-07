/*
 * Copyright 2017 The Kubernetes Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Copyright 2026 The Kubernetes Authors.
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

import {DecimalPipe} from '@angular/common';
import {CoreFormatter} from './coreformatter';
import {MemoryFormatter} from './memoryformatter';
import {RelativeTimeFormatter} from './relativetime';
import {TrimPipe} from './trim';
import {UntrimPipe} from './untrim';

describe('CoreFormatter', () => {
  const formatter = new CoreFormatter(new DecimalPipe('en-US'));

  it.each([
    [250, '250.00m'],
    [1000, '1,000.00m'],
    [1001, '1.00'],
    [1_000_001, '1.00k'],
    [1e18, '1,000.00T'],
  ])('formats %s millicores as %s', (value, expected) => {
    expect(formatter.transform(Number(value))).toBe(expected);
  });
});

describe('MemoryFormatter', () => {
  const formatter = new MemoryFormatter(new DecimalPipe('en-US'));

  it.each([
    [0, '0.00'],
    [1024, '1,024.00'],
    [1025, '1.00Ki'],
    [1_048_577, '1.00Mi'],
    [Number.MAX_VALUE, '1.60E+293Pi'],
  ])('formats %s bytes as %s', (value, expected) => {
    expect(formatter.transform(Number(value))).toBe(expected);
  });
});

describe('RelativeTimeFormatter', () => {
  const now = new Date('2024-11-06T12:00:00.000Z');
  const formatter = new RelativeTimeFormatter({getServerTime: () => now} as never);
  const timeBefore = (milliseconds: number): string => new Date(now.getTime() - milliseconds).toISOString();

  it('returns the placeholder when the value is absent', () => {
    expect(formatter.transform(null as never, 0)).toBe('-');
  });

  it('returns the placeholder for dates more than one second in the future', () => {
    expect(formatter.transform(new Date(now.getTime() + 2_000).toISOString(), 0)).toBe('-');
  });

  it.each([
    [0, '0 seconds ago'],
    [1_000, 'a second ago'],
    [5_000, '5 seconds ago'],
    [60_000, 'a minute ago'],
    [2 * 60_000, '2 minutes ago'],
    [60 * 60_000, 'an hour ago'],
    [2 * 60 * 60_000, '2 hours ago'],
    [24 * 60 * 60_000, 'a day ago'],
    [2 * 24 * 60 * 60_000, '2 days ago'],
    [30 * 24 * 60 * 60_000, 'a month ago'],
    [60 * 24 * 60 * 60_000, '2 months ago'],
    [365 * 24 * 60 * 60_000, 'a year ago'],
    [2 * 365 * 24 * 60 * 60_000, '2 years ago'],
  ])('formats an age of %s milliseconds as %s', (milliseconds, expected) => {
    expect(formatter.transform(timeBefore(Number(milliseconds)), 0)).toBe(expected);
  });

  it('uses the client clock when the server time is unavailable', () => {
    jest.useFakeTimers().setSystemTime(now);
    const clientClockFormatter = new RelativeTimeFormatter({getServerTime: (): Date => undefined as never} as never);

    expect(clientClockFormatter.transform(timeBefore(5_000), 0)).toBe('5 seconds ago');

    jest.useRealTimers();
  });
});

describe('TrimPipe', () => {
  const pipe = new TrimPipe();

  it('returns an empty string for missing input and trims text', () => {
    expect(pipe.transform('')).toBe('');
    expect(pipe.transform('  value  ')).toBe('value');
  });
});

describe('UntrimPipe', () => {
  const pipe = new UntrimPipe();

  it('replaces whitespace with non-breaking spaces and accepts safe HTML values', () => {
    const nbsp = String.fromCharCode(160);
    expect(pipe.transform('a b\nc')).toBe('a' + nbsp + 'b' + nbsp + 'c');
    expect(pipe.transform({toString: () => 'x y'} as never)).toBe('x' + nbsp + 'y');
    expect(pipe.transform(null as never)).toBeUndefined();
  });
});
