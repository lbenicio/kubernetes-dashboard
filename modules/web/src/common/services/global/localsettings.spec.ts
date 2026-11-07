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

import {LocalSettingsService} from './localsettings';
import {ThemeService} from './theme';

describe('LocalSettingsService', () => {
  let service: LocalSettingsService;
  let theme: {theme: string};

  beforeEach(() => {
    localStorage.clear();
    theme = {theme: ''};
    service = new LocalSettingsService(theme as never);
  });

  it('starts with the system theme when no setting is stored', () => {
    service.init();

    expect(service.get()).toEqual({theme: ThemeService.SystemTheme});
  });

  it('loads stored settings and persists theme changes', () => {
    localStorage.setItem('localSettings', JSON.stringify({theme: 'kd-dark-theme'}));
    service.init();
    expect(service.get()).toEqual({theme: 'kd-dark-theme'});

    service.handleThemeChange('kd-light-theme');

    expect(service.get()).toEqual({theme: 'kd-light-theme'});
    expect(theme.theme).toBe('kd-light-theme');
    expect(localStorage.getItem('localSettings')).toBe(JSON.stringify({theme: 'kd-light-theme'}));
  });
});
