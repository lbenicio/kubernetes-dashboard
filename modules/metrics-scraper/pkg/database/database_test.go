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

package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	_ "modernc.org/sqlite"

	"k8s.io/dashboard/metrics-scraper/pkg/database"
)

func TestMetricsUtil(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Sidecar Database Test")
}

func TestDatabaseOperationsReturnTransactionAndTableErrors(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := database.CreateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE nodes"); err != nil {
		t.Fatal(err)
	}
	nodes := nodeMetrics()
	if err := database.UpdateDatabase(db, &nodes, &v1beta1.PodMetricsList{}); err == nil {
		t.Fatal("UpdateDatabase() succeeded without the nodes table")
	}
	_ = db.Close()

	cullDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cullDB.SetMaxOpenConns(1)
	if err := database.CreateDatabase(cullDB); err != nil {
		t.Fatal(err)
	}
	if _, err := cullDB.Exec("DROP TABLE pods"); err != nil {
		t.Fatal(err)
	}
	if err := database.CullDatabase(cullDB, time.Minute); err == nil {
		t.Fatal("CullDatabase() succeeded without the pods table")
	}
	_ = cullDB.Close()

	closedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateDatabase(closedDB, &v1beta1.NodeMetricsList{}, &v1beta1.PodMetricsList{}); err == nil {
		t.Fatal("UpdateDatabase() succeeded with a closed database")
	}
	if err := database.CreateDatabase(closedDB); err == nil {
		t.Fatal("CreateDatabase() succeeded with a closed database")
	}
	if err := database.CullDatabase(closedDB, time.Minute); err == nil {
		t.Fatal("CullDatabase() succeeded with a closed database")
	}
}

func TestUpdateDatabaseReportsPodPrepareAndInsertErrors(t *testing.T) {
	t.Run("pod table missing", func(t *testing.T) {
		db := newMetricsTestDB(t)
		if _, err := db.Exec("DROP TABLE pods"); err != nil {
			t.Fatal(err)
		}
		nodes := nodeMetrics()
		pods := podMetrics()
		if err := database.UpdateDatabase(db, &nodes, &pods); err == nil {
			t.Fatal("UpdateDatabase() succeeded without the pods table")
		}
		assertMetricsDBReusable(t, db)
	})

	t.Run("node insert rejected", func(t *testing.T) {
		db := newMetricsTestDB(t)
		if _, err := db.Exec(`CREATE TRIGGER reject_node BEFORE INSERT ON nodes BEGIN SELECT RAISE(ABORT, 'rejected'); END;`); err != nil {
			t.Fatal(err)
		}
		nodes := nodeMetrics()
		if err := database.UpdateDatabase(db, &nodes, &v1beta1.PodMetricsList{}); err == nil {
			t.Fatal("UpdateDatabase() succeeded after the node insert was rejected")
		}
		assertMetricsDBReusable(t, db)
	})

	t.Run("pod insert rejected", func(t *testing.T) {
		db := newMetricsTestDB(t)
		if _, err := db.Exec(`CREATE TRIGGER reject_pod BEFORE INSERT ON pods BEGIN SELECT RAISE(ABORT, 'rejected'); END;`); err != nil {
			t.Fatal(err)
		}
		nodes := nodeMetrics()
		pods := podMetrics()
		if err := database.UpdateDatabase(db, &nodes, &pods); err == nil {
			t.Fatal("UpdateDatabase() succeeded after the pod insert was rejected")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var insertedNodes int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM nodes").Scan(&insertedNodes); err != nil {
			t.Fatalf("database remained unavailable after rollback: %v", err)
		}
		assertMetricsDBReusable(t, db)
		if insertedNodes != 0 {
			t.Fatalf("failed pod insertion left %d node rows committed, want rollback", insertedNodes)
		}
	})
}

func TestCullDatabaseReportsDeleteErrors(t *testing.T) {
	for _, table := range []string{"nodes", "pods"} {
		t.Run(table, func(t *testing.T) {
			db := newMetricsTestDB(t)
			insert := fmt.Sprintf("INSERT INTO %s(time) VALUES (datetime('now', '-5 minutes'))", table)
			if _, err := db.Exec(insert); err != nil {
				t.Fatal(err)
			}
			query := fmt.Sprintf("CREATE TRIGGER reject_delete BEFORE DELETE ON %s BEGIN SELECT RAISE(ABORT, 'rejected'); END;", table)
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			if err := database.CullDatabase(db, time.Minute); err == nil {
				t.Fatalf("CullDatabase() succeeded after the %s delete was rejected", table)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var rows int
			if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&rows); err != nil {
				t.Fatalf("database remained unavailable after failed cull: %v", err)
			}
			if rows != 1 {
				t.Fatalf("failed cull left %d %s rows, want original row retained", rows, table)
			}
			assertMetricsDBReusable(t, db)
		})
	}
}

func assertMetricsDBReusable(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var value int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&value); err != nil || value != 1 {
		t.Fatalf("database is not reusable after failed transaction: value=%d error=%v", value, err)
	}
}

func newMetricsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := database.CreateDatabase(db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func nodeMetrics() v1beta1.NodeMetricsList {
	tmp := v1beta1.NodeMetrics{}
	tmp.SetName("testing")
	tmp.Usage = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1"),
		v1.ResourceMemory: resource.MustParse("100"),
	}

	nm := v1beta1.NodeMetricsList{
		Items: []v1beta1.NodeMetrics{
			tmp,
		},
	}

	return nm
}

func podMetrics() v1beta1.PodMetricsList {
	tmp2 := v1beta1.ContainerMetrics{}
	tmp2.Name = "container_test"
	tmp2.Usage = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1"),
		v1.ResourceMemory: resource.MustParse("100"),
	}

	tmp := v1beta1.PodMetrics{}
	tmp.SetName("testing")
	tmp.Containers = []v1beta1.ContainerMetrics{
		tmp2,
	}

	nm := v1beta1.PodMetricsList{
		Items: []v1beta1.PodMetrics{
			tmp,
		},
	}

	return nm
}

var _ = ginkgo.Describe("Database functions", func() {
	ginkgo.Context("With an in-memory database", func() {
		ginkgo.It("should generate 'nodes' table to dump metrics in.", func() {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				panic(err.Error())
			}

			defer db.Close()

			err = database.CreateDatabase(db)
			if err != nil {
				panic(err.Error())
			}

			_, err = db.Query("select * from nodes;")
			if err != nil {
				panic(err.Error())
			}
			gomega.Expect(err).To(gomega.BeNil())
		})

		ginkgo.It("should generate 'pods' table to dump metrics in.", func() {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				panic(err.Error())
			}
			defer db.Close()

			err = database.CreateDatabase(db)
			if err != nil {
				panic(err.Error())
			}

			_, err = db.Query("select * from pods;")
			if err != nil {
				panic(err.Error())
			}
			gomega.Expect(err).To(gomega.BeNil())
		})

		ginkgo.It("should insert metrics into the database.", func() {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				panic(err.Error())
			}
			defer db.Close()

			err = database.CreateDatabase(db)
			if err != nil {
				panic(err.Error())
			}

			nm := nodeMetrics()
			pm := podMetrics()

			err = database.UpdateDatabase(db, &nm, &pm)
			if err != nil {
				panic(err.Error())
			}

			rows, err := db.Query("select name, cpu, memory from nodes")
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var name string
				var cpu int64
				var memory int64
				err = rows.Scan(&name, &cpu, &memory)
				if err != nil {
					log.Fatal(err)
				}
				testCpu := resource.MustParse("1")
				testMemory := resource.MustParse("100")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(name).To(gomega.Equal("testing"))
				gomega.Expect(cpu).To(gomega.Equal(testCpu.MilliValue()))
				gomega.Expect(memory).To(gomega.Equal(testMemory.MilliValue() / 1000))
			}

			rows, err = db.Query("select name, container, cpu, memory from pods")
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var name string
				var container string
				var cpu int64
				var memory int64
				err = rows.Scan(&name, &container, &cpu, &memory)
				if err != nil {
					log.Fatal(err)
				}
				testCpu := resource.MustParse("1")
				testMemory := resource.MustParse("100")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(name).To(gomega.Equal("testing"))
				gomega.Expect(container).To(gomega.Equal("container_test"))
				gomega.Expect(cpu).To(gomega.Equal(testCpu.MilliValue()))
				gomega.Expect(memory).To(gomega.Equal(testMemory.MilliValue() / 1000))
			}
		})
		ginkgo.It("should cull the database based on a window.", func() {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				panic(err.Error())
			}
			defer db.Close()

			err = database.CreateDatabase(db)
			if err != nil {
				panic(err.Error())
			}

			nm := nodeMetrics()
			pm := podMetrics()

			err = database.UpdateDatabase(db, &nm, &pm)
			if err != nil {
				panic(err.Error())
			}

			sqlStmt := "insert into nodes(name,cpu,memory,storage,time) values('lame','1000','100000','0',datetime('now','-20 minutes'));"
			_, err = db.Exec(sqlStmt)
			if err != nil {
				panic(err.Error())
			}

			timeWindow, err := time.ParseDuration("5m")
			if err != nil {
				panic(err.Error())
			}

			err = database.CullDatabase(db, timeWindow)
			if err != nil {
				panic(err.Error())
			}

			rows, err := db.Query("select name, cpu, memory from nodes")
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var name string
				var cpu int64
				var memory int64
				err = rows.Scan(&name, &cpu, &memory)
				if err != nil {
					log.Fatal(err)
				}
				testCpu := resource.MustParse("1")
				testMemory := resource.MustParse("100")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(name).To(gomega.Equal("testing"))
				gomega.Expect(cpu).To(gomega.Equal(testCpu.MilliValue()))
				gomega.Expect(memory).To(gomega.Equal(testMemory.MilliValue() / 1000))
			}

		})
	})
})
