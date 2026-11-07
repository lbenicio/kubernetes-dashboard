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

import {HttpErrorResponse} from '@angular/common/http';
import {EventEmitter} from '@angular/core';
import {PinnedResource} from '@api/root.api';
import {of} from 'rxjs';
import {PinnerService} from './pinner';

describe('PinnerService', () => {
  const pinnedResources = [
    {kind: 'Pod', name: 'web', namespace: 'default', displayName: 'Web pod'},
    {kind: 'Service', name: 'web', namespace: 'default', displayName: 'Web service'},
    {kind: 'Pod', name: 'metrics', namespace: 'monitoring', displayName: 'Metrics pod'},
  ] as PinnedResource[];

  it('loads resources and reloads when a pin changes or a resource is deleted', () => {
    const http = {get: jest.fn().mockReturnValue(of(pinnedResources))};
    const verber = {onDelete: new EventEmitter<boolean>()};
    const service = new PinnerService({open: jest.fn()} as never, http as never, verber as never);

    expect(service.isInitialized()).toBe(false);
    service.init();

    expect(service.isInitialized()).toBe(true);
    expect(service.isPinned('Pod', 'web', 'default')).toBe(true);
    expect(service.isPinned('Pod', 'web', 'other')).toBe(false);
    expect(service.getPinnedForKind('Pod')).toEqual([pinnedResources[0], pinnedResources[2]]);
    expect(http.get).toHaveBeenCalledTimes(1);

    service.onPinUpdate.next();
    verber.onDelete.emit(true);
    expect(http.get).toHaveBeenCalledTimes(3);
  });

  it('pins, unpins, and deletes pinned resources through the expected endpoints', () => {
    const http = {
      get: jest.fn().mockReturnValue(of(pinnedResources)),
      put: jest.fn().mockReturnValue(of({})),
      delete: jest.fn().mockReturnValue(of({})),
    };
    const service = new PinnerService(
      {open: jest.fn()} as never,
      http as never,
      {onDelete: new EventEmitter()} as never
    );
    service.init();

    service.pin('Pod', 'web', 'default', 'Web pod', true);
    expect(http.put).toHaveBeenCalledWith('settings/pinnedresources', {
      kind: 'Pod',
      name: 'web',
      namespace: 'default',
      displayName: 'Web pod',
      namespaced: true,
    });

    service.unpin('Pod', 'web', 'default');
    service.unpin('Namespace', 'cluster', undefined as never);
    service.unpinResource(pinnedResources[0]);
    expect(http.delete).toHaveBeenNthCalledWith(1, 'settings/pinnedresources/Pod/default/web');
    expect(http.delete).toHaveBeenNthCalledWith(2, 'settings/pinnedresources/Namespace/cluster');
    expect(http.delete).toHaveBeenNthCalledWith(3, 'settings/pinnedresources/Pod/default/web');
  });

  it('opens an alert for a failed mutation and ignores a missing error', () => {
    const dialog = {open: jest.fn()};
    const service = new PinnerService(dialog as never, {} as never, {} as never);
    const error = new HttpErrorResponse({status: 500, statusText: 'Internal Server Error', error: 'backend failed'});

    service.handleErrorResponse_(error);

    expect(dialog.open).toHaveBeenCalledWith(
      expect.any(Function),
      expect.objectContaining({
        width: '630px',
        data: {
          title: 'Internal Server Error',
          message: 'backend failed',
          confirmLabel: 'OK',
        },
      })
    );

    dialog.open.mockClear();
    service.handleErrorResponse_(null as never);
    expect(dialog.open).not.toHaveBeenCalled();
  });

  it('uses a fallback alert message when the server response has no message', () => {
    const dialog = {open: jest.fn()};
    const service = new PinnerService(dialog as never, {} as never, {} as never);

    service.handleErrorResponse_(new HttpErrorResponse({status: 500, statusText: 'OK'}));

    expect(dialog.open).toHaveBeenCalledWith(
      expect.any(Function),
      expect.objectContaining({
        data: {
          title: 'Internal server error',
          message: 'Could not perform the operation.',
          confirmLabel: 'OK',
        },
      })
    );
  });
});
