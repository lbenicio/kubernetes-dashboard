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

import {HttpClient} from '@angular/common/http';
import {Component, Inject, OnInit, ChangeDetectionStrategy} from '@angular/core';
import {
  MAT_DIALOG_DATA,
  MatDialogRef,
  MatDialogTitle,
  MatDialogContent,
  MatDialogActions,
  MatDialogClose,
} from '@angular/material/dialog';
import {ReplicaCounts} from '@api/root.api';

import {ResourceMeta} from '../../services/global/actionbar';
import {firstValueFrom} from 'rxjs';
import {CdkScrollable} from '@angular/cdk/scrolling';
import {MatFormField} from '@angular/material/form-field';
import {FormsModule} from '@angular/forms';
import {MatInput} from '@angular/material/input';
import {MatIcon} from '@angular/material/icon';
import {MatButton} from '@angular/material/button';

@Component({
  selector: 'kd-delete-resource-dialog',
  templateUrl: 'template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    MatDialogTitle,
    CdkScrollable,
    MatDialogContent,
    MatFormField,
    FormsModule,
    MatInput,
    MatIcon,
    MatDialogActions,
    MatButton,
    MatDialogClose,
  ],
})
export class ScaleResourceDialogComponent implements OnInit {
  actual = 0;
  desired = 0;

  constructor(
    public dialogRef: MatDialogRef<ScaleResourceDialogComponent>,
    @Inject(MAT_DIALOG_DATA) public data: ResourceMeta,
    private readonly http_: HttpClient
  ) {}

  ngOnInit(): void {
    const url =
      `api/v1/scale/${this.data.typeMeta.kind}` +
      (this.data.objectMeta.namespace ? `/${this.data.objectMeta.namespace}` : '') +
      `/${this.data.objectMeta.name}`;

    firstValueFrom(this.http_.get<ReplicaCounts>(url)).then(rc => {
      this.actual = rc.actualReplicas;
      this.desired = rc.desiredReplicas;
    });
  }

  onNoClick(): void {
    this.dialogRef.close();
  }
}
