all: build

help: # Display help
	@awk -F ':|##' \
		'/^[^\t].+?:.*?##/ {\
			printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF \
		}' $(MAKEFILE_LIST)

test:
	(cd tests; go test -parallel 20)

build:
	docker build -t vicary:$(git describe --always)

.PHONY:build test
