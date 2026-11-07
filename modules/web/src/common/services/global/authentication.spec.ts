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
import {AuthService} from './authentication';
import {of, throwError} from 'rxjs';

describe('AuthService', () => {
  let auth: AuthService;
  let cookies: {get: jest.Mock; set: jest.Mock; delete: jest.Mock};
  let router: {navigate: jest.Mock};
  let http: {get: jest.Mock; post: jest.Mock};
  let csrf: {getTokenForAction: jest.Mock};
  let state: {onBefore: {subscribe: jest.Mock}};
  let me: {getUser: jest.Mock; refresh: jest.Mock; reset: jest.Mock};
  const config = {
    authTokenCookieName: 'kd-token',
    csrfHeaderName: 'X-CSRF-TOKEN',
  } as never;

  beforeEach(() => {
    cookies = {get: jest.fn().mockReturnValue(''), set: jest.fn(), delete: jest.fn()};
    router = {navigate: jest.fn()};
    http = {get: jest.fn(), post: jest.fn()};
    csrf = {getTokenForAction: jest.fn().mockReturnValue(of({token: 'csrf'}))};
    state = {onBefore: {subscribe: jest.fn()}};
    me = {
      getUser: jest.fn().mockReturnValue({authenticated: false}),
      refresh: jest.fn().mockReturnValue(of({authenticated: true})),
      reset: jest.fn(),
    };
    auth = new AuthService(
      cookies as never,
      router as never,
      http as never,
      csrf as never,
      state as never,
      me as never,
      config
    );
  });

  it('logs in with a CSRF header, stores a token, then refreshes the user', done => {
    http.post.mockReturnValue(of({token: 'signed-token'}));
    const user = {authenticated: true, name: 'Ada'};
    me.refresh.mockReturnValue(of(user));

    auth.login({token: 'login-token'} as never).subscribe(result => {
      expect(result).toBe(user);
      expect(csrf.getTokenForAction).toHaveBeenCalledWith('login');
      const [url, body, options] = http.post.mock.calls[0];
      expect(url).toBe('api/v1/login');
      expect(body).toEqual({token: 'login-token'});
      expect((options.headers as HttpHeaders).get('X-CSRF-TOKEN')).toBe('csrf');
      expect(cookies.set).toHaveBeenCalledWith('kd-token', 'signed-token', null, null, 'localhost', false, 'Strict');
      expect(me.refresh).toHaveBeenCalledTimes(1);
      done();
    });
  });

  it('does not write a token cookie when the login response has no token', done => {
    http.post.mockReturnValue(of({}));

    auth.login({token: 'login-token'} as never).subscribe(() => {
      expect(cookies.set).not.toHaveBeenCalled();
      done();
    });
  });

  it('loads login configuration without using a cached response', () => {
    const configuration = {oidcEnabled: true, anonymousAdmin: false};
    http.get.mockReturnValue(of(configuration));

    expect(auth.loginConfiguration()).toBeTruthy();
    const [, options] = http.get.mock.calls[0];
    expect(options.headers.get('Cache-Control')).toBe('no-cache');
  });

  it('starts OIDC login with an action-bound CSRF header', () => {
    http.post.mockReturnValue(of({url: 'https://identity.example.test/authorize'}));

    auth.beginOidcLogin().subscribe();

    expect(csrf.getTokenForAction).toHaveBeenCalledWith('oidc');
    const [url, body, options] = http.post.mock.calls[0];
    expect(url).toBe('api/v1/oidc/login');
    expect(body).toEqual({});
    expect(options.headers.get('X-CSRF-TOKEN')).toBe('csrf');
  });

  it('routes anonymous logout directly to the dashboard', () => {
    me.getUser.mockReturnValue({authMethod: 'anonymous'});

    auth.logout();

    expect(cookies.delete).toHaveBeenCalledWith('kd-token', null, 'localhost', false, 'Strict');
    expect(me.reset).toHaveBeenCalled();
    expect(router.navigate).toHaveBeenCalledWith(['/']);
    expect(csrf.getTokenForAction).not.toHaveBeenCalled();
  });

  it.each([of(undefined), throwError(() => new Error('logout failed'))])(
    'clears browser state and routes an authenticated logout to login',
    result => {
      me.getUser.mockReturnValue({authMethod: 'token'});
      http.post.mockReturnValue(result);

      auth.logout();

      expect(me.reset).toHaveBeenCalled();
      expect(csrf.getTokenForAction).toHaveBeenCalledWith('logout');
      expect(http.post.mock.calls[0][0]).toBe('api/v1/logout');
      expect(router.navigate).toHaveBeenCalledWith(['login']);
    }
  );

  it('reports authentication methods and token-cookie presence', () => {
    me.getUser.mockReturnValue({authenticated: false, authMethod: 'header'});
    expect(auth.isAuthenticated()).toBe(false);
    expect(auth.hasAuthHeader()).toBe(true);
    expect(auth.hasOidcSession()).toBe(false);
    expect(auth.hasBrowserSession()).toBe(false);
    expect(auth.isAnonymousAdmin()).toBe(false);

    me.getUser.mockReturnValue({authenticated: true, authMethod: 'oidc'});
    expect(auth.isAuthenticated()).toBe(true);
    expect(auth.hasAuthHeader()).toBe(false);
    expect(auth.hasOidcSession()).toBe(true);
    expect(auth.hasBrowserSession()).toBe(true);

    me.getUser.mockReturnValue({authenticated: false, authMethod: 'token'});
    expect(auth.hasBrowserSession()).toBe(true);

    me.getUser.mockReturnValue({authenticated: false, authMethod: 'anonymous'});
    expect(auth.isAnonymousAdmin()).toBe(true);

    cookies.get.mockReturnValue('token-cookie');
    expect(auth.hasTokenCookie()).toBe(true);
    expect(auth.isAuthenticated()).toBe(true);
    expect(auth.hasAuthHeader()).toBe(false);
  });
});
