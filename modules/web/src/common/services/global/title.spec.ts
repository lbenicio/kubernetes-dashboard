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

import {TitleService} from './title';

describe('TitleService', () => {
  it('includes the cluster name after settings load', () => {
    const title = {setTitle: jest.fn()};
    const settings = {
      load: jest.fn((success: () => void) => success()),
      getClusterName: jest.fn().mockReturnValue('production'),
    };
    const service = new TitleService(title as never, settings as never);

    service.update();

    expect(service.clusterName).toBe('production');
    expect(title.setTitle).toHaveBeenCalledWith('production - Kubernetes Dashboard');
  });

  it('uses the standard title when settings cannot load', () => {
    const title = {setTitle: jest.fn()};
    const settings = {
      load: jest.fn((_success: () => void, failure: () => void) => failure()),
      getClusterName: jest.fn(),
    };
    const service = new TitleService(title as never, settings as never);

    service.update();

    expect(service.clusterName).toBe('');
    expect(title.setTitle).toHaveBeenCalledWith('Kubernetes Dashboard');
    expect(settings.getClusterName).not.toHaveBeenCalled();
  });
});
