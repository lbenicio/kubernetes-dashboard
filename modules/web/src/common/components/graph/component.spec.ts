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

import {DataPoint, Metric} from '@api/root.api';
import {GraphComponent, GraphType} from './component';

describe('GraphComponent', () => {
  const makePoints = (values: number[]): DataPoint[] => {
    const start = Date.parse('2024-11-06T00:00:00.000Z') / 1000;
    return values.map((y, index) => ({x: start + index * 60, y}) as DataPoint);
  };
  const makeMetric = (values: number[]): Metric => ({dataPoints: makePoints(values)}) as Metric;

  let component: GraphComponent;

  beforeEach(() => {
    component = new GraphComponent();
    component.id = 'usage';
  });

  it('requires a graph type before initialization', () => {
    component.graphType = undefined as never;

    expect(() => component.ngOnInit()).toThrow('Graph type has to be provided.');
  });

  it('builds the CPU graph series, labels, and color', () => {
    component.metric = makeMetric([100, 200, 300, 400, 500, 600, 700]);
    component.graphType = GraphType.CPU;

    component.ngOnInit();

    expect(component.yAxisLabel).toBe('CPU (cores)');
    expect(component.customColors).toEqual([{name: 'usage', value: '#00c752'}]);
    expect(component.shouldShowGraph).toBe(true);
    expect(component.series).toHaveLength(1);
    expect(component.series[0].name).toBe('usage');
    expect(component.series[0].series.length).toBeGreaterThanOrEqual(5);
    expect(component.series[0].series[0].value).toBeCloseTo(0.1);
    expect(component.series[0].series[0].name).toMatch(/^\d{2}:\d{2}$/);
    expect(component.yScaleMax).toBeGreaterThan(0.5);
    expect(component.yAxisTickFormatting(2)).toBe('2 ');
  });

  it('normalizes memory values and selects memory labels and color', () => {
    component.metric = makeMetric([2_000_000, 2_100_000, 2_200_000, 2_300_000, 2_400_000, 2_500_000, 2_600_000]);
    component.graphType = GraphType.Memory;

    component.ngOnInit();

    expect(component.yAxisLabel).toBe('Memory (bytes)');
    expect(component.customColors).toEqual([{name: 'usage', value: '#326de6'}]);
    expect(component.yAxisTickFormatting(3)).toBe('3 Mi');
    expect(component.yScaleMax).toBeGreaterThan(1);
  });

  it('leaves the graph hidden when there are too few points', () => {
    component.metric = makeMetric([100, 200, 300, 400]);

    component.ngOnInit();

    expect(component.shouldShowGraph).toBe(false);
    expect(component.series[0].series.length).toBeLessThan(5);
  });

  it('rebuilds the series on changes only after the graph becomes visible', () => {
    component.metric = makeMetric([100, 200, 300, 400, 500, 600, 700]);
    component.ngOnInit();
    const initialSeries = component.series;
    const internals = component as unknown as {suffixMap_: Map<number, string>};
    internals.suffixMap_.set(1, 'cores');

    component.metric = makeMetric([200, 300, 400, 500, 600, 700, 800]);
    component.ngOnChanges({});
    expect(component.series).toBe(initialSeries);

    component.onChangeState(true);
    component.ngOnChanges({});

    expect(component.series).not.toBe(initialSeries);
    expect(component.getTooltipValue(1)).toBe('1 ');
  });

  it('formats tooltip values with a matching suffix when present', () => {
    const internals = component as unknown as {suffixMap_: Map<number, string>};
    internals.suffixMap_.set(4, 'Mi');

    expect(component.getTooltipValue(4)).toBe('4 Mi');
    expect(component.getTooltipValue(5)).toBe('5 ');
  });

  it('rejects unsupported graph types', () => {
    component.metric = makeMetric([1]);
    component.graphType = 'disk' as GraphType;

    expect(() => component.ngOnInit()).toThrow();
  });

  it('averages points into time buckets and handles same-bucket points', () => {
    const internals = component as unknown as {
      _average: (acc: DataPoint[], point: DataPoint, index: number, points: DataPoint[]) => DataPoint[];
    };
    const first = {x: 60, y: 2} as DataPoint;
    const sameMinute = {x: 61, y: 4} as DataPoint;
    const nextMinute = {x: 120, y: 8} as DataPoint;
    const points = [first, sameMinute, nextMinute];

    expect(internals._average([], sameMinute, 1, points)).toEqual([]);
    expect(internals._average([], nextMinute, 2, points)).toEqual([{x: 61, y: 3}]);
  });

  it('reduces long series and returns short series unchanged', () => {
    const internals = component as unknown as {
      _averageWithReduce: (points: DataPoint[], limit: number) => DataPoint[];
    };
    const short = makePoints([1, 2, 3]);
    const long = makePoints(Array.from({length: 36}, (_, index) => index + 1));

    expect(internals._averageWithReduce(short, 15)).toBe(short);
    const reduced = internals._averageWithReduce(long, 15);
    expect(reduced.length).toBeLessThan(long.length);
    expect(reduced.every(point => Number.isFinite(point.y))).toBe(true);
  });

  it('chooses and adjusts time scales while reading minute, hour, and day values', () => {
    const internals = component as unknown as {
      timeScale_: number;
      _findTimeScale: (points: DataPoint[]) => void;
      _getTimeScale: (date: number) => number;
    };
    const timestamp = Date.parse('2024-11-06T14:25:00.000Z') / 1000;
    const date = new Date(timestamp * 1000);

    internals.timeScale_ = 0;
    expect(internals._getTimeScale(timestamp)).toBe(date.getMinutes());
    internals.timeScale_ = 1;
    expect(internals._getTimeScale(timestamp)).toBe(date.getHours());
    internals.timeScale_ = 2;
    expect(internals._getTimeScale(timestamp)).toBe(date.getDay());

    internals.timeScale_ = 2;
    internals._findTimeScale(makePoints([1, 2, 3]));
    expect(internals.timeScale_).toBe(1);

    internals.timeScale_ = 0;
    internals._findTimeScale(makePoints(Array.from({length: 20}, (_, index) => index)));
    expect(internals.timeScale_).toBe(0);
  });
});
