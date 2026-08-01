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

import {HttpClient, HttpHeaders} from '@angular/common/http';
import {Inject, Injectable} from '@angular/core';
import {Router} from '@angular/router';
import {IConfig} from '@api/root.ui';
import {CookieService} from 'ngx-cookie-service';
import {interval, Observable} from 'rxjs';
import {switchMap, take, tap} from 'rxjs/operators';
import {
  AuthResponse,
  CsrfToken,
  LoginSpec,
  OIDCConfig,
  OIDCLoginResponse,
  OIDCSession,
  OIDCUserInfo,
  User,
} from 'typings/root.api';
import {CONFIG_DI_TOKEN} from '../../../index.config';
import {CsrfTokenService} from './csrftoken';
import {KdStateService} from './state';
import isEmpty from 'lodash-es/isEmpty';
import {MeService} from '@common/services/global/me';

// Refresh OIDC token every 10 minutes to keep the session alive.
// OIDC ID tokens typically expire after 1 hour, so refreshing at half that
// interval ensures the user never sees a 401 due to token expiry.
const OIDC_REFRESH_INTERVAL_MS = 10 * 60 * 1000;

@Injectable()
export class AuthService {
  private _hasAuthHeader = false;
  private _oidcConfig: OIDCConfig | null = null;
  private _refreshInProgress = false;

  constructor(
    private readonly cookies_: CookieService,
    private readonly router_: Router,
    private readonly http_: HttpClient,
    private readonly csrfTokenService_: CsrfTokenService,
    private readonly stateService_: KdStateService,
    private readonly _meService: MeService,
    @Inject(CONFIG_DI_TOKEN) private readonly config_: IConfig
  ) {
    // Refresh token on every navigation to keep the session alive
    this.stateService_.onBefore.subscribe(_ => this.refreshToken());

    // Periodic OIDC token refresh (every 10 minutes)
    interval(OIDC_REFRESH_INTERVAL_MS).subscribe(() => this.refreshToken());

    // Initialize CSRF token from server-set cookie into sessionStorage.
    // This avoids relying on client-side document.cookie writes which may be
    // blocked by Safari's Intelligent Tracking Prevention (ITP).
    this.csrfTokenService_.initializeToken();
  }

  /**
   * Fetches the OIDC configuration from the backend.
   */
  getOIDCConfig(): Observable<OIDCConfig> {
    return this.http_.get<OIDCConfig>('api/v1/oidc/config').pipe(tap(config => (this._oidcConfig = config)));
  }

  /**
   * Returns the cached OIDC configuration.
   */
  oidcConfig(): OIDCConfig | null {
    return this._oidcConfig;
  }

  /**
   * Initialize the CSRF token from server-set cookie into sessionStorage.
   * Call this after a successful OIDC callback to ensure the token is available.
   */
  initializeCsrfToken(): void {
    this.csrfTokenService_.initializeToken();
  }

  /**
   * Whether OIDC is enabled and available for login.
   */
  isOIDCEnabled(): boolean {
    return this._oidcConfig?.enabled ?? false;
  }

  /**
   * Fetches the current OIDC session info from the server.
   * Returns user info needed for impersonation-based K8s API access.
   */
  getOIDCSession(): Observable<OIDCSession> {
    return this.http_.get<OIDCSession>('api/v1/oidc/session');
  }

  /**
   * Returns the OIDC user info stored in the oidc-user cookie, or null.
   */
  getOIDCUserInfo(): OIDCUserInfo | null {
    try {
      const cookieValue = this.cookies_.get('oidc-user');
      if (!cookieValue) return null;
      const decoded = atob(cookieValue);
      return JSON.parse(decoded) as OIDCUserInfo;
    } catch {
      return null;
    }
  }

  /**
   * Returns the user's display name for the current auth mode.
   */
  getDisplayName(): string {
    if (this.isOIDCEnabled()) {
      const info = this.getOIDCUserInfo();
      return info?.displayName || info?.username || '';
    }
    return this._meService.getUserName() || '';
  }

  /**
   * Returns the user's email for the current auth mode.
   */
  getEmail(): string {
    if (this.isOIDCEnabled()) {
      const info = this.getOIDCUserInfo();
      return info?.email || '';
    }
    return '';
  }

  /**
   * Returns the user's avatar URL for the current auth mode.
   */
  getAvatarURL(): string {
    if (this.isOIDCEnabled()) {
      const info = this.getOIDCUserInfo();
      return info?.avatarUrl || '';
    }
    return '';
  }

