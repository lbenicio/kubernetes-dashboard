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

import {K8sError} from '@api/root.api';
import {Notification, NotificationSeverity, NotificationsService} from './notifications';

describe('NotificationsService', () => {
  it('creates styled notifications, ignores duplicate messages, and updates unread state', () => {
    const service = new NotificationsService({getDisableAccessDeniedNotifications: () => false} as never);

    service.push('Saved', NotificationSeverity.info);
    service.push('Saved', NotificationSeverity.error);
    service.push('Warning', NotificationSeverity.warning);
    service.push('Failure', NotificationSeverity.error);

    expect(service.getNotifications()).toHaveLength(3);
    expect(service.getNotifications()[0]).toMatchObject({message: 'Failure', icon: 'error', cssClass: 'kd-error'});
    expect(new Notification('Info', NotificationSeverity.info).cssClass).toBe('kd-success');
    expect(new Notification('Warning', NotificationSeverity.warning).cssClass).toBe('kd-warning');
    expect(new Notification('Custom', 'custom' as NotificationSeverity).cssClass).toBe('');
    expect(service.getUnreadCount()).toBe(3);

    service.getNotifications()[0].read = true;
    expect(service.getUnreadCount()).toBe(2);

    service.markAllAsRead();
    expect(service.getUnreadCount()).toBe(0);

    service.remove(1);
    expect(service.getNotifications()).toHaveLength(2);

    service.clear();
    expect(service.getNotifications()).toEqual([]);
  });

  it('filters access-denied errors only when the setting disables them', () => {
    let disableAccessDenied = true;
    const service = new NotificationsService({
      getDisableAccessDeniedNotifications: () => disableAccessDenied,
    } as never);
    const forbidden = {ErrStatus: {code: 403, message: 'forbidden'}} as K8sError;
    const internal = {ErrStatus: {code: 500, message: 'internal'}} as K8sError;

    service.pushErrors([forbidden, internal]);
    expect(service.getNotifications().map(notification => notification.message)).toEqual(['internal']);

    disableAccessDenied = false;
    service.pushErrors([forbidden]);
    expect(service.getNotifications().map(notification => notification.message)).toEqual(['forbidden', 'internal']);

    service.pushErrors(null as never);
    expect(service.getNotifications()).toHaveLength(2);
  });
});
