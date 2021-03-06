.PHONY: default
default: sync-manifest
	go build -o _output/kubectl-dev ./cmd/dev

.PHONY: mac
mac: sync-manifest
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

.PHONY: linux
linux: sync-manifest
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli

.PHONY: sync-manifest
sync-manifest:
	hack/sync_manifest.sh
