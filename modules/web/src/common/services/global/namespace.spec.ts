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

import {NamespaceService} from './namespace';
import {IConfig} from '@api/root.ui';

describe('NamespaceService', () => {
  it('uses the configured namespace until another namespace is selected', () => {
    const service = new NamespaceService({defaultNamespace: 'default'} as IConfig);

    expect(service.current()).toBe('default');
    expect(service.getDefaultNamespace()).toBe('default');
    expect(service.getAllNamespacesKey()).toBe('_all');

    service.setCurrent('kube-system');
    expect(service.current()).toBe('kube-system');
  });

  it('validates namespace names and detects multi-namespace selection', () => {
    const service = new NamespaceService({defaultNamespace: ''} as IConfig);

    expect(service.isNamespaceValid('default')).toBe(true);
    expect(service.isNamespaceValid('kube-system')).toBe(true);
    expect(service.isNamespaceValid('_all')).toBe(true);
    expect(service.isNamespaceValid('Uppercase')).toBe(false);
    expect(service.isNamespaceValid('trailing-')).toBe(false);
    expect(service.isNamespaceValid('')).toBe(false);
    expect(service.areMultipleNamespacesSelected()).toBe(true);

    service.setCurrent('_all');
    expect(service.isMultiNamespace(service.current())).toBe(true);
    expect(service.areMultipleNamespacesSelected()).toBe(true);

    service.setCurrent('default');
    expect(service.isMultiNamespace(service.current())).toBe(false);
    expect(service.areMultipleNamespacesSelected()).toBe(false);
  });
});
