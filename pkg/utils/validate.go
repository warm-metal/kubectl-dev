package utils

import (
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	"golang.org/x/xerrors"
	"strings"
)

func ValidateDistro(distro string) (appcorev1.CliAppDistro, error) {
	appDistro := appcorev1.CliAppDistro(strings.ToLower(distro))
	switch appDistro {
	case appcorev1.CliAppDistroAlpine, appcorev1.CliAppDistroUbuntu:
		return appDistro, nil
	default:
		return "", xerrors.Errorf("distro must be either alpine or ubuntu.")
	}
}

func ValidateShell(shell string) (appcorev1.CliAppShell, error) {
	appShell := appcorev1.CliAppShell(strings.ToLower(shell))
	switch appShell {
	case appcorev1.CliAppShellBash, appcorev1.CliAppShellZsh:
		return appShell, nil
	default:
		return "", xerrors.Errorf("distro must be either bash or zsh.")
	}
}
