package bench

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultImage     = "ubuntu:24.04"
	DefaultDriver    = "pgsql"
	DefaultThreads   = 8
	DefaultDuration  = 30
	DefaultTables    = 4
	DefaultTableSize = 10000
)

// Config describes a sysbench benchmark Job.
type Config struct {
	Name      string
	Namespace string
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	Driver    string // pgsql or mysql
	Threads   int
	Duration  int // seconds
	Tables    int
	TableSize int
	Image     string
}

func (c Config) withDefaults() Config {
	if c.Name == "" {
		c.Name = "cluster-load-lab-sysbench"
	}
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Driver == "" {
		c.Driver = DefaultDriver
	}
	if c.Port == 0 {
		if c.Driver == "mysql" {
			c.Port = 3306
		} else {
			c.Port = 5432
		}
	}
	if c.Database == "" {
		if c.Driver == "mysql" {
			c.Database = "sbtest"
		} else {
			c.Database = "postgres"
		}
	}
	if c.Threads == 0 {
		c.Threads = DefaultThreads
	}
	if c.Duration == 0 {
		c.Duration = DefaultDuration
	}
	if c.Tables == 0 {
		c.Tables = DefaultTables
	}
	if c.TableSize == 0 {
		c.TableSize = DefaultTableSize
	}
	if c.Image == "" {
		c.Image = DefaultImage
	}
	return c
}

func (c Config) connFlags() string {
	cfg := c.withDefaults()
	pass := cfg.Password
	if pass == "" {
		pass = "REPLACE_DB_PASSWORD"
	}
	if cfg.Driver == "mysql" {
		return fmt.Sprintf("--mysql-host=%s --mysql-port=%d --mysql-user=%s --mysql-password=%s --mysql-db=%s",
			cfg.Host, cfg.Port, cfg.User, pass, cfg.Database)
	}
	return fmt.Sprintf("--pgsql-host=%s --pgsql-port=%d --pgsql-user=%s --pgsql-password=%s --pgsql-db=%s",
		cfg.Host, cfg.Port, cfg.User, pass, cfg.Database)
}

// BuildSysbenchJob returns a Kubernetes Job that prepares schema and runs OLTP read/write.
func BuildSysbenchJob(cfg Config) *batchv1.Job {
	cfg = cfg.withDefaults()
	conn := cfg.connFlags()

	script := strings.Join([]string{
		"set -eu",
		"export DEBIAN_FRONTEND=noninteractive",
		"apt-get update -qq",
		"apt-get install -y -qq sysbench > /dev/null",
		fmt.Sprintf("sysbench --db-driver=%s %s oltp_read_write prepare --tables=%d --table-size=%d",
			cfg.Driver, conn, cfg.Tables, cfg.TableSize),
		fmt.Sprintf("sysbench --db-driver=%s %s oltp_read_write run --threads=%d --time=%d --report-interval=5",
			cfg.Driver, conn, cfg.Threads, cfg.Duration),
		fmt.Sprintf("sysbench --db-driver=%s %s oltp_read_write cleanup --tables=%d",
			cfg.Driver, conn, cfg.Tables),
	}, "\n")

	ttl := int32(600)
	backoff := int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "cluster-load-lab",
				"app.kubernetes.io/component": "sysbench",
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "cluster-load-lab",
						"app.kubernetes.io/component": "sysbench",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "sysbench",
							Image:   cfg.Image,
							Command: []string{"/bin/bash", "-ec", script},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}
