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
