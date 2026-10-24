GO ?= go
OS := $(shell $(GO) env GOOS)
ARCH := $(shell $(GO) env GOARCH)
EXT := so
ifeq ($(OS),darwin)
EXT := dylib
endif
ifeq ($(OS),windows)
EXT := dll
endif
.PHONY: build test ci web preview dev clean
build:
	GO=$(GO) python3 scripts/build_native.py --target $(OS)_$(ARCH)
test:
	GO=$(GO) python3 scripts/ci.py
web:
	python3 scripts/build_web.py
dev:
	$(GO) run ./cmd/dev
clean:
	rm -f dist/cpa-spark.so dist/cpa-spark.dylib dist/cpa-spark.dll dist/cpa-spark.h dist/cpa-spark-dev

ci:
	python3 scripts/ci.py
preview:
	python3 design/readable-preview/build_preview.py
	node design/readable-preview/check_preview.cjs
