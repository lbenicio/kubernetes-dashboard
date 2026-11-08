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

import {HttpHeaders} from '@angular/common/http';
import {DEFAULT_SETTINGS, GlobalSettingsService} from './globalsettings';
import {firstValueFrom, of, throwError} from 'rxjs';

describe('GlobalSettingsService', () => {
  it('loads settings merged with defaults and refreshes visibility-dependent intervals', async () => {
    const settings = {
      clusterName: 'production',
      defaultNamespace: 'kube-system',
      namespaceFallbackList: ['kube-system', 'default'],
      logsAutoRefreshTimeInterval: 12,
      resourceAutoRefreshTimeInterval: 20,
    };
    const http = {get: jest.fn().mockReturnValue(of(settings))};
    const authorizer = {proxyGET: jest.fn()};
    const service = new GlobalSettingsService(http as never, authorizer as never);
    const update = jest.fn();
    service.onSettingsUpdate.subscribe(update);

    expect(service.isInitialized()).toBe(false);
    const result = await service.init();

    expect(result).toEqual(settings);
    expect(service.getItemsPerPage()).toBe(DEFAULT_SETTINGS.itemsPerPage);
    expect(service.isInitialized()).toBe(true);
    expect(service.getClusterName()).toBe('production');
    expect(service.getDefaultNamespace()).toBe('kube-system');
    expect(service.getNamespaceFallbackList()).toEqual(['kube-system', 'default']);
    expect(service.getLogsAutoRefreshTimeInterval()).toBe(12);
    expect(service.getResourceAutoRefreshTimeInterval()).toBe(20);
    expect(update).toHaveBeenCalled();

    service.onPageVisibilityChange.emit(false);
    expect(service.getLogsAutoRefreshTimeInterval()).toBe(0);
    expect(service.getResourceAutoRefreshTimeInterval()).toBe(0);

    service.onPageVisibilityChange.emit(true);
    expect(service.getLogsAutoRefreshTimeInterval()).toBe(12);
  });

  it('invokes the successful settings callback with merged settings', async () => {
    const serverSettings = {clusterName: 'staging'};
    const http = {get: jest.fn().mockReturnValue(of(serverSettings))};
    const service = new GlobalSettingsService(http as never, {} as never);
    const onLoad = jest.fn();

    await service.load(onLoad);

    expect(onLoad).toHaveBeenCalledWith(
      expect.objectContaining({
        ...DEFAULT_SETTINGS,
        clusterName: 'staging',
      })
    );
  });

  it('returns defaults and invokes the failure callback when settings cannot load', async () => {
    const error = new Error('settings unavailable');
    const http = {get: jest.fn().mockReturnValue(throwError(() => error))};
    const service = new GlobalSettingsService(http as never, {} as never);
    const onFail = jest.fn();

    await expect(service.load(undefined, onFail)).resolves.toEqual(DEFAULT_SETTINGS);

    expect(service.isInitialized()).toBe(false);
    expect(onFail).toHaveBeenCalledWith(error);
    expect(service.getItemsPerPage()).toBe(DEFAULT_SETTINGS.itemsPerPage);
    expect(service.getLabelsLimit()).toBe(DEFAULT_SETTINGS.labelsLimit);
    expect(service.getDisableAccessDeniedNotifications()).toBe(false);
    expect(service.getHideAllNamespaces()).toBe(false);
  });

  it('returns defaults when settings fail and no failure callback is supplied', async () => {
    const http = {get: jest.fn().mockReturnValue(throwError(() => new Error('settings unavailable')))};
    const service = new GlobalSettingsService(http as never, {} as never);

    await expect(service.load()).resolves.toEqual(DEFAULT_SETTINGS);
  });

  it('falls back to the configured namespace when the server fallback list is invalid', async () => {
    const http = {get: jest.fn().mockReturnValue(of({defaultNamespace: 'custom', namespaceFallbackList: 'invalid'}))};
    const service = new GlobalSettingsService(http as never, {} as never);

    await service.load();

    expect(service.getNamespaceFallbackList()).toEqual(['custom']);
  });

  it('checks permissions and saves settings with JSON content headers', async () => {
    const savedSettings = {clusterName: 'test'};
    const http = {
      get: jest.fn(),
      put: jest.fn().mockReturnValue(of(savedSettings)),
    };
    const authorizer = {proxyGET: jest.fn().mockReturnValue(of({clusterName: 'test'}))};
    const service = new GlobalSettingsService(http as never, authorizer as never);

    await expect(firstValueFrom(service.canI())).resolves.toBe(true);
    authorizer.proxyGET.mockReturnValue(throwError(() => new Error('forbidden')));
    await expect(firstValueFrom(service.canI())).resolves.toBe(false);

    await expect(firstValueFrom(service.save(savedSettings as never))).resolves.toBe(savedSettings);
    const [url, body, options] = http.put.mock.calls[0];
    expect(url).toBe('settings');
    expect(body).toBe(savedSettings);
    expect(options.method).toBe('PUT');
    expect((options.headers as HttpHeaders).get('Content-Type')).toBe('application/json');
  });
});
