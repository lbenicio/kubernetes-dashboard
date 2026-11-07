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

import type {SimpleChange, SimpleChanges} from '@angular/core';
import {EditorMode, TextInputComponent} from './component';

describe('TextInputComponent', () => {
  let component: TextInputComponent;
  let themeService: {isThemeDark: jest.Mock};

  beforeEach(() => {
    themeService = {isThemeDark: jest.fn().mockReturnValue(false)};
    component = new TextInputComponent(themeService as never);
    component.text = '{"message":"hello"}';
  });

  it('selects the editor theme from the theme service', () => {
    component.ngOnInit();
    expect(component.theme).toBe('textmate');

    themeService.isThemeDark.mockReturnValue(true);
    component.ngOnInit();
    expect(component.theme).toBe('idle_fingers');
  });

  it('emits text changes', () => {
    const listener = jest.fn();
    component.textChange.subscribe(listener);

    component.onTextChange('updated');

    expect(listener).toHaveBeenCalledWith('updated');
  });

  it('initializes Ace, pretty prints JSON, and forwards editor changes', () => {
    const editorChange = {callback: undefined as (() => void) | undefined};
    const editor = {
      setOptions: jest.fn(),
      setValue: jest.fn(),
      setReadOnly: jest.fn(),
      setTheme: jest.fn(),
      getValue: jest.fn().mockReturnValue('{"message":"from editor"}'),
      getCursorPosition: jest.fn().mockReturnValue({row: 1, column: 2}),
      moveCursorToPosition: jest.fn(),
      on: jest.fn((_event: string, callback: () => void) => (editorChange.callback = callback)),
      session: {setUseWorker: jest.fn(), setMode: jest.fn()},
    };
    const ace = {config: {set: jest.fn()}, edit: jest.fn().mockReturnValue(editor)};
    const previousAce = window.ace;
    window.ace = ace as never;
    component.editorRef = {nativeElement: document.createElement('div')} as never;
    component.mode = EditorMode.JSON;
    component.readOnly = true;
    component.theme = 'textmate';
    const listener = jest.fn();
    component.textChange.subscribe(listener);

    component.ngAfterViewInit();

    expect(ace.config.set).toHaveBeenCalledWith('basePath', 'ace');
    expect(ace.edit).toHaveBeenCalledWith(component.editorRef.nativeElement);
    expect(component.text).toBe('{\n\t"message": "hello"\n}');
    expect(editor.setValue).toHaveBeenCalledWith(component.text, -1);
    expect(editor.setReadOnly).toHaveBeenCalledWith(true);
    expect(editor.setTheme).toHaveBeenCalledWith('ace/theme/textmate');
    expect(editor.session.setMode).toHaveBeenCalledWith('ace/mode/json');
    expect(editor.session.setUseWorker).toHaveBeenCalledWith(false);

    editorChange.callback?.();
    expect(component.text).toBe('{"message":"from editor"}');
    expect(listener).toHaveBeenCalledWith('{"message":"from editor"}');

    window.ace = previousAce;
  });

  it('updates external text while preserving the cursor position and handles mode changes', () => {
    const editor = {
      getCursorPosition: jest.fn().mockReturnValue({row: 3, column: 4}),
      setValue: jest.fn(),
      moveCursorToPosition: jest.fn(),
      session: {setMode: jest.fn()},
      setTheme: jest.fn(),
    };
    component.editor = editor as never;
    component.mode = EditorMode.JSON;
    component.text = '{"value":2}';

    component.ngOnChanges({
      text: {} as SimpleChange,
      mode: {} as SimpleChange,
      readOnly: {} as SimpleChange,
    } as SimpleChanges);

    expect(component.text).toBe('{\n\t"value": 2\n}');
    expect(editor.setValue).toHaveBeenCalledWith(component.text, -1);
    expect(editor.moveCursorToPosition).toHaveBeenCalledWith({row: 3, column: 4});
    expect(editor.session.setMode).toHaveBeenCalledWith('ace/mode/json');
  });

  it('ignores changes until the editor exists', () => {
    expect(() => component.ngOnChanges({text: {} as SimpleChange})).not.toThrow();
  });

  it('does not prettify YAML, invalid JSON, or content when prettification is disabled', () => {
    const editor = {
      getCursorPosition: jest.fn().mockReturnValue({row: 0, column: 0}),
      setValue: jest.fn(),
      moveCursorToPosition: jest.fn(),
    };
    component.editor = editor as never;
    component.mode = EditorMode.YAML;
    component.text = 'key: value';
    component.ngOnChanges({text: {} as SimpleChange});
    expect(component.text).toBe('key: value');

    component.mode = EditorMode.JSON;
    component.text = '{invalid';
    expect(() => component.ngOnChanges({text: {} as SimpleChange})).not.toThrow();
    expect(component.text).toBe('{invalid');

    component.prettify = false;
    component.text = '{"compact":true}';
    component.ngOnChanges({text: {} as SimpleChange});
    expect(component.text).toBe('{"compact":true}');
  });
});
