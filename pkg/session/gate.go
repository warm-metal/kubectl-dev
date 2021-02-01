package session

import (
	"context"
	"fmt"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	"strings"
	"time"
)

//go:generate protoc rpc.proto --go_out=plugins=grpc:.

func PrepareGate(s *grpc.Server, opts *GateOptions) {
	gate := terminalGate{opts: opts}
	gate.prepareNamespaces()

	RegisterAppGateServer(s, &gate)
}

type GateOptions struct {
	Namespace     string
	coreNamespace string
}

type terminalGate struct {
	opts      *GateOptions
	config    *rest.Config
	clientset *kubernetes.Clientset
}

func timeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), 5*time.Second)
}

func (t *terminalGate) prepareNamespaces() {
	t.opts.coreNamespace = getCurrentNamespace()

	if t.opts.Namespace == "" {
		panic("namespace is required.")
	}

	var err error
	t.config, err = rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	t.clientset, err = kubernetes.NewForConfig(t.config)
	if err != nil {
		panic(err.Error())
	}

	client := t.clientset.CoreV1().Namespaces()
	ctx, cancel := timeoutContext()
	defer cancel()
	_, err = client.Get(ctx, t.opts.Namespace, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !errors.IsNotFound(err) {
		panic(err.Error())
	}

	ctx2, cancel2 := timeoutContext()
	defer cancel2()
	_, err = client.Create(ctx2, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.opts.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func getCurrentNamespace() string {
	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	panic("can't fetch the current namespace")
}

func (t *terminalGate) openSession(pod string, cmd []string, s AppGate_OpenAppServer) (err error) {
	// FIXME count sessions to the same app

	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(t.opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: appContainer,
			Command:   append([]string{"chroot", "/app-root"}, cmd...),
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		klog.Errorf("can't create executor: %s", err)
		return
	}

	klog.Infof("open session to Pod %s", pod)
	stdin, stdout, stderr := genIOStreams(s)
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    true,
	})

	return
}

type reader struct {
	s AppGate_OpenAppServer
}

func (r reader) Read(p []byte) (n int, err error) {
	req, err := r.s.Recv()
	if err != nil {
		klog.Errorf("can't read stdin: %s", err)
		return
	}

	if len(p) < len(req.Stdin[0]) {
		err = io.ErrShortBuffer
		klog.Errorf("buffer too small %d, %d", len(p), len(req.Stdin[0]))
		return
	}

	n = copy(p, req.Stdin[0])
	return
}

type stdoutWriter struct {
	s AppGate_OpenAppServer
}

func (w stdoutWriter) Write(p []byte) (n int, err error) {
	err = w.s.Send(&AppResponse{
		Stdout: string(p),
	})

	if err != nil {
		klog.Errorf("can't write stdout: %s", err)
		return
	}

	n = len(p)
	return
}

type stderrWriter struct {
	s AppGate_OpenAppServer
}

func (w stderrWriter) Write(p []byte) (n int, err error) {
	err = w.s.Send(&AppResponse{
		Stderr: string(p),
	})

	if err != nil {
		klog.Errorf("can't write stderr: %s", err)
		return
	}

	n = len(p)
	return
}

func genIOStreams(s AppGate_OpenAppServer) (stdin io.Reader, stdout, stderr io.Writer) {
	return &reader{s}, &stdoutWriter{s}, &stderrWriter{s}
}

func (t *terminalGate) OpenApp(s AppGate_OpenAppServer) error {
	req, err := s.Recv()
	if err != nil {
		klog.Errorf("can't receive date from client: %s", err)
		return status.Error(codes.Unavailable, err.Error())
	}

	// FIXME app name is allowed to be empty.
	if req.App.Name == "" {
		return status.Error(codes.InvalidArgument, "App.Name is required in the first request.")
	}
	if req.App.Image == "" {
		return status.Error(codes.InvalidArgument, "App.Image is required in the first request.")
	}
	if len(req.Stdin) == 0 {
		return status.Error(codes.InvalidArgument, "App.Stdin is required.")
	}

	ctx, cancel := timeoutContext()
	defer cancel()
	client := t.clientset.CoreV1().Pods(t.opts.Namespace)

	klog.Infof("fetch app %s", req.App.Name)
	pod, err := client.Get(ctx, req.App.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("can't fetch Pod of app %s: %s", req.App.Name, err)
		return status.Error(codes.Unavailable, err.Error())
	}

	if err != nil {
		klog.Infof("install app %s", req.App.Image)
		ctx, cancel := timeoutContext()
		defer cancel()
		pod, err = client.Create(
			ctx, genAppManifest(req.App.Name, t.opts.Namespace, req.App.Image, t.opts.coreNamespace, req.App.Hostpath),
			metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("can't create Pod of app %s: %s", req.App.Name, err)
			return status.Error(codes.Unavailable, err.Error())
		}
	}

	klog.Infof("watch app %s until it is ready", req.App.Name)
	watcher, err := client.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": req.App.Name}.AsSelector().String(),
		Watch:         true,
	})

	if err != nil {
		klog.Errorf("can't watch Pod of app %s: %s", req.App.Name, err)
		return status.Error(codes.Unavailable, err.Error())
	}

	err = utils.WaitUntilErrorOr(watcher, func(object runtime.Object) (b bool, err error) {
		return utils.IsPodReady(object.(*corev1.Pod)), nil
	})

	if err != nil {
		klog.Errorf("can't found a ready Pod of app %s: %s", req.App.Name, err)
		return status.Error(codes.Unavailable, fmt.Sprintf("can't start Pod %s: %s", req.App.Name, err))
	}

	klog.Infof("create session to app %s", req.App.Name)
	err = t.openSession(pod.Name, req.Stdin, s)
	if err != nil {
		klog.Errorf("can't open stream of app %s: %#v", req.App.Name, err)
		return status.Error(codes.Unavailable, err.Error())
	}

	return nil
}
