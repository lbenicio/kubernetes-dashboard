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

// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"k8s.io/dashboard/metrics-scraper/pkg/database"
)

func TestDatabaseTransactionsReturnFailuresAtEachStage(t *testing.T) {
	const (
		stageBegin       = "begin"
		stagePrepareNode = "prepare nodes"
		stageExecNode    = "insert nodes"
		stagePreparePod  = "prepare pods"
		stageExecPod     = "insert pods"
		stagePrepareCull = "prepare cull pods"
		stageExecCull    = "delete pods"
		stageCommit      = "commit"
	)

	for _, test := range []struct {
		name      string
		operation func(*sql.DB) error
		stage     string
	}{
		{name: "update begin", operation: func(db *sql.DB) error {
			return database.UpdateDatabase(db, &v1beta1.NodeMetricsList{}, &v1beta1.PodMetricsList{})
		}, stage: stageBegin},
		{name: "update node prepare", operation: func(db *sql.DB) error {
			return database.UpdateDatabase(db, &v1beta1.NodeMetricsList{}, &v1beta1.PodMetricsList{})
		}, stage: stagePrepareNode},
		{name: "update node insert", operation: func(db *sql.DB) error {
			metrics := nodeMetrics()
			return database.UpdateDatabase(db, &metrics, &v1beta1.PodMetricsList{})
		}, stage: stageExecNode},
		{name: "update pod prepare", operation: func(db *sql.DB) error {
			return database.UpdateDatabase(db, &v1beta1.NodeMetricsList{}, &v1beta1.PodMetricsList{})
		}, stage: stagePreparePod},
		{name: "update pod insert", operation: func(db *sql.DB) error {
			metrics := podMetrics()
			return database.UpdateDatabase(db, &v1beta1.NodeMetricsList{}, &metrics)
		}, stage: stageExecPod},
		{name: "update commit", operation: func(db *sql.DB) error {
			return database.UpdateDatabase(db, &v1beta1.NodeMetricsList{}, &v1beta1.PodMetricsList{})
		}, stage: stageCommit},
		{name: "cull begin", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: stageBegin},
		{name: "cull node prepare", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: "prepare cull nodes"},
		{name: "cull node delete", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: "delete nodes"},
		{name: "cull pod prepare", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: stagePrepareCull},
		{name: "cull pod delete", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: stageExecCull},
		{name: "cull commit", operation: func(db *sql.DB) error { return database.CullDatabase(db, time.Minute) }, stage: stageCommit},
	} {
		t.Run(test.name, func(t *testing.T) {
			behavior := &databaseFailureBehavior{stage: test.stage}
			db := sql.OpenDB(databaseFailureConnector{behavior: behavior})
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = db.Close() })
			if err := test.operation(db); err == nil {
				t.Fatalf("operation unexpectedly succeeded at %s", test.stage)
			}
		})
	}
}

type databaseFailureBehavior struct {
	stage string
}

type databaseFailureConnector struct {
	behavior *databaseFailureBehavior
}

func (in databaseFailureConnector) Connect(context.Context) (driver.Conn, error) {
	return databaseFailureConn(in), nil
}

func (databaseFailureConnector) Driver() driver.Driver { return databaseFailureDriver{} }

type databaseFailureDriver struct{}

func (databaseFailureDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("database failure driver requires OpenDB")
}

type databaseFailureConn struct {
	behavior *databaseFailureBehavior
}

func (in databaseFailureConn) Prepare(query string) (driver.Stmt, error) {
	if in.behavior.stage == "prepare nodes" && strings.Contains(query, "insert into nodes") ||
		in.behavior.stage == "prepare pods" && strings.Contains(query, "insert into pods") ||
		in.behavior.stage == "prepare cull nodes" && strings.Contains(query, "delete from nodes") ||
		in.behavior.stage == "prepare cull pods" && strings.Contains(query, "delete from pods") {
		return nil, errors.New("statement preparation rejected")
	}
	return databaseFailureStmt{behavior: in.behavior, query: query}, nil
}

func (in databaseFailureConn) Close() error { return nil }

func (in databaseFailureConn) Begin() (driver.Tx, error) {
	if in.behavior.stage == "begin" {
		return nil, errors.New("transaction begin rejected")
	}
	return databaseFailureTx(in), nil
}

type databaseFailureTx struct {
	behavior *databaseFailureBehavior
}

func (in databaseFailureTx) Commit() error {
	if in.behavior.stage == "commit" {
		return errors.New("transaction commit rejected")
	}
	return nil
}

func (databaseFailureTx) Rollback() error { return nil }

type databaseFailureStmt struct {
	behavior *databaseFailureBehavior
	query    string
}

func (databaseFailureStmt) Close() error  { return nil }
func (databaseFailureStmt) NumInput() int { return -1 }

func (in databaseFailureStmt) Exec([]driver.Value) (driver.Result, error) {
	if in.behavior.stage == "insert nodes" && strings.Contains(in.query, "insert into nodes") ||
		in.behavior.stage == "insert pods" && strings.Contains(in.query, "insert into pods") ||
		in.behavior.stage == "delete nodes" && strings.Contains(in.query, "delete from nodes") ||
		in.behavior.stage == "delete pods" && strings.Contains(in.query, "delete from pods") {
		return nil, errors.New("statement execution rejected")
	}
	return databaseFailureResult{}, nil
}

func (databaseFailureStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("queries are not supported by the failure driver")
}

type databaseFailureResult struct{}

func (databaseFailureResult) LastInsertId() (int64, error) { return 0, nil }
func (databaseFailureResult) RowsAffected() (int64, error) { return 0, nil }
