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

import {AuthorizerService} from './authorizer';
import {firstValueFrom, of, throwError} from 'rxjs';

describe('AuthorizerService', () => {
  it('checks permission before requesting a resource', async () => {
    const resource = {name: 'settings'};
    const http = {
      get: jest
        .fn()
        .mockReturnValueOnce(of({allowed: true}))
        .mockReturnValueOnce(of(resource)),
    };
    const service = new AuthorizerService(http as never);

    await expect(firstValueFrom(service.proxyGET('settings'))).resolves.toBe(resource);

    expect(http.get).toHaveBeenNthCalledWith(1, 'settings/cani');
    expect(http.get).toHaveBeenNthCalledWith(2, 'settings');
  });

  it('rejects forbidden and failed permission checks without loading the resource', async () => {
    const forbiddenHttp = {get: jest.fn().mockReturnValue(of({allowed: false}))};
    const forbiddenService = new AuthorizerService(forbiddenHttp as never);
    await expect(firstValueFrom(forbiddenService.proxyGET('settings'))).rejects.toBeDefined();
    expect(forbiddenHttp.get).toHaveBeenCalledTimes(1);

    const failedHttp = {get: jest.fn().mockReturnValue(throwError(() => new Error('offline')))};
    const failedService = new AuthorizerService(failedHttp as never);
    await expect(firstValueFrom(failedService.proxyGET('settings'))).rejects.toThrow('offline');
    expect(failedHttp.get).toHaveBeenCalledTimes(1);
  });
});
