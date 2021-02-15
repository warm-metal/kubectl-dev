package app

import (
	"fmt"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type shortcutInstallOptions struct {
	shortcutRoot string
	appRoot      string
	shortcutPath string
	appPath      string
}

func initShortcutInstallOptions() shortcutInstallOptions {
	return shortcutInstallOptions{
		shortcutRoot: "/usr/local/bin",
		appRoot:      "~/.cliapps",
	}
}

func (o *shortcutInstallOptions) init(app string) error {
	if len(o.shortcutRoot) == 0 {
		return xerrors.Errorf("install-base not set")
	}

	if path, found := os.LookupEnv("PATH"); !found || strings.Index(path, o.shortcutRoot) < 0 {
		return xerrors.Errorf("install-base not found in PATH")
	}

	o.shortcutRoot = utils.ExpandTilde(o.shortcutRoot)
	if err := os.MkdirAll(o.shortcutRoot, 0755); err != nil {
		return err
	}

	o.appRoot = utils.ExpandTilde(o.appRoot)
	if err := os.MkdirAll(o.appRoot, 0755); err != nil {
		return err
	}

	o.appPath = filepath.Join(o.appRoot, app)
	o.shortcutPath = filepath.Join(o.shortcutRoot, app)
	return nil
}

const appShortcut = `#!/usr/bin/env bash

kubectl-dev app -n %s --name %s -- $@`

func (o *shortcutInstallOptions) installShortcut(app, namespace string) error {
	for _, path := range []string{o.appPath, o.shortcutPath} {
		_, err := os.Lstat(path)
		if !os.IsNotExist(err) {
			return xerrors.Errorf(`"%s" already exists`, path)
		}
	}

	bootstrap := fmt.Sprintf(appShortcut, namespace, app)
	if err := ioutil.WriteFile(o.appPath, []byte(bootstrap), 0755); err != nil {
		return err
	}

	if err := os.Link(o.appPath, o.shortcutPath); err != nil {
		os.Remove(o.appPath)
		return err
	}

	return nil
}

func (o *shortcutInstallOptions) uninstallShortcut() error {
	if err := os.Remove(o.appPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(o.shortcutPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
