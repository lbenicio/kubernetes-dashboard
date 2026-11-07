// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {HttpErrorResponse} from '@angular/common/http';
import {AsKdError, ErrorCode, ErrorStatus, K8SError, KdError} from './errors';

describe('API error conversion', () => {
  it.each([
    [401, ErrorStatus.unauthorized],
    [403, ErrorStatus.forbidden],
    [500, ErrorStatus.internal],
    [404, ErrorStatus.notFound],
    [502, ErrorStatus.unknown],
  ])('maps HTTP status %s', (status, expectedStatus) => {
    const result = AsKdError(new HttpErrorResponse({status, statusText: 'failure', error: 'backend message'}));

    expect(result).toEqual(new KdError(expectedStatus, status, 'backend message'));
  });

  it('uses the structured backend message and localizes known API messages', () => {
    const error = new HttpErrorResponse({
      status: 403,
      error: {ErrStatus: {message: 'MSG_ACCESS_DENIED'}},
    });

    expect(AsKdError(error).message).toBe('Access denied.');
    expect(new K8SError({reason: 'Denied', code: 403, message: 'backend'} as never).toKdError()).toEqual(
      new KdError('Denied', 403, 'backend')
    );
  });

  it('matches configured API errors only when the response body is a string', () => {
    expect(
      KdError.isError(
        new HttpErrorResponse({status: 401, error: '  MSG_LOGIN_UNAUTHORIZED_ERROR  '}),
        'MSG_LOGIN_UNAUTHORIZED_ERROR'
      )
    ).toBe(true);
    expect(
      KdError.isError(new HttpErrorResponse({status: 401, error: {message: 'unauthorized'}}), 'unauthorized')
    ).toBe(false);
    expect(KdError.isError(new HttpErrorResponse({status: 401, error: 'other'}), 'expected')).toBe(false);
  });

  it('leaves unknown localized messages unchanged', () => {
    const error = new KdError(ErrorStatus.internal, ErrorCode.internal, ' custom error ');
    expect(error.localize()).toBe(error);
    expect(error.message).toBe(' custom error ');
  });
});
