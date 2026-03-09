package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/remotecommand"
)

func TestKubePodExecRunnerExecuteSuccess(t *testing.T) {
	var (
		capturedMethod string
		capturedURL    *url.URL
	)

	runner := kubePodExecRunner{
		kube: fakeKubeClient{
			core: fakeCoreV1Client{
				restClient: &restfake.RESTClient{
					GroupVersion:         schema.GroupVersion{Version: "v1"},
					VersionedAPIPath:     "/api",
					NegotiatedSerializer: kubescheme.Codecs.WithoutConversion(),
				},
			},
		},
		restConfig: &rest.Config{Host: "https://cluster.example"},
		newExecutor: func(_ *rest.Config, method string, targetURL *url.URL) (remotecommand.Executor, error) {
			capturedMethod = method
			cloned := *targetURL
			capturedURL = &cloned
			return &fakeRemoteExecutor{
				streamWithContextFn: func(_ context.Context, opts remotecommand.StreamOptions) error {
					_, _ = opts.Stdout.Write([]byte("ok\n"))
					_, _ = opts.Stderr.Write([]byte("warn\n"))
					return nil
				},
			}, nil
		},
	}

	result, err := runner.Execute(context.Background(), podExecRequest{
		Namespace:     "smith-system",
		PodName:       "loop-a-pod",
		ContainerName: "replica",
		Command:       "echo ok",
	})
	if err != nil {
		t.Fatalf("expected no execution error, got %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit_code=0, got %d", result.ExitCode)
	}
	if result.Stdout != "ok\n" || result.Stderr != "warn\n" {
		t.Fatalf("unexpected output capture: %+v", result)
	}
	if capturedMethod != http.MethodPost {
		t.Fatalf("expected new executor method POST, got %q", capturedMethod)
	}
	if capturedURL == nil {
		t.Fatal("expected command URL to be captured")
	}
	if got := capturedURL.Path; got != "/api/namespaces/smith-system/pods/loop-a-pod/exec" {
		t.Fatalf("unexpected pod exec path: %q", got)
	}
	query := capturedURL.Query()
	if query.Get("container") != "replica" {
		t.Fatalf("expected container query param replica, got %q", query.Get("container"))
	}
	if query.Get("stdout") != "true" || query.Get("stderr") != "true" {
		t.Fatalf("unexpected stream query params: %s", capturedURL.RawQuery)
	}
	commands := query["command"]
	if len(commands) != 3 || commands[0] != "/bin/sh" || commands[1] != "-lc" || commands[2] != "echo ok" {
		t.Fatalf("unexpected command query values: %#v", commands)
	}
}

func TestKubePodExecRunnerExecuteHandlesExitError(t *testing.T) {
	runner := kubePodExecRunner{
		kube: fakeKubeClient{
			core: fakeCoreV1Client{
				restClient: &restfake.RESTClient{
					GroupVersion:         schema.GroupVersion{Version: "v1"},
					VersionedAPIPath:     "/api",
					NegotiatedSerializer: kubescheme.Codecs.WithoutConversion(),
				},
			},
		},
		restConfig: &rest.Config{Host: "https://cluster.example"},
		newExecutor: func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
			return &fakeRemoteExecutor{
				streamWithContextFn: func(_ context.Context, opts remotecommand.StreamOptions) error {
					_, _ = opts.Stdout.Write([]byte("partial-out\n"))
					_, _ = opts.Stderr.Write([]byte("partial-err\n"))
					return fakePodExitError{status: 23}
				},
			}, nil
		},
	}

	result, err := runner.Execute(context.Background(), podExecRequest{
		Namespace:     "smith-system",
		PodName:       "loop-b-pod",
		ContainerName: "replica",
		Command:       "false",
	})
	if err != nil {
		t.Fatalf("expected no terminal runner error for exit-status failure, got %v", err)
	}
	if result.ExitCode != 23 {
		t.Fatalf("expected mapped exit code 23, got %d", result.ExitCode)
	}
	if result.Stdout != "partial-out\n" || result.Stderr != "partial-err\n" {
		t.Fatalf("expected stdout/stderr capture with exit error, got %+v", result)
	}
}

func TestKubePodExecRunnerExecuteReturnsStreamFailure(t *testing.T) {
	runner := kubePodExecRunner{
		kube: fakeKubeClient{
			core: fakeCoreV1Client{
				restClient: &restfake.RESTClient{
					GroupVersion:         schema.GroupVersion{Version: "v1"},
					VersionedAPIPath:     "/api",
					NegotiatedSerializer: kubescheme.Codecs.WithoutConversion(),
				},
			},
		},
		restConfig: &rest.Config{Host: "https://cluster.example"},
		newExecutor: func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
			return &fakeRemoteExecutor{
				streamWithContextFn: func(_ context.Context, opts remotecommand.StreamOptions) error {
					_, _ = opts.Stdout.Write([]byte("partial-out\n"))
					_, _ = opts.Stderr.Write([]byte("partial-err\n"))
					return errors.New("spdy upgrade failed")
				},
			}, nil
		},
	}

	result, err := runner.Execute(context.Background(), podExecRequest{
		Namespace:     "smith-system",
		PodName:       "loop-c-pod",
		ContainerName: "replica",
		Command:       "echo ok",
	})
	if err == nil {
		t.Fatal("expected stream failure to return error")
	}
	if err.Error() != "spdy upgrade failed" {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if result.Stdout != "partial-out\n" || result.Stderr != "partial-err\n" {
		t.Fatalf("expected partial output capture on stream failure, got %+v", result)
	}
}

type fakeKubeClient struct {
	kubernetes.Interface
	core corev1client.CoreV1Interface
}

func (f fakeKubeClient) CoreV1() corev1client.CoreV1Interface {
	return f.core
}

type fakeCoreV1Client struct {
	corev1client.CoreV1Interface
	restClient rest.Interface
}

func (f fakeCoreV1Client) RESTClient() rest.Interface {
	return f.restClient
}

type fakeRemoteExecutor struct {
	streamWithContextFn func(context.Context, remotecommand.StreamOptions) error
}

func (f *fakeRemoteExecutor) Stream(options remotecommand.StreamOptions) error {
	return f.StreamWithContext(context.Background(), options)
}

func (f *fakeRemoteExecutor) StreamWithContext(ctx context.Context, options remotecommand.StreamOptions) error {
	if f.streamWithContextFn == nil {
		return nil
	}
	return f.streamWithContextFn(ctx, options)
}

type fakePodExitError struct {
	status int
}

func (f fakePodExitError) Error() string {
	return "command exited with non-zero status"
}

func (f fakePodExitError) String() string {
	return f.Error()
}

func (f fakePodExitError) Exited() bool {
	return true
}

func (f fakePodExitError) ExitStatus() int {
	return f.status
}
