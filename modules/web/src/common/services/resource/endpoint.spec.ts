// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {EndpointManager, Resource, Utility} from './endpoint';

describe('EndpointManager', () => {
  it('builds cluster-scoped resource endpoints', () => {
    const endpoint = EndpointManager.resource(Resource.node);

    expect(endpoint.list()).toBe('api/v1/node');
    expect(endpoint.detail()).toBe('api/v1/node/:name');
    expect(endpoint.child('worker-1', Resource.pod)).toBe('api/v1/node/worker-1/pod');
  });

  it('builds namespaced resource endpoints and children', () => {
    const endpoint = EndpointManager.resource(Resource.service, true);

    expect(endpoint.list()).toBe('api/v1/service/:namespace');
    expect(endpoint.detail()).toBe('api/v1/service/:namespace/:name');
    expect(endpoint.child('frontend', Resource.pod)).toBe('api/v1/service/:namespace/frontend/pod');
    expect(endpoint.child('frontend', Resource.event, 'default')).toBe('api/v1/service/default/frontend/event');
  });

  it('builds utility endpoints', () => {
    expect(EndpointManager.utility(Utility.shell).shell('default', 'web-0')).toBe('api/v1/pod/default/web-0/shell');
  });
});
