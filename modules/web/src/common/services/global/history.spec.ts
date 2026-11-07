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

import {Navigation, NavigationEnd} from '@angular/router';
import {HistoryService} from './history';
import {Subject} from 'rxjs';

describe('HistoryService', () => {
  it('tracks consecutive navigation URLs and returns to the previous URL', async () => {
    const events = new Subject<unknown>();
    const router = {events, navigateByUrl: jest.fn().mockResolvedValue(true), navigate: jest.fn()};
    const service = new HistoryService({get: jest.fn().mockReturnValue(router)} as never);

    service.init();
    events.next(new NavigationEnd(1, '/pods', '/pods'));
    events.next(new NavigationEnd(2, '/services', '/services'));

    await expect(service.goToPreviousState('overview')).resolves.toBe(true);
    expect(router.navigateByUrl).toHaveBeenCalledWith('/pods');
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it('uses the fallback route when there is no distinct previous state', async () => {
    const router = {events: new Subject(), navigateByUrl: jest.fn(), navigate: jest.fn().mockResolvedValue(true)};
    const service = new HistoryService({get: jest.fn().mockReturnValue(router)} as never);

    service.init();
    await expect(service.goToPreviousState('overview')).resolves.toBe(true);
    expect(router.navigate).toHaveBeenCalledWith(['overview'], {queryParamsHandling: 'preserve'});

    service.pushState({
      extractedUrl: {toString: () => '/same'} as never,
      initialUrl: {toString: () => '/same'} as never,
    } as unknown as Navigation);
    await service.goToPreviousState('overview');
    expect(router.navigate).toHaveBeenCalledTimes(2);
  });
});
