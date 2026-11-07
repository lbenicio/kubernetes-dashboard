// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {DeleteResourceDialogComponent} from './dialog';

describe('DeleteResourceDialogComponent', () => {
  it('starts with a deferred background deletion', () => {
    const data = {displayName: 'Pod'} as never;
    const component = new DeleteResourceDialogComponent({} as never, data);

    expect(component.data).toBe(data);
    expect(component.options).toEqual({deleteNow: false, propagation: 'Background'});
  });
});
