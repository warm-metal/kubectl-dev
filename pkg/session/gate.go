package session

import (
	"context"
	"fmt"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

//go:generate protoc rpc.proto --go_out=plugins=grpc:.

func PrepareGate(s *grpc.Server) {
	gate := terminalGate{
		sessionMap: make(map[types.NamespacedName]*appSession),
	}
	gate.init()
	RegisterAppGateServer(s, &gate)
}

type terminalGate struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
	appClient *appv1.Clientset

	sessionMap   map[types.NamespacedName]*appSession
	sessionGuard sync.Mutex
}

func timeoutContext(parent ...context.Context) (context.Context, context.CancelFunc) {
	if len(parent) > 1 {
		panic(len(parent))
	}

	if len(parent) > 0 {
		return context.WithTimeout(parent[0], 5*time.Second)
	} else {
		return context.WithTimeout(context.TODO(), 5*time.Second)
	}
}

func (t *terminalGate) init() {
	if err := appcorev1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	fmt.Printf("All types:%#v\n", scheme.Scheme.AllKnownTypes())

	var err error
	t.config, err = rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	t.clientset, err = kubernetes.NewForConfig(t.config)
	if err != nil {
		panic(err.Error())
	}

	t.appClient, err = appv1.NewForConfig(t.config)
	if err != nil {
		return
	}

	_, err = t.appClient.CliappV1().CliApps(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
}

func (t *terminalGate) attach(app *appcorev1.CliApp, cmd []string, in *clientReader, stdout io.Writer) (err error) {
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(app.Status.PodName).Namespace(app.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: "workspace",
		Command:   append([]string{"chroot", "/app-root"}, append(app.Spec.Command, cmd...)...),
		Stdin:     true,
		Stdout:    true,
		Stderr:    false,
		TTY:       true,
	}, scheme.ParameterCodec)

	remoteExec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		klog.Errorf("can't create executor: %s", err)
		return
	}

	klog.Infof("open session to Pod %s/%s", app.Namespace, app.Status.PodName)

	err = remoteExec.Stream(remotecommand.StreamOptions{
		Stdin:             in,
		Stdout:            stdout,
		Tty:               true,
		TerminalSizeQueue: in,
	})

	return
}

func (t *terminalGate) OpenApp(s AppGate_OpenAppServer) error {
	req, err := s.Recv()
	if err != nil {
		klog.Errorf("can't receive date from client: %s", err)
		return status.Error(codes.Unavailable, err.Error())
	}

	if req.App.Name == "" {
		return status.Error(codes.InvalidArgument, "App.Name is required in the first request.")
	}
	if req.App.Namespace == "" {
		return status.Error(codes.InvalidArgument, "App.Namespace is required in the first request.")
	}
	if len(req.Stdin) == 0 {
		return status.Error(codes.InvalidArgument, "Stdin is required.")
	}
	if req.TerminalSize == nil {
		return status.Error(codes.InvalidArgument, "TerminalSize is required.")
	}

	sessionKey := types.NamespacedName{
		Namespace: req.App.Namespace,
		Name:      req.App.Name,
	}

	t.sessionGuard.Lock()

	session := t.sessionMap[sessionKey]
	if session == nil {
		session = &appSession{appClient: t.appClient}
		t.sessionMap[sessionKey] = session
	}

	t.sessionGuard.Unlock()

	klog.Infof("fetch app %s", &sessionKey)
	app, err := session.open(s.Context(), &sessionKey)
	if err != nil {
		klog.Errorf("unable to open app %s: %s", &sessionKey, err)
		return status.Error(codes.Unavailable, err.Error())
	}

	defer func() {
		if err := session.close(s.Context(), &sessionKey); err != nil {
			klog.Errorf("unable to close session of app %s", &sessionKey)
		}
	}()

	klog.Infof("open session to app %s", &sessionKey)
	stdin, stdout := genIOStreams(s, req.TerminalSize)
	defer stdin.Close()

	if err = t.attach(app, req.Stdin, stdin, stdout); err != nil {
		if details, ok := err.(exec.CodeExitError); ok {
			klog.Errorf("can't open stream of app %s: %s", &sessionKey, details.Err.Error())
			return status.Errorf(codes.Aborted, "%d", details.Code)
		} else {
			klog.Errorf("can't open stream of app %s: %#v", &sessionKey, err)
			return status.Error(codes.Unavailable, err.Error())
		}
	}

	return nil
}