  /**
   * Initiates the OIDC login flow.
   * Returns a login response that may contain a redirect URL.
   */
  loginWithOIDC(): Observable<OIDCLoginResponse> {
    return this.http_.get<OIDCLoginResponse>('api/v1/oidc/login');
  }

  /**
   * Refreshes the OIDC token using the refresh token stored in the encrypted
   * session cookie. Returns an observable that completes when the refresh is done.
   */
  refreshOIDCToken(): Observable<OIDCLoginResponse> {
    return this.http_.post<OIDCLoginResponse>('api/v1/oidc/refresh', {});
  }

  /**
   * Logs out from OIDC, clearing the session on both client and server.
   */
  logoutOIDC(): Observable<any> {
    return this.http_.post<any>('api/v1/oidc/logout', {});
  }

  /**
   * Sends a login request to the backend with filled in login spec structure.
   */
  login(loginSpec: LoginSpec): Observable<User> {
    return this.csrfTokenService_
      .getTokenForAction('login')
      .pipe(
        switchMap((csrfToken: CsrfToken) =>
          this.http_.post<AuthResponse>('api/v1/login', loginSpec, {
            headers: new HttpHeaders().set(this.config_.csrfHeaderName, csrfToken.token),
          })
        )
      )
      .pipe(
        switchMap((authResponse: AuthResponse) => {
          if (authResponse.token.length !== 0) {
            this.setTokenCookie_(authResponse.token);
          }

          return this._meService.refresh();
        })
      );
  }

  logout(): void {
    this.removeTokenCookie();
    this._meService.reset();
    this.router_.navigate(['login']);
  }

  /**
   * Performs a full logout including OIDC if configured.
   */
  fullLogout(): void {
    if (this.isOIDCEnabled()) {
      this.logoutOIDC().subscribe({
        next: () => this.completeLogout_(),
        error: () => this.completeLogout_(),
      });
    } else {
      this.completeLogout_();
    }
  }

  private completeLogout_(): void {
    this.removeTokenCookie();
    this._meService.reset();
    this.router_.navigate(['login']);
  }

  /**
   * Refreshes the authentication token. For OIDC mode, this uses the refresh
   * token stored in the server-side encrypted session cookie to obtain new
   * tokens. For token mode, this is a no-op (token-based auth doesn't auto-refresh).
   *
   * Called on every navigation and every 10 minutes via interval.
   */
  refreshToken(): void {
    // Only refresh in OIDC mode and when already authenticated
    if (!this.isOIDCEnabled()) return;
    if (!this.hasTokenCookie() && !this.getOIDCUserInfo()) return;

    // Prevent concurrent refresh attempts
    if (this._refreshInProgress) return;
    this._refreshInProgress = true;

    this.refreshOIDCToken()
      .pipe(take(1))
      .subscribe({
        next: () => {
          // Refresh succeeded — the server updated all cookies (session, token,
          // oidc-user, csrf-token). Re-initialize the CSRF token from the
          // updated cookie into sessionStorage.
          this.initializeCsrfToken();
        },
        error: () => {
          // Refresh failed — the session may have expired. The next API call
          // will return 401 and the GlobalErrorHandler will redirect to login.
        },
      })
      .add(() => {
        this._refreshInProgress = false;
      });
  }

  isAuthenticated(): boolean {
    return this._meService.getUser().authenticated || this.hasTokenCookie();
  }

  hasAuthHeader(): boolean {
    return this._meService.getUser().authenticated && !this.hasTokenCookie();
  }

  private getTokenCookie(): string {
    return this.cookies_.get(this.config_.authTokenCookieName) || '';
  }

  hasTokenCookie(): boolean {
    return !isEmpty(this.getTokenCookie());
  }

  private setTokenCookie_(token: string): void {
    if (this.isCurrentProtocolSecure_()) {
      this.cookies_.set(this.config_.authTokenCookieName, token, null, null, null, true, 'Strict');
      return;
    }

    if (this.isCurrentDomainSecure_()) {
      this.cookies_.set(this.config_.authTokenCookieName, token, null, null, location.hostname, false, 'Strict');
    }
  }

  removeTokenCookie(): void {
    if (this.isCurrentProtocolSecure_()) {
      this.cookies_.delete(this.config_.authTokenCookieName, null, null, true, 'Strict');
      return;
    }

    if (this.isCurrentDomainSecure_()) {
      this.cookies_.delete(this.config_.authTokenCookieName, null, location.hostname, false, 'Strict');
    }
  }

  private isCurrentDomainSecure_(): boolean {
    return ['localhost', '127.0.0.1'].indexOf(location.hostname) > -1;
  }

  private isCurrentProtocolSecure_(): boolean {
    return location.protocol.includes('https');
  }
}
