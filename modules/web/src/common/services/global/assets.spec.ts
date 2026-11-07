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

import {AssetsService} from './assets';

describe('AssetsService', () => {
  it('registers the application and pin icons and exposes application icon names', () => {
    const registry = {addSvgIcon: jest.fn()};
    const sanitizer = {bypassSecurityTrustResourceUrl: jest.fn((url: string) => 'safe:' + url)};
    const service = new AssetsService(registry as never, sanitizer as never);

    expect(registry.addSvgIcon).toHaveBeenCalledTimes(4);
    expect(registry.addSvgIcon).toHaveBeenCalledWith('kd-logo', 'safe:assets/images/kubernetes-logo.svg');
    expect(registry.addSvgIcon).toHaveBeenCalledWith('kd-logo-text', 'safe:assets/images/kubernetes-logo-text.svg');
    expect(registry.addSvgIcon).toHaveBeenCalledWith('pin', 'safe:assets/images/pin.svg');
    expect(registry.addSvgIcon).toHaveBeenCalledWith('pin-crossed', 'safe:assets/images/pin-crossed.svg');
    expect(service.getAppLogo()).toBe('kd-logo');
    expect(service.getAppLogoText()).toBe('kd-logo-text');
  });
});
