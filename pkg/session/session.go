package session

import (
	"context"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"sync"
)

type appSession struct {
	appClient *appv1.Clientset

	activeCount atomic.Int32
	app         *appcorev1.CliApp

	outgoingOpenSession  *RemoteChanges
	outgoingCloseSession *RemoteChanges
	ctxCancel            context.CancelFunc
	guard                sync.Mutex
}

func (t *appSession) open(ctx context.Context, name *types.NamespacedName) (app *appcorev1.CliApp, err error) {
	active := t.activeCount.Add(1)
	if active < 1 {
		panic(name.String())
	}

	// The t.app could be nil if the remote spec changes are committing.
	if active > 1 && t.app != nil {
		return t.app, nil
	}

	err = <-t.remoteOpen(ctx, name)
	app = t.app
	return
}

func (t *appSession) close(ctx context.Context, name *types.NamespacedName) (err error) {
	active := t.activeCount.Sub(1)
	if active < 0 {
		panic(t.app.Name)
	}

	if active > 0 {
		return
	}

	err = <-t.remoteClose(ctx, name)
	return
}

type RemoteChangesApplier func(context.Context) error

type RemoteChanges struct {
	Context      context.Context
	Cancel       context.CancelFunc
	ApplyChanges RemoteChangesApplier
	Error        chan error
}

func (t *appSession) commitChanges(rc *RemoteChanges) {
	defer close(rc.Error)
	defer func() {
		rc.Cancel()
		t.guard.Lock()
		defer t.guard.Unlock()
		if t.outgoingOpenSession == rc || t.outgoingCloseSession == rc {
			t.outgoingOpenSession = nil
			t.outgoingCloseSession = nil
			t.ctxCancel = nil
		}
	}()

	if err := rc.ApplyChanges(rc.Context); err != nil {
		rc.Error <- err
	}
}

func (t *appSession) remoteClose(ctx context.Context, name *types.NamespacedName) <-chan error {
	t.guard.Lock()
	defer t.guard.Unlock()

	if t.outgoingCloseSession != nil {
		panic("try to close an app more than once")
	}

	if t.outgoingOpenSession != nil {
		// Cancel all outgoing session open
		t.ctxCancel()
		t.outgoingOpenSession = nil
		t.ctxCancel = nil
	}

	t.app = nil
	done := make(chan error)
	ctx, t.ctxCancel = context.WithCancel(ctx)
	t.outgoingCloseSession = &RemoteChanges{
		Context: ctx,
		Cancel:  t.ctxCancel,
		ApplyChanges: func(ctx context.Context) error {
			return t.closeApp(ctx, name)
		},
		Error: done,
	}

	go t.commitChanges(t.outgoingCloseSession)
	return done
}

func (t *appSession) closeApp(parent context.Context, name *types.NamespacedName) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ctx, cancel := timeoutContext(parent)
		defer cancel()
		app, err := t.appClient.CliappV1().CliApps(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if app.Spec.TargetPhase == appcorev1.CliAppPhaseRest {
			return nil
		}

		app.Spec.TargetPhase = appcorev1.CliAppPhaseRest
		ctx2, cancel2 := timeoutContext(ctx)
		defer cancel2()
		app, err = t.appClient.CliappV1().CliApps(app.Namespace).Update(ctx2, app, metav1.UpdateOptions{})
		return err
	})
}

func (t *appSession) remoteOpen(ctx context.Context, name *types.NamespacedName) <-chan error {
	t.guard.Lock()
	defer t.guard.Unlock()

	if t.outgoingOpenSession != nil {
		if t.outgoingCloseSession != nil {
			panic("t.outgoingCloseSession != nil")
		}

		return t.outgoingOpenSession.Error
	}

	if t.outgoingCloseSession != nil {
		// Cancel all outgoing session close
		t.ctxCancel()
		t.outgoingCloseSession = nil
		t.ctxCancel = nil
	}

	done := make(chan error)
	ctx, t.ctxCancel = context.WithCancel(ctx)

	t.outgoingOpenSession = &RemoteChanges{
		Context: ctx,
		Cancel:  t.ctxCancel,
		ApplyChanges: func(ctx context.Context) error {
			app, err := t.startApp(ctx, name)
			if err != nil {
				return err
			}

			if t.app != nil {
				panic("t.app != nil")
			}

			t.app = app
			return nil
		},
		Error: done,
	}

	go t.commitChanges(t.outgoingOpenSession)
	return done
}

func (t *appSession) startApp(parent context.Context, name *types.NamespacedName) (app *appcorev1.CliApp, err error) {
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ctx, cancel := timeoutContext(parent)
		defer cancel()
		app, err = t.appClient.CliappV1().CliApps(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if app.Status.Phase == appcorev1.CliAppPhaseLive || app.Spec.TargetPhase == appcorev1.CliAppPhaseLive {
			return nil
		}

		app.Spec.TargetPhase = appcorev1.CliAppPhaseLive
		ctx2, cancel2 := timeoutContext(parent)
		defer cancel2()
		app, err = t.appClient.CliappV1().CliApps(app.Namespace).Update(ctx2, app, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		klog.Errorf("unable to update app %s: %s", name, err)
		return
	}

	if app.Status.Phase == appcorev1.CliAppPhaseLive {
		return
	}

	ctx := context.TODO()
	watcher, err := t.appClient.CliappV1().CliApps(app.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": app.Name}.AsSelector().String(),
	})
	if err != nil {
		klog.Errorf("unable to watch app %s: %s", name, err)
		return
	}

	defer watcher.Stop()

	for {
		select {
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				err = xerrors.Errorf("remote watch closed")
				return
			}

			switch ev.Type {
			case watch.Added, watch.Modified:
				app = ev.Object.(*appcorev1.CliApp)
				if app.Status.Phase == appcorev1.CliAppPhaseLive {
					return
				}
			case watch.Deleted:
				err = xerrors.Errorf("app is deleted")
				return
			case watch.Error:
				st := ev.Object.(*metav1.Status)
				err = xerrors.Errorf("failed %s", st.String())
				return
			default:
				panic(ev.Type)
			}

		case <-ctx.Done():
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				err = xerrors.Errorf("context closed")
			}

			klog.Errorf("watch context exited %s: %s", name, err)
			return
		}
	}
}
