package kubectl

import "io"

func Exec(pod, namespace, container string, args ...string) error {
	cmd := []string{
		"exec", "-ti",
		"-n", namespace,
		pod,
		"-c", container,
		"--",
	}
	return runWithIO(append(cmd, args...))
}

func ApplyManifests(manifestPath string) error {
	return run([]string{
		"apply", "--wait", "-f", manifestPath,
	})
}

func ApplyManifestsFromStdin(stdin io.Reader) error {
	return run([]string{"apply", "--wait", "-f", "-"}, WithStdin(stdin))
}

func DeleteManifests(manifestsPath string) error {
	return run([]string{"delete", "--ignore-not-found", "-f", manifestsPath})
}

func Delete(kind, name, namespace string) error {
	return run([]string{"delete", "--ignore-not-found", "-n", namespace, kind, name})
}
