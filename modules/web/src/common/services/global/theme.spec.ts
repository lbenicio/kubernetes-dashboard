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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {ThemeService} from './theme';

describe('ThemeService', () => {
  it('filters duplicate custom themes, follows system changes, and notifies subscribers', () => {
    let onMediaChange: ((event: {matches: boolean}) => void) | undefined;
    const matchMedia = jest.fn().mockReturnValue({
      matches: true,
      addEventListener: jest.fn((_event: string, listener: (event: {matches: boolean}) => void) => {
        onMediaChange = listener;
      }),
    });
    const documentStub = {defaultView: {matchMedia}};
    const config = {
      appConfig: {
        themes: [
          {name: 'kd-light-theme', displayName: 'Custom light', isDark: true},
          {name: 'solarized', displayName: 'Solarized', isDark: true},
        ],
      },
    };
    const service = new ThemeService(documentStub as never, config as never);
    const listener = jest.fn();
    service.subscribe(listener);

    service.init();

    expect(service.themes).toEqual([
      {name: 'kd-light-theme', displayName: 'Light', isDark: false},
      {name: 'kd-dark-theme', displayName: 'Dark', isDark: true},
      {name: 'solarized', displayName: 'Solarized', isDark: true},
    ]);
    expect(service.isThemeDark()).toBe(true);

    service.theme = 'solarized';
    expect(service.isThemeDark()).toBe(true);
    service.theme = 'unknown-theme';
    expect(service.isThemeDark()).toBe(false);

    service.theme = ThemeService.SystemTheme;
    expect(listener).toHaveBeenLastCalledWith('kd-dark-theme');
    onMediaChange?.({matches: false});
    expect(listener).toHaveBeenLastCalledWith('kd-light-theme');
  });

  it('uses the light system theme when the dark color scheme is not enabled', () => {
    const matchMedia = jest.fn().mockReturnValue({matches: false, addEventListener: jest.fn()});
    const service = new ThemeService({defaultView: {matchMedia}} as never, {appConfig: {themes: []}} as never);

    service.init();
    expect(service.isThemeDark()).toBe(false);
    service.theme = ThemeService.SystemTheme;
    expect(service.theme).toBe(ThemeService.SystemTheme);
  });
});
