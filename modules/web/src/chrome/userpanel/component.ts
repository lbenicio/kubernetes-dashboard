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

import {Component, ViewChild} from '@angular/core';
import {MatMenuTrigger} from '@angular/material/menu';
import {AuthService} from '@common/services/global/authentication';
import {MeService} from '@common/services/global/me';

@Component({
  selector: 'kd-user-panel',
  templateUrl: './template.html',
  styleUrls: ['./style.scss'],
})
export class UserPanelComponent {
  @ViewChild(MatMenuTrigger)
  private readonly trigger_: MatMenuTrigger;

  constructor(
    private readonly authService_: AuthService,
    private readonly _meService: MeService
  ) {}

  get username(): string {
    return this._meService.getUserName();
  }

  get displayName(): string {
    return this.authService_.getDisplayName();
  }

  get email(): string {
    return this.authService_.getEmail();
  }

  get avatarUrl(): string {
    return this.authService_.getAvatarURL();
  }

  get hasAvatar(): boolean {
    return !!this.avatarUrl;
  }

  get isOIDC(): boolean {
    return this.authService_.isOIDCEnabled();
  }

  hasAuthHeader(): boolean {
    return this.authService_.hasAuthHeader();
  }

  hasTokenCookie(): boolean {
    return this.authService_.hasTokenCookie();
  }

  isAuthenticated(): boolean {
    return this.authService_.isAuthenticated();
  }

  logout(): void {
    this.authService_.fullLogout();
  }

  close(): void {
    this.trigger_.closeMenu();
  }
}
