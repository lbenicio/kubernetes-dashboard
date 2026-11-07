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

import {DecoderService} from './decoder';

describe('DecoderService', () => {
  const service = new DecoderService();

  it('decodes base64 text including UTF-8 characters', () => {
    const encoded = window.btoa('Dashboard');
    expect(service.base64(encoded)).toBe('Dashboard');
  });

  it('returns a useful message for invalid or binary input', () => {
    expect(service.base64('%%%')).toBe('Unable to decode file. It might contain binary data.');
  });
});
