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

import {HttpErrorResponse} from '@angular/common/http';
import {ObjectMeta, TypeMeta} from '@api/root.api';
import {of, throwError} from 'rxjs';
import {VerberService} from './verber';

describe('VerberService', () => {
  const typeMeta = {kind: 'pod'} as TypeMeta;
  const objectMeta = {name: 'web', namespace: 'default'} as ObjectMeta;

  it('builds resource dialog data and HTTP headers', () => {
    const service = new VerberService({open: jest.fn()} as never, {} as never);

    expect(service.getDialogConfig_('Pod', typeMeta, objectMeta)).toEqual({
      width: '900px',
      data: {displayName: 'Pod', typeMeta, objectMeta},
    });
    expect(service.getHttpHeaders_()).toBeTruthy();
  });

  it('deletes a resource when the delete dialog is confirmed and ignores cancellation', () => {
    const dialogRef = {afterClosed: jest.fn().mockReturnValue(of({deleteNow: true, propagation: 'Foreground'}))};
    const dialog = {open: jest.fn().mockReturnValue(dialogRef)};
    const http = {delete: jest.fn().mockReturnValue(of('deleted'))};
    const service = new VerberService(dialog as never, http as never);
    const onDelete = jest.fn();
    service.onDelete.subscribe(onDelete);

    service.showDeleteDialog('Pod', typeMeta, objectMeta);

    expect(http.delete).toHaveBeenCalledWith('api/v1/_raw/pod/namespace/default/name/web', {
      params: {deleteNow: true, propagation: 'Foreground'},
      responseType: 'text',
    });
    expect(onDelete).toHaveBeenCalledWith(true);

    dialog.open.mockReturnValue({afterClosed: () => of(false)});
    service.showDeleteDialog('Pod', typeMeta, objectMeta);
    expect(http.delete).toHaveBeenCalledTimes(1);
  });

  it('edits a resource and emits after the update succeeds', () => {
    const dialog = {open: jest.fn().mockReturnValue({afterClosed: () => of('{"metadata":{"name":"web"}}')})};
    const http = {put: jest.fn().mockReturnValue(of('updated'))};
    const service = new VerberService(dialog as never, http as never);
    const onEdit = jest.fn();
    service.onEdit.subscribe(onEdit);

    service.showEditDialog('Pod', typeMeta, objectMeta);

    expect(http.put).toHaveBeenCalledWith(
      'api/v1/_raw/pod/namespace/default/name/web',
      {metadata: {name: 'web'}},
      expect.objectContaining({headers: expect.anything(), responseType: 'text'})
    );
    expect(onEdit).toHaveBeenCalledWith(true);
  });

  it('restarts, scales, and triggers resources through their API endpoints', () => {
    const dialog = {open: jest.fn().mockReturnValue({afterClosed: () => of(true)})};
    const http = {put: jest.fn().mockReturnValue(of('updated'))};
    const service = new VerberService(dialog as never, http as never);
    const onTrigger = jest.fn();
    const onScale = jest.fn();
    service.onTrigger.subscribe(onTrigger);
    service.onScale.subscribe(onScale);

    service.showRestartDialog('Pod', typeMeta, objectMeta);
    expect(http.put).toHaveBeenCalledWith('api/v1/pod/default/web/restart', {responseType: 'text'});
    expect(onTrigger).toHaveBeenCalledWith(true);

    dialog.open.mockReturnValue({afterClosed: () => of(3)});
    service.showScaleDialog('Pod', typeMeta, objectMeta);
    expect(http.put).toHaveBeenCalledWith('api/v1/scale/pod/default/web', {scaleBy: 3}, {params: {scaleBy: 3}});
    expect(onScale).toHaveBeenCalledWith(true);

    dialog.open.mockReturnValue({afterClosed: () => of(true)});
    service.showTriggerDialog('CronJob', {kind: 'cronjob'} as TypeMeta, objectMeta);
    expect(http.put).toHaveBeenLastCalledWith('api/v1/cronjob/default/web/trigger', {}, {responseType: 'text'});
    expect(onTrigger).toHaveBeenCalledTimes(2);
  });

  it('uses a cluster-scoped scale URL and shows alerts when a mutation fails', () => {
    const dialog = {open: jest.fn().mockReturnValue({afterClosed: () => of(2)})};
    const http = {put: jest.fn().mockReturnValue(throwError(() => new Error('scale failed')))};
    const service = new VerberService(dialog as never, http as never);

    service.showScaleDialog('Node', {kind: 'node'} as TypeMeta, {name: 'worker'} as ObjectMeta);

    expect(http.put).toHaveBeenCalledWith('api/v1/scale/node/worker', {scaleBy: 2}, {params: {scaleBy: 2}});

    service.handleErrorResponse_(
      new HttpErrorResponse({status: 500, statusText: 'Internal Server Error', error: 'failed'})
    );
    expect(dialog.open).toHaveBeenCalledWith(
      expect.any(Function),
      expect.objectContaining({
        width: '630px',
        data: {title: 'Internal Server Error', message: 'failed', confirmLabel: 'OK'},
      })
    );
  });
});
