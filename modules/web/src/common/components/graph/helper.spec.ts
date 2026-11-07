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

import {FormattedValue} from './helper';

describe('FormattedValue', () => {
  it('formats core values using decimal prefixes', () => {
    const value = FormattedValue.NewFormattedCoreValue(1_234_567);

    expect(value.value).toBe(1.23);
    expect(value.suffix).toBe('k');
    expect(value.suffixPower).toBe(1);
  });

  it('formats memory values using binary prefixes', () => {
    const value = FormattedValue.NewFormattedMemoryValue(3 * 1_048_576);

    expect(value.value).toBe(3);
    expect(value.suffix).toBe('Mi');
    expect(value.suffixPower).toBe(2);
  });

  it('keeps zero and values below a prefix in the base unit', () => {
    const zero = FormattedValue.NewFormattedMemoryValue(0);
    const small = FormattedValue.NewFormattedCoreValue(999);

    expect(zero.value).toBe(0);
    expect(zero.suffix).toBe('');
    expect(small.value).toBe(0.999);
    expect(small.suffix).toBe('');
  });

  it('caps values at the largest supported suffix', () => {
    const value = FormattedValue.NewFormattedMemoryValue(Number.MAX_VALUE);

    expect(value.suffix).toBe('Pi');
    expect(value.value).toBeGreaterThan(0);
  });

  it('normalizes a value to a requested suffix in either direction', () => {
    const value = FormattedValue.NewFormattedMemoryValue(3 * 1_048_576);

    value.normalize('Ki');
    expect(value.value).toBe(3070);
    expect(value.suffix).toBe('Ki');

    value.normalize('Mi');
    expect(value.value).toBe(3);
    expect(value.suffix).toBe('Mi');
  });

  it('rejects suffixes that are not supported', () => {
    const value = FormattedValue.NewFormattedMemoryValue(1024);

    expect(() => value.normalize('Ei')).toThrow("Suffix 'Ei' not recognized.");
  });
});
