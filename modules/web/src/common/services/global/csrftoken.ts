// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {HttpClient} from '@angular/common/http';
import {Injectable} from '@angular/core';
import {CookieService} from 'ngx-cookie-service';
import {CsrfToken} from '@api/root.api';
import {Observable, of} from 'rxjs';
import {tap} from 'rxjs/operators';

const CSRF_TOKEN_STORAGE_KEY = 'kd-csrf-token';
const CSRF_COOKIE_NAME = 'csrf-token';

@Injectable()
export class CsrfTokenService {
  constructor(
    private readonly http_: HttpClient,
    private readonly cookies_: CookieService,
  ) {}

  /**
   * Get a CSRF token for an action you want to perform.
   *
   * The token is retrieved from sessionStorage first (which is not affected by
   * Safari's Intelligent Tracking Prevention). If not available, we try to read
   * the server-set csrf-token cookie, then fall back to the API.
   */
  getTokenForAction(action: string): Observable<CsrfToken> {
    // 1. Check sessionStorage first (fast path, works even with ITP)
    const stored = this._getFromStorage();
    if (stored) {
      return of({token: stored});
    }

    // 2. Try to read from server-set cookie (set during OIDC callback, avoids
    //    client-side document.cookie writes that ITP may block)
    const cookieToken = this._getFromCookie();
    if (cookieToken) {
      this._storeInSessionStorage(cookieToken);
      return of({token: cookieToken});
    }

    // 3. Fall back to API fetch (last resort, may be blocked in some scenarios)
    return this.http_.get<CsrfToken>(`api/v1/csrftoken/${action}`).pipe(
      tap(response => {
        if (response.token) {
          this._storeInSessionStorage(response.token);
        }
      }),
    );
  }

  /**
   * Initialize the CSRF token storage by attempting to read the server-set
   * csrf-token cookie. Call this on app startup.
   */
  initializeToken(): void {
    const cookieToken = this._getFromCookie();
    if (cookieToken) {
      this._storeInSessionStorage(cookieToken);
    }
  }

  private _getFromStorage(): string | null {
    try {
      return sessionStorage.getItem(CSRF_TOKEN_STORAGE_KEY);
    } catch {
      return null;
    }
  }

  private _storeInSessionStorage(token: string): void {
    try {
      sessionStorage.setItem(CSRF_TOKEN_STORAGE_KEY, token);
    } catch {
      // sessionStorage may be unavailable (e.g., in private browsing on some
      // browsers). Fall back gracefully — the token will be fetched on demand.
    }
  }

  private _getFromCookie(): string | null {
    try {
      const value = this.cookies_.get(CSRF_COOKIE_NAME);
      return value || null;
    } catch {
      return null;
    }
  }
}
