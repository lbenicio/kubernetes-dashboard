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

/*
 * Adapted from crop-url by William Belle, Copyright (c) 2017-2021.
 * Licensed under the MIT License:
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

export function cropUrl(url: string, length: number): string {
  if (typeof url !== 'string') {
    throw new TypeError('Expected url to be a string');
  }

  if (typeof length !== 'number') {
    throw new TypeError('Expected length to be a number');
  }

  if (url.length <= length) {
    return url;
  }

  url = url.replace(/^https?:\/\//, '');
  url = url.replace(/^www\./, '');

  let previousLength = url.length + 1;
  while (url.length > length && url.length !== previousLength) {
    previousLength = url.length;
    url = url.replace(/(.*[^/])\/[^/…]+\/([^/])/, '$1/…/$2');
  }

  url = url.replace(/\/…\/(?:…\/)+/, '/…/');

  let index: number;
  while (url.length > length) {
    index = url.lastIndexOf('&');
    if (index === -1) {
      break;
    }
    url = url.substring(0, index) + '…';
  }

  if (url.length > length) {
    index = url.lastIndexOf('?');
    if (index !== -1) {
      url = url.substring(0, index) + '?…';
    }
  }

  while (url.length > length) {
    index = url.lastIndexOf('-');
    if (index === -1) {
      break;
    }
    url = url.substring(0, index) + '…';
  }

  return url;
}
