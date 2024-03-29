package utils

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net"
	"net/url"
	"os"
	"strings"
)

func GetSysProxy() (envs []corev1.EnvVar, err error) {
	proxyEnvs := []string{"http_proxy", "https_proxy", "no_proxy"}
	for _, env := range proxyEnvs {
		v, found := os.LookupEnv(env)
		if !found {
			v, found = os.LookupEnv(strings.ToUpper(env))
		}

		if !found {
			fmt.Println(env, "not found")
			continue
		}

		if strings.HasPrefix(env, "http") {
			proxy, err := url.Parse(v)
			if err != nil {
				return nil, fmt.Errorf(`value of environment variable "%s", %s is not invalid: %s`,
					env, v, err)
			}

			if net.ParseIP(proxy.Hostname()).IsLoopback() {
				return nil, fmt.Errorf(`proxy "%s=%s" is a loopback URL. can't work in Pods.'`, env, v)
			}
		}

		envs = append(envs, corev1.EnvVar{
			Name:  env,
			Value: v,
		}, corev1.EnvVar{
			Name:  strings.ToUpper(env),
			Value: v,
		})
	}

	if len(envs) == 0 {
		fmt.Fprintln(os.Stderr, "http proxy configuration not found.")
	}

	return
}

func GetSysProxyEnvs() (envs []string, err error) {
	vars, err := GetSysProxy()
	if err != nil {
		return
	}

	if len(vars) == 0 {
		return
	}

	envs = make([]string, 0, len(vars))
	for _, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}

	return
}
