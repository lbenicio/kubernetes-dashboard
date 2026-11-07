// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {HttpClient, HttpHeaders} from '@angular/common/http';
import {of} from 'rxjs';
import {CsrfTokenService} from './csrftoken';

describe('CsrfTokenService', () => {
  it('requests an action-bound token without a cached response', () => {
    const http = {get: jest.fn().mockReturnValue(of({token: 'csrf'}))};
    const service = new CsrfTokenService(http as unknown as HttpClient);
    const result = jest.fn();

    service.getTokenForAction('oidc').subscribe(result);

    expect(result).toHaveBeenCalledWith({token: 'csrf'});
    expect(http.get).toHaveBeenCalledWith('api/v1/csrftoken/oidc', {
      headers: expect.any(HttpHeaders),
    });
    expect(http.get.mock.calls[0][1].headers.get('Cache-Control')).toBe('no-cache');
  });
});
