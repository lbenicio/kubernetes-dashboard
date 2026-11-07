// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {HttpClient} from '@angular/common/http';
import {of} from 'rxjs';
import {LocalConfigLoaderService} from './loader';

describe('LocalConfigLoaderService', () => {
  it('loads and exposes the local application configuration', async () => {
    const config = {oidcEnabled: true, authTokenCookieName: 'kd-token'};
    const http = {get: jest.fn().mockReturnValue(of(config))};
    const loader = new LocalConfigLoaderService(http as unknown as HttpClient);

    expect(loader.appConfig).toEqual({});
    await expect(loader.init()).resolves.toBe(config);

    expect(http.get).toHaveBeenCalledWith('assets/config/config.json');
    expect(loader.appConfig).toBe(config);
  });
});
