package utils

import (
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func WaitUntilErrorOr(watcher watch.Interface, breakCb func(runtime.Object) (bool, error)) error {
	defer watcher.Stop()
	for {
		ev := <-watcher.ResultChan()

		switch ev.Type {
		case watch.Modified, watch.Added:
			if exit, err := breakCb(ev.Object); exit {
				return err
			}

		case watch.Error:
			if status, ok := ev.Object.(*metav1.Status); ok {
				return xerrors.Errorf("%s", status)
			}

			return xerrors.Errorf("object error")
		case watch.Deleted:
			return xerrors.Errorf("object deleted")
		}
	}
}
