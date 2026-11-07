// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

import {AlertDialogComponent, AlertDialogConfig} from './dialog';

describe('AlertDialogComponent', () => {
  it('keeps the supplied message and closes when cancelled', () => {
    const data: AlertDialogConfig = {title: 'Warning', message: 'Continue?', confirmLabel: 'Continue'};
    const close = jest.fn();
    const component = new AlertDialogComponent({close} as never, data);

    expect(component.data).toBe(data);
    component.onNoClick();
    expect(close).toHaveBeenCalled();
  });
});
