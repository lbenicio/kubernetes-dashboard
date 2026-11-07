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

import {ActionbarService, ResourceMeta} from './actionbar';

describe('ActionbarService', () => {
  it('publishes resource metadata and detail navigation events', () => {
    const service = new ActionbarService();
    const onInit = jest.fn();
    const onLeave = jest.fn();
    const metadata = new ResourceMeta('Pod', {name: 'web'} as never, {kind: 'pod'} as never, true);

    service.onInit.subscribe(onInit);
    service.onDetailsLeave.subscribe(onLeave);
    service.onInit.emit(metadata);
    service.onDetailsLeave.emit();

    expect(metadata).toMatchObject({
      displayName: 'Pod',
      objectMeta: {name: 'web'},
      typeMeta: {kind: 'pod'},
      namespaced: true,
    });
    expect(onInit).toHaveBeenCalledWith(metadata);
    expect(onLeave).toHaveBeenCalled();
  });
});
