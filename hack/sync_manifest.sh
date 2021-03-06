#!/usr/bin/env bash

set -e

containerdManifestURL="https://raw.githubusercontent.com/warm-metal/cliapp/master/config/samples/containerd.yaml"
minikubeManifestURL="https://raw.githubusercontent.com/warm-metal/cliapp/master/config/samples/minikube.yaml"

containerdManifests=$(curl -kL "${containerdManifestURL}")
minikubeManifests=$(curl -kL "${minikubeManifestURL}")

tee pkg/cmd/prepare_manifests.go <<- EOF > /dev/null
package cmd

const latestContainerdManifests = "${containerdManifestURL}"
const latestMinikubeManifests = "${minikubeManifestURL}"

const containerdManifests = \`${containerdManifests}
\`
const minikubeManifests = \`${minikubeManifests}
\`
EOF

set +e
