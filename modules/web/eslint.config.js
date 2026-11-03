// Copyright 2017 The Kubernetes Authors.
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

import eslint from '@eslint/js';
import angular from 'angular-eslint';
import eslintConfigPrettier from 'eslint-config-prettier/flat';
import eslintN from 'eslint-plugin-n';
import prettier from 'eslint-plugin-prettier';
import rxjs from 'eslint-plugin-rxjs-x';
import tseslint from 'typescript-eslint';

const tsOnly = config => ({...config, files: ['**/*.ts']});
const tsConfigs = configs => (Array.isArray(configs) ? configs : [configs]).map(tsOnly);

export default [
  {ignores: ['**/*.js', '**/*.mjs', '**/*.cjs', '**/node_modules/**', '**/.yarn/**']},
  tsOnly(eslint.configs.recommended),
  ...tsConfigs(tseslint.configs.recommended),
  ...tsConfigs(angular.configs.tsRecommended),
  ...tsConfigs(eslintN.configs['flat/recommended']),
  tsOnly(eslintConfigPrettier),
  {
    files: ['**/*.ts'],
    languageOptions: {
      parserOptions: {
        project: ['./tsconfig.app.json', './tsconfig.spec.json', './cypress/tsconfig.json'],
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      node: eslintN,
      prettier,
      'rxjs-x': rxjs,
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        {argsIgnorePattern: '^_', caughtErrors: 'none'},
      ],
      '@typescript-eslint/no-unused-expressions': ['error', {allowTernary: true}],
      '@typescript-eslint/no-unsafe-function-type': 'off',
      '@typescript-eslint/ban-ts-comment': 'warn',
      '@typescript-eslint/no-non-null-assertion': 'off',
      '@typescript-eslint/no-use-before-define': 'off',
      '@typescript-eslint/no-warning-comments': 'off',
      '@typescript-eslint/no-empty-function': 'off',
      '@typescript-eslint/no-var-requires': 'off',
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-namespace': 'off',
      '@typescript-eslint/no-this-alias': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      '@angular-eslint/no-host-metadata-property': 'off',
      '@angular-eslint/prefer-inject': 'off',
      '@angular-eslint/prefer-standalone': 'off',
      '@angular-eslint/prefer-on-push-component-change-detection': 'off',
      'node/no-missing-import': 'off',
      'node/no-empty-function': 'off',
      'node/no-unpublished-import': 'off',
      'n/no-missing-import': 'off',
      'n/no-unpublished-import': 'off',
      'n/no-extraneous-import': 'off',
      'node/no-extraneous-require': 'error',
      'node/no-unpublished-require': 'error',
      'node/no-unsupported-features/es-syntax': 'off',
      'node/no-missing-require': 'off',
      'node/shebang': 'off',
      'prettier/prettier': 'error',
      'block-scoped-var': 'error',
      eqeqeq: 'error',
      'no-var': 'error',
      'prefer-const': 'error',
      'prefer-arrow-callback': 'error',
      'no-trailing-spaces': 'error',
      quotes: ['warn', 'single', {avoidEscape: true}],
      'no-restricted-properties': [
        'error',
        {object: 'describe', property: 'only'},
        {object: 'it', property: 'only'},
      ],
      'no-dupe-class-members': 'off',
      'require-atomic-updates': 'off',
      'no-undef': 'off',
      'no-inner-declarations': 'off',
      'no-redeclare': 'off',
      'eol-last': 'error',
      'no-console': 'off',
      'no-prototype-builtins': 'off',
      'no-extra-boolean-cast': 'error',
      'no-else-return': 'error',
      'prefer-rest-params': 'error',
      'rxjs-x/ban-operators': ['error', {first: 'use take(1) instead'}],
      'rxjs-x/no-floating-observables': 'error',
      'rxjs-x/no-unbound-methods': 'error',
      'rxjs-x/no-unsafe-takeuntil': 'error',
      'rxjs-x/no-nested-subscribe': 'error',
    },
  },
];
