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

import {cropUrl} from './crop-url';

describe('cropUrl', () => {
  it('returns URLs that fit without changing them', () => {
    const url = 'https://www.example.com/path';

    expect(cropUrl(url, url.length)).toBe(url);
  });

  it('removes the scheme and crops middle path segments', () => {
    expect(cropUrl('http://www.foobar.com/abc/def/ghi/index.html', 26)).toBe('foobar.com/…/index.html');
  });

  it('keeps the first and last path segments where possible', () => {
    expect(cropUrl('https://www.foobar.com/abc/def/ghi/jkl/', 26)).toBe('foobar.com/abc/def/…/jkl/');
  });

  it('shortens query parameters from the end', () => {
    expect(cropUrl('http://www.foobar.com/search/?q=foo&page=bar', 20)).toBe('foobar.com/…/?q=foo…');
    expect(cropUrl('http://www.foobar.com/?query=reallylong', 15)).toBe('foobar.com/?…');
  });

  it('shortens hyphenated values when other parts cannot be cropped', () => {
    expect(cropUrl('http://www.foobar.com/foo-bar-foo-bar.html', 12)).toBe('foobar.com/foo…');
  });

  it('validates the input types', () => {
    expect(() => cropUrl(null as never, 10)).toThrow(new TypeError('Expected url to be a string'));
    expect(() => cropUrl('https://example.com', '10' as never)).toThrow(
      new TypeError('Expected length to be a number')
    );
  });

  it('keeps an unbreakable URL intact when no shortening point is available', () => {
    const url = 'longstringwithoutseparators';

    expect(cropUrl(url, 5)).toBe(url);
  });
});
