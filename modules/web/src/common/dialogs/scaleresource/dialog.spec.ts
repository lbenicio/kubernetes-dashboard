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

import {ScaleResourceDialogComponent} from './dialog';
import {of} from 'rxjs';

describe('ScaleResourceDialogComponent', () => {
  it('loads replica counts for a namespaced resource', async () => {
    const http = {get: jest.fn().mockReturnValue(of({actualReplicas: 2, desiredReplicas: 5}))};
    const component = new ScaleResourceDialogComponent(
      {close: jest.fn()} as never,
      {
        typeMeta: {kind: 'deployment'},
        objectMeta: {name: 'web', namespace: 'default'},
      } as never,
      http as never
    );

    component.ngOnInit();
    await Promise.resolve();

    expect(http.get).toHaveBeenCalledWith('api/v1/scale/deployment/default/web');
    expect(component.actual).toBe(2);
    expect(component.desired).toBe(5);
  });

  it('loads replica counts for a cluster-scoped resource and closes without changes', async () => {
    const close = jest.fn();
    const http = {get: jest.fn().mockReturnValue(of({actualReplicas: 1, desiredReplicas: 1}))};
    const component = new ScaleResourceDialogComponent(
      {close} as never,
      {typeMeta: {kind: 'node'}, objectMeta: {name: 'worker'}} as never,
      http as never
    );

    component.ngOnInit();
    await Promise.resolve();
    component.onNoClick();

    expect(http.get).toHaveBeenCalledWith('api/v1/scale/node/worker');
    expect(component.actual).toBe(1);
    expect(component.desired).toBe(1);
    expect(close).toHaveBeenCalled();
  });
});
