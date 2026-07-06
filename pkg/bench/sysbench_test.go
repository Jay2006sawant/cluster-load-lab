package bench

import (
	"strings"
	"testing"
)

func TestBuildSysbenchJobPostgres(t *testing.T) {
	job := BuildSysbenchJob(Config{
		Host: "pg.default.svc", User: "postgres", Password: "secret",
		Driver: "pgsql",
	})
	if job.Name != "cluster-load-lab-sysbench" {
		t.Fatalf("name: %s", job.Name)
	}
	script := job.Spec.Template.Spec.Containers[0].Command[2]
	if !strings.Contains(script, "--pgsql-password=secret") {
		t.Fatalf("expected password in script: %s", script)
	}
}

func TestBuildSysbenchJobMySQL(t *testing.T) {
	job := BuildSysbenchJob(Config{
		Host: "mysql.default.svc", User: "root", Password: "pw", Driver: "mysql",
	})
	script := job.Spec.Template.Spec.Containers[0].Command[2]
	if !strings.Contains(script, "--mysql-password=pw") {
		t.Fatalf("expected mysql password in script: %s", script)
	}
}
