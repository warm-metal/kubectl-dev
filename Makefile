.PHONY: default
default:
	go build -o _output/kubectl-dev ./cmd/dev

.PHONY: mac
mac:
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

.PHONY: linux
linux:
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli
