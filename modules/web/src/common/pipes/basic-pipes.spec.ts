// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {CommaSeparatedPipe} from './commaseparated';
import {FilterByPipe} from './filterby';
import {StripAnsiPipe} from './stripansi';

describe('CommaSeparatedPipe', () => {
  const pipe = new CommaSeparatedPipe();

  it('joins values and returns an empty string for missing values', () => {
    expect(pipe.transform(['default', 'kube-system'])).toBe('default, kube-system');
    expect(pipe.transform(null as never)).toBe('');
  });
});

describe('FilterByPipe', () => {
  const pipe = new FilterByPipe();

  it('keeps only entries containing the predicate', () => {
    expect(pipe.transform(['default', 'kube-system', 'kube-public'], 'kube')).toEqual(['kube-system', 'kube-public']);
  });
});

describe('StripAnsiPipe', () => {
  const pipe = new StripAnsiPipe();

  it('removes terminal color codes', () => {
    expect(pipe.transform('\u001b[31mfailed\u001b[39m')).toBe('failed');
  });
});
