package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Jay2006sawant/cluster-load-lab/pkg/bench"
	"github.com/Jay2006sawant/cluster-load-lab/pkg/runner"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "manifest":
		cmdManifest(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `cluster-load-lab — run sysbench inside Kubernetes Jobs

Usage:
  cluster-load-lab manifest [flags]   Print a Job manifest to stdout
  cluster-load-lab run [flags]        Create Job, wait, print logs + TPS

Flags (manifest and run):
  --namespace string    Target namespace (default "default")
  --name string         Job name (default "cluster-load-lab-sysbench")
  --host string         Database host (required)
  --port int            Database port (default 5432 for pgsql)
  --user string         Database user (required)
  --password string     Database password (required for run; optional for manifest)
  --database string     Database name (default postgres)
  --driver string       sysbench driver: pgsql or mysql (default pgsql)
  --threads int         Worker threads (default 8)
  --duration int        Run duration in seconds (default 30)
  --kubeconfig string   Path to kubeconfig (default in-cluster or ~/.kube/config)

`)
}

type flags struct {
	namespace  string
	name       string
	host       string
	port       int
	user       string
	password   string
	database   string
	driver     string
	threads    int
	duration   int
	kubeconfig string
}

func parseFlags(args []string) flags {
	fs := flag.NewFlagSet("cluster-load-lab", flag.ExitOnError)
	var f flags
	fs.StringVar(&f.namespace, "namespace", "default", "target namespace")
	fs.StringVar(&f.name, "name", "cluster-load-lab-sysbench", "job name")
	fs.StringVar(&f.host, "host", "", "database host")
	fs.IntVar(&f.port, "port", 0, "database port")
	fs.StringVar(&f.user, "user", "", "database user")
	fs.StringVar(&f.password, "password", "", "database password")
	fs.StringVar(&f.database, "database", "", "database name")
	fs.StringVar(&f.driver, "driver", "pgsql", "sysbench driver")
	fs.IntVar(&f.threads, "threads", 8, "worker threads")
	fs.IntVar(&f.duration, "duration", 30, "run duration seconds")
	fs.StringVar(&f.kubeconfig, "kubeconfig", "", "kubeconfig path")
	_ = fs.Parse(args)
	return f
}

func (f flags) benchConfig() bench.Config {
	return bench.Config{
		Name:      f.name,
		Namespace: f.namespace,
		Host:      f.host,
		Port:      f.port,
		User:      f.user,
		Password:  f.password,
		Database:  f.database,
		Driver:    f.driver,
		Threads:   f.threads,
		Duration:  f.duration,
	}
}

func cmdManifest(args []string) {
	f := parseFlags(args)
	if f.host == "" || f.user == "" {
		fmt.Fprintln(os.Stderr, "manifest requires --host and --user")
		os.Exit(1)
	}
	job := bench.BuildSysbenchJob(f.benchConfig())
	out, err := encodeJobYAML(job)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(string(out))
}

func cmdRun(args []string) {
	f := parseFlags(args)
	if f.host == "" || f.user == "" || f.password == "" {
		fmt.Fprintln(os.Stderr, "run requires --host, --user, and --password")
		os.Exit(1)
	}

	client, err := runner.NewFromKubeconfig(f.kubeconfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	job := bench.BuildSysbenchJob(f.benchConfig())
	ctx := context.Background()

	created, err := client.CreateJob(ctx, job)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create job:", err)
		os.Exit(1)
	}
	fmt.Printf("created job %s/%s\n", created.Namespace, created.Name)

	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(f.duration+300)*time.Second)
	defer cancel()

	_, err = client.WaitForJob(waitCtx, created.Namespace, created.Name, 3*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wait:", err)
	}

	logs, err := client.JobLogs(ctx, created.Namespace, created.Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logs:", err)
		os.Exit(1)
	}
	fmt.Println("--- sysbench logs ---")
	fmt.Println(logs)

	if tps, ok := runner.ParseTPS(logs); ok {
		summary := map[string]any{
			"job":      created.Name,
			"namespace": created.Namespace,
			"tps":      tps,
			"threads":  f.threads,
			"duration": f.duration,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		fmt.Println("--- summary ---")
		_ = enc.Encode(summary)
	}
}

func encodeJobYAML(job *batchv1.Job) ([]byte, error) {
	job = job.DeepCopy()
	job.TypeMeta = metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"}
	return yaml.Marshal(job)
}
