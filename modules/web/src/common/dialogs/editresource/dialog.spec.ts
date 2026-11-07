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

import {TestBed} from '@angular/core/testing';
import {Subject, of} from 'rxjs';
import {EditorMode} from '../../components/textinput/component';
import {EditResourceDialogComponent} from './dialog';
import {load as fromYaml} from 'js-yaml';

describe('EditResourceDialogComponent', () => {
  beforeEach(() => TestBed.configureTestingModule({}));

  it('loads YAML, switches between YAML and JSON, and serializes the edited resource', async () => {
    const dialogRef = {close: jest.fn()};
    const data = {
      typeMeta: {kind: 'pod'},
      objectMeta: {name: 'web', namespace: 'default'},
    };
    const http = {get: jest.fn().mockReturnValue(of({apiVersion: 'v1', kind: 'Pod'}))};
    const valueChange = new Subject<EditorMode>();
    const buttonGroup = {value: EditorMode.YAML, valueChange};
    const component = TestBed.runInInjectionContext(
      () => new EditResourceDialogComponent(dialogRef as never, data as never, http as never)
    );
    component.buttonToggleGroup = buttonGroup as never;

    component.ngOnInit();
    await Promise.resolve();

    expect(http.get).toHaveBeenCalledWith('api/v1/_raw/pod/namespace/default/name/web');
    expect(fromYaml(component.text)).toEqual({apiVersion: 'v1', kind: 'Pod'});
    expect(component.getSelectedMode()).toBe(EditorMode.YAML);
    expect(JSON.parse(component.getJSON())).toEqual({apiVersion: 'v1', kind: 'Pod'});

    buttonGroup.value = EditorMode.JSON;
    valueChange.next(EditorMode.JSON);
    expect(component.selectedMode).toBe(EditorMode.JSON);
    expect(JSON.parse(component.text)).toEqual({apiVersion: 'v1', kind: 'Pod'});
    expect(component.getJSON()).toBe(component.text);

    buttonGroup.value = EditorMode.YAML;
    valueChange.next(EditorMode.YAML);
    expect(component.selectedMode).toBe(EditorMode.YAML);
    expect(fromYaml(component.text)).toEqual({apiVersion: 'v1', kind: 'Pod'});

    component.onNoClick();
    expect(dialogRef.close).toHaveBeenCalled();
  });

  it('does not rewrite empty editor content when the mode changes', () => {
    const valueChange = new Subject<EditorMode>();
    const component = TestBed.runInInjectionContext(
      () =>
        new EditResourceDialogComponent(
          {close: jest.fn()} as never,
          {
            typeMeta: {kind: 'pod'},
            objectMeta: {name: 'web', namespace: 'default'},
          } as never,
          {get: jest.fn().mockReturnValue(of({}))} as never
        )
    );
    component.buttonToggleGroup = {value: EditorMode.JSON, valueChange} as never;
    component.text = '';
    component.ngOnInit();

    valueChange.next(EditorMode.JSON);

    expect(component.selectedMode).toBe(EditorMode.JSON);
    expect(component.text).toBe('');
  });
});
