// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {HttpRequest, HttpResponse} from '@angular/common/http';
import {of} from 'rxjs';
import {AuthInterceptor} from './interceptor';

describe('AuthInterceptor', () => {
  const config = {authTokenCookieName: 'kd-token', authTokenHeaderName: 'X-Auth-Token'} as never;
  let cookies: {get: jest.Mock};
  let auth: {
    isAnonymousAdmin: jest.Mock;
    removeTokenCookie: jest.Mock;
    isAuthenticated: jest.Mock;
    hasTokenCookie: jest.Mock;
  };
  let me: object;
  let next: {handle: jest.Mock};
  let interceptor: AuthInterceptor;

  beforeEach(() => {
    cookies = {get: jest.fn().mockReturnValue('signed-token')};
    auth = {
      isAnonymousAdmin: jest.fn().mockReturnValue(false),
      removeTokenCookie: jest.fn(),
      isAuthenticated: jest.fn().mockReturnValue(false),
      hasTokenCookie: jest.fn().mockReturnValue(true),
    };
    me = {};
    next = {handle: jest.fn().mockReturnValue(of(new HttpResponse()))};
    interceptor = new AuthInterceptor(cookies as never, auth as never, me as never, config);
  });

  it('removes the token and passes through requests for anonymous administrators', () => {
    const request = new HttpRequest('GET', 'api/v1/pod');
    auth.isAnonymousAdmin.mockReturnValue(true);

    interceptor.intercept(request, next as never).subscribe();

    expect(auth.removeTokenCookie).toHaveBeenCalled();
    expect(cookies.get).not.toHaveBeenCalled();
    expect(next.handle).toHaveBeenCalledWith(request);
  });

  it('does not add a cookie token when authentication comes from an authorization header', () => {
    const request = new HttpRequest('GET', 'api/v1/pod');
    auth.isAuthenticated.mockReturnValue(true);
    auth.hasTokenCookie.mockReturnValue(false);

    interceptor.intercept(request, next as never).subscribe();

    expect(cookies.get).not.toHaveBeenCalled();
    expect(next.handle).toHaveBeenCalledWith(request);
  });

  it.each(['api/v1/pod', 'settings'])('adds the bearer header to dashboard request %s', url => {
    const request = new HttpRequest('GET', url);

    interceptor.intercept(request, next as never).subscribe();

    const forwardedRequest = next.handle.mock.calls[0][0] as HttpRequest<unknown>;
    expect(forwardedRequest).not.toBe(request);
    expect(forwardedRequest.headers.get('X-Auth-Token')).toBe('Bearer signed-token');
  });

  it('does not add the token to requests outside the dashboard API', () => {
    const request = new HttpRequest('GET', 'https://identity.example.test/profile');

    interceptor.intercept(request, next as never).subscribe();

    expect(next.handle).toHaveBeenCalledWith(request);
  });
});
