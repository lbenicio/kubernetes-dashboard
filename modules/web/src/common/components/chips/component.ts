// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, OnInit} from '@angular/core';
import type {SimpleChanges} from '@angular/core';
import {MatDialog, MatDialogConfig} from '@angular/material/dialog';
import {StringMap} from '@api/root.shared';

import {GlobalSettingsService} from '../../services/global/globalsettings';
import {ChipDialogComponent} from './chipdialog/dialog';
import {cropUrl} from './crop-url';
import {KdStateService} from '@common/services/global/state';

export interface Chip {
  key: string;
  value: string;
}

const MAX_CHIP_VALUE_LENGTH = 63;

@Component({
  selector: 'kd-chips',
  styleUrls: ['./style.scss'],
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class ChipsComponent implements OnInit, OnChanges {
  @Input() map: StringMap | string[] | number[];
  @Input() displayAll = false;
  keys: string[];
  isShowingAll = false;
  private _labelsLimit = 3;

  constructor(
    private readonly _kdStateService: KdStateService,
    private readonly _globalSettingsService: GlobalSettingsService,
    private readonly _matDialog: MatDialog,
    private readonly _changeDetectorRef: ChangeDetectorRef
  ) {}

  ngOnInit(): void {
    this._labelsLimit = this._globalSettingsService.getLabelsLimit();
    this.processMap();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes.map) {
      this.processMap();
    }
  }

  isVisible(index: number): boolean {
    return this.isShowingAll || index < this._labelsLimit || this.displayAll;
  }

  isAnythingHidden(): boolean {
    return this.keys.length > this._labelsLimit && !this.displayAll;
  }

  toggleView(): void {
    this.isShowingAll = !this.isShowingAll;
  }

  isTooLong(value: string): boolean {
    return value !== undefined && value.length > MAX_CHIP_VALUE_LENGTH;
  }

  getTruncatedURL(url: string): string {
    return cropUrl(url, MAX_CHIP_VALUE_LENGTH);
  }

  isHref(value: string): boolean {
    const trimmedValue = value.trim();
    if (/\s/.test(trimmedValue)) {
      return false;
    }

    let url: URL;
    try {
      url = new URL(trimmedValue);
    } catch {
      return false;
    }

    if (!['http:', 'https:', 'ftp:'].includes(url.protocol) || !this.isAllowedHostname(url.hostname)) {
      return false;
    }

    const schemeEnd = trimmedValue.indexOf('://');
    const authorityStart = schemeEnd + 3;
    let authorityEnd = trimmedValue.length;
    for (const delimiter of ['/', '?', '#']) {
      const delimiterIndex = trimmedValue.indexOf(delimiter, authorityStart);
      if (delimiterIndex >= 0 && delimiterIndex < authorityEnd) {
        authorityEnd = delimiterIndex;
      }
    }

    const authority = trimmedValue.slice(authorityStart, authorityEnd);
    const hostAndPort = authority.slice(authority.lastIndexOf('@') + 1);
    const portSeparator = hostAndPort.lastIndexOf(':');
    if (portSeparator >= 0 && !/^\d{2,5}$/.test(hostAndPort.slice(portSeparator + 1))) {
      return false;
    }

    return true;
  }

  private isAllowedHostname(hostname: string): boolean {
    const normalizedHostname = hostname.endsWith('.') ? hostname.slice(0, -1) : hostname;
    const labels = normalizedHostname.split('.');

    if (labels.length === 4 && labels.every(label => /^\d+$/.test(label))) {
      const octets = labels.map(Number);
      const [first, second, , last] = octets;

      return (
        octets.every(octet => octet >= 0 && octet <= 255) &&
        first >= 1 &&
        first <= 223 &&
        first !== 10 &&
        first !== 127 &&
        !(first === 169 && second === 254) &&
        !(first === 192 && second === 168) &&
        !(first === 172 && second >= 16 && second <= 31) &&
        last >= 1 &&
        last <= 254
      );
    }

    if (/^[\d.]+$/.test(normalizedHostname) || labels.length < 2) {
      return false;
    }

    const topLevelDomain = labels[labels.length - 1];
    return (
      labels.every(label => /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/i.test(label)) &&
      (/^[a-z]{2,}$/i.test(topLevelDomain) || /^xn--[a-z0-9-]{2,}$/i.test(topLevelDomain))
    );
  }

  isSerializedRef(value: string): boolean {
    try {
      const obj = JSON.parse(value);
      return obj && obj.kind === 'SerializedReference';
    } catch (e) {
      return false;
    }
  }

  getSerializedHref(value: string): string {
    const ref = JSON.parse(value);
    if (!ref.reference || !ref.reference.kind || !ref.reference.name || !ref.reference.namespace) {
      return '';
    }

    return this._kdStateService.href(ref.reference.kind.toLowerCase(), ref.reference.name, ref.reference.namespace);
  }

  getSerializedRefDisplayName(value: string): string {
    const ref = JSON.parse(value);
    if (!ref.reference || !ref.reference.kind || !ref.reference.name || !ref.reference.namespace) {
      return 'Invalid reference';
    }

    return `${ref.reference.kind.replace(/([A-Z])/g, ' $1')} ${ref.reference.namespace}/${ref.reference.name}`;
  }

  openChipDialog(key: string, value: string): void {
    const dialogConfig: MatDialogConfig<Chip> = {
      width: '630px',
      data: {
        key,
        value,
      },
    };
    this._matDialog.open(ChipDialogComponent, dialogConfig);
  }

  private processMap() {
    if (!this.map) {
      this.map = [];
    }

    if (Array.isArray(this.map)) {
      this.keys = this.map as string[];
    } else {
      this.keys = Object.keys(this.map);
    }
    this._changeDetectorRef.markForCheck();
  }
}
