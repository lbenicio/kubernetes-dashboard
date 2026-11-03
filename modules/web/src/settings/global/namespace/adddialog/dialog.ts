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

import {Component, Inject, ChangeDetectionStrategy} from '@angular/core';
import {
  MAT_DIALOG_DATA,
  MatDialogTitle,
  MatDialogContent,
  MatDialogActions,
  MatDialogClose,
} from '@angular/material/dialog';
import {CdkScrollable} from '@angular/cdk/scrolling';
import {MatFormField} from '@angular/material/form-field';
import {MatInput} from '@angular/material/input';
import {MatAutocompleteTrigger, MatAutocomplete} from '@angular/material/autocomplete';
import {MatOption} from '@angular/material/select';
import {MatButton} from '@angular/material/button';
import {FilterByPipe} from '@common/pipes/filterby';

export interface AddFallbackNamespaceDialogData {
  namespaces: string[];
}

@Component({
  selector: 'kd-namespace-settings-add-dialog',
  templateUrl: 'template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    MatDialogTitle,
    CdkScrollable,
    MatDialogContent,
    MatFormField,
    MatInput,
    MatAutocompleteTrigger,
    MatAutocomplete,
    MatOption,
    MatDialogActions,
    MatButton,
    MatDialogClose,
    FilterByPipe,
  ],
})
export class AddFallbackNamespaceDialogComponent {
  constructor(@Inject(MAT_DIALOG_DATA) public data: AddFallbackNamespaceDialogData) {}
}
