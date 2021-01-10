.PHONY: default
default:
	go build -o _output/kubectl-dev .

.PHONY: install
install:
	go install .
