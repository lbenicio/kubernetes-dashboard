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

import {enableProdMode, ErrorHandler, importProvidersFrom} from '@angular/core';

import {environment} from '@environments/environment';

import {GlobalErrorHandler} from './error/handler';
import {provideHttpClient, withXhr, withInterceptorsFromDi} from '@angular/common/http';
import {BrowserModule, bootstrapApplication} from '@angular/platform-browser';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';
import {CoreModule} from './core.module';
import {ChromeModule} from './chrome/module';
import {LoginModule} from './login/module';
import {withHashLocation, withRouterConfig, provideRouter} from '@angular/router';
import {routes} from './index.routing';
import {RootComponent} from './index.component';

if (environment.production) {
  enableProdMode();
}

bootstrapApplication(RootComponent, {
  providers: [
    importProvidersFrom(BrowserModule, BrowserAnimationsModule, CoreModule, ChromeModule, LoginModule),
    {provide: ErrorHandler, useClass: GlobalErrorHandler},
    provideHttpClient(withXhr(), withInterceptorsFromDi()),
    provideRouter(routes, withHashLocation(), withRouterConfig({onSameUrlNavigation: 'reload'})),
  ],
});
