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

import {jest} from '@jest/globals';
import {saveBlob} from './save';

const realURL = globalThis.URL;

describe('saveBlob', () => {
  afterEach(() => {
    jest.restoreAllMocks();
    jest.useRealTimers();
    Object.defineProperty(globalThis, 'URL', {configurable: true, writable: true, value: realURL});
  });

  it('downloads the blob with the requested filename and revokes its URL later', () => {
    jest.useFakeTimers();
    const objectUrl = 'blob:dashboard-log';
    const createObjectUrl = jest.fn(() => objectUrl);
    const revokeObjectUrl = jest.fn();
    class MockURL extends realURL {
      static override createObjectURL(): string {
        return createObjectUrl();
      }

      static override revokeObjectURL(url: string): void {
        revokeObjectUrl(url);
      }
    }
    globalThis.URL = MockURL;
    let clickedAnchor: HTMLAnchorElement | undefined;
    jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clickedAnchor = this;
    });

    saveBlob(new Blob(['log contents']), 'pod-container.log');

    expect(createObjectUrl).toHaveBeenCalledTimes(1);
    expect(clickedAnchor?.href).toBe(objectUrl);
    expect(clickedAnchor?.download).toBe('pod-container.log');
    expect(clickedAnchor?.rel).toBe('noopener');
    expect(revokeObjectUrl).not.toHaveBeenCalled();

    jest.advanceTimersByTime(40_000);

    expect(revokeObjectUrl).toHaveBeenCalledWith(objectUrl);
  });
});
