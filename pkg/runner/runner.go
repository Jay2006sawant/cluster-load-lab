package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes operations for benchmark Jobs.
type Client struct {
	k8s kubernetes.Interface
}

func NewFromKubeconfig(kubeconfig string) (*Client, error) {
	cfg, err := loadConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	return &Client{k8s: cs}, nil
}

func loadConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		cfg, err := rest.InClusterConfig()
		if err == nil {
			return cfg, nil
		}
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func (c *Client) CreateJob(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	return c.k8s.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
}

func (c *Client) WaitForJob(ctx context.Context, namespace, name string, poll time.Duration) (*batchv1.Job, error) {
	for {
		job, err := c.k8s.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				return job, nil
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				return job, fmt.Errorf("job failed: %s", cond.Message)
			}
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (c *Client) JobLogs(ctx context.Context, namespace, jobName string) (string, error) {
	pods, err := c.k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobName)
	}

	var b strings.Builder
	for _, pod := range pods.Items {
		req := c.k8s.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		stream, err := req.Stream(ctx)
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			return "", err
		}
		b.Write(data)
	}
	return b.String(), nil
}

// ParseTPS extracts transactions per second from sysbench stdout.
func ParseTPS(logs string) (float64, bool) {
	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "transactions:") {
			continue
		}
		// Format: transactions: 12345 (1234.56 per sec.)
		start := strings.Index(line, "(")
		end := strings.Index(line, " per sec.)")
		if start >= 0 && end > start {
			var tps float64
			if _, err := fmt.Sscanf(line[start+1:end], "%f", &tps); err == nil {
				return tps, true
			}
		}
	}
	return 0, false
}
