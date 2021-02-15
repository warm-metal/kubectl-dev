.PHONY: session-gate
session-gate:
	kubectl dev build -f hack/session-gate/Dockerfile -t docker.io/warmmetal/session-gate:v0.1.0

.PHONY: dev
mac:
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

.PHONY: dev
linux:
	kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli