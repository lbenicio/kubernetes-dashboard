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

import angularPresetFactories from 'jest-preset-angular/presets/index.js';

const {createEsmPreset} = angularPresetFactories;

const esmPreset = createEsmPreset({
  tsconfig: '<rootDir>/../tsconfig.spec.json',
});

const config = {
  ...esmPreset,
  rootDir: './src',
  coverageDirectory: '../../../.tmp',
  setupFilesAfterEnv: ["<rootDir>/test.base.ts"],
  transform: {
    '^.+\\.(ts|tsx|js|html|svg|mjs)$': [
      'jest-preset-angular', {
        tsconfig: '<rootDir>/../tsconfig.spec.json',
        useESM: true,
        stringifyContentPathRegex: '\\.(html|svg)$',
      }
    ]
  },
  moduleNameMapper: {
    ...esmPreset.moduleNameMapper,
    "^@api/(.*)$": "<rootDir>/typings/$1",
    "^@common/(.*)$": "<rootDir>/common/$1",
    "^@environments/(.*)$": "<rootDir>/environments/$1",
  },
  transformIgnorePatterns: [
    "/node_modules/(?!(.*))",
  ],
};

export default config;
