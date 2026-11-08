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

import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

const rootDirectory = resolve(fileURLToPath(new URL('../..', import.meta.url)));
const coverageDirectory = resolve(rootDirectory, '.tmp');
const minimumCoverage = 0.98;

function readGoCoverage(profileName) {
  const profilePath = resolve(coverageDirectory, profileName);
  const lines = readFileSync(profilePath, 'utf8').trim().split('\n').slice(1);
  const totals = lines.reduce((result, line) => {
    const match = line.match(/^\S+:\d+\.\d+,\d+\.\d+\s+(\d+)\s+(\d+)$/);

    if (!match) {
      throw new Error(`Invalid Go coverage record in ${profileName}: ${line}`);
    }

    const statements = Number(match[1]);
    const executed = Number(match[2]) > 0;

    return {
      total: result.total + statements,
      covered: result.covered + (executed ? statements : 0),
    };
  }, {total: 0, covered: 0});

  if (totals.total === 0) {
    throw new Error(`No Go coverage records found in ${profileName}`);
  }

  return totals;
}

function readFrontendCoverage() {
  const reportPath = resolve(coverageDirectory, 'coverage-final.json');
  const report = JSON.parse(readFileSync(reportPath, 'utf8'));
  const totals = Object.values(report).reduce((result, fileCoverage) => {
    for (const count of Object.values(fileCoverage.s)) {
      result.total += 1;
      result.covered += count > 0 ? 1 : 0;
    }

    return result;
  }, {total: 0, covered: 0});

  if (totals.total === 0) {
    throw new Error('No frontend statement coverage records found');
  }

  return totals;
}

function combine(totals, additionalTotals) {
  return {
    total: totals.total + additionalTotals.total,
    covered: totals.covered + additionalTotals.covered,
  };
}

function reportCoverage(name, totals) {
  const coverage = totals.covered / totals.total;
  const percent = (coverage * 100).toFixed(2);
  const passes = coverage >= minimumCoverage;

  console.log(`${name}: ${percent}% (${totals.covered}/${totals.total} statements)${passes ? '' : ' — below 98%'}`);

  return passes;
}

const goProfiles = [
  ['API', 'dashboard-api.coverage.out'],
  ['Auth', 'dashboard-auth.coverage.out'],
  ['Metrics Scraper', 'dashboard-metrics-scraper.coverage.out'],
  ['Web server', 'dashboard-web.coverage.out'],
];
const commonProfiles = [
  'common.certificates.coverage.out',
  'common.client.coverage.out',
  'common.csrf.coverage.out',
  'common.errors.coverage.out',
  'common.helpers.coverage.out',
  'common.types.coverage.out',
];

let passed = true;
let overallTotals = {total: 0, covered: 0};

for (const [name, profileName] of goProfiles) {
  const totals = readGoCoverage(profileName);
  passed = reportCoverage(name, totals) && passed;
  overallTotals = combine(overallTotals, totals);
}

const commonTotals = commonProfiles.reduce(
  (totals, profileName) => combine(totals, readGoCoverage(profileName)),
  {total: 0, covered: 0},
);
passed = reportCoverage('Common Go libraries', commonTotals) && passed;
overallTotals = combine(overallTotals, commonTotals);

const frontendTotals = readFrontendCoverage();
passed = reportCoverage('Angular frontend', frontendTotals) && passed;
overallTotals = combine(overallTotals, frontendTotals);
passed = reportCoverage('Combined project', overallTotals) && passed;

if (!passed) {
  process.exitCode = 1;
}
