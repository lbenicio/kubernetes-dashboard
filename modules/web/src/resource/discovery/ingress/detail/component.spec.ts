// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {TestBed} from '@angular/core/testing';
import {IngressDetail} from '@api/root.api';
import {of} from 'rxjs';
import {IngressDetailComponent} from './component';

describe('IngressDetailComponent', () => {
  beforeEach(() => TestBed.configureTestingModule({}));

  it('loads ingress details, builds its event endpoint, and leaves the action bar on destroy', () => {
    const detail = {
      objectMeta: {name: 'public', namespace: 'default'},
      typeMeta: {kind: 'Ingress'},
      errors: [],
    } as unknown as IngressDetail;
    const ingress = {get: jest.fn().mockReturnValue(of(detail))};
    const actionbar = {
      onInit: {emit: jest.fn()},
      onDetailsLeave: {emit: jest.fn()},
    };
    const route = {snapshot: {params: {resourceName: 'public', resourceNamespace: 'default'}}};
    const notifications = {pushErrors: jest.fn()};
    const component = TestBed.runInInjectionContext(
      () => new IngressDetailComponent(ingress as never, actionbar as never, route as never, notifications as never)
    );

    component.ngOnInit();

    expect(ingress.get).toHaveBeenCalledWith('api/v1/ingress/:namespace/:name', 'public', 'default');
    expect(component.eventListEndpoint).toBe('api/v1/ingress/default/public/event');
    expect(component.ingress).toBe(detail);
    expect(component.isInitialized).toBe(true);
    expect(notifications.pushErrors).toHaveBeenCalledWith([]);
    expect(actionbar.onInit.emit).toHaveBeenCalledWith(expect.objectContaining({displayName: 'Ingress'}));

    component.ngOnDestroy();
    expect(actionbar.onDetailsLeave.emit).toHaveBeenCalled();
  });
});
