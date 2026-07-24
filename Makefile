.PHONY: build test install manifest-demo

build:
	go build -o bin/cluster-load-lab ./cmd/cluster-load-lab

test:
	go test ./...

install: build
	install -d "$(DESTDIR)/usr/local/bin"
	install -m 755 bin/cluster-load-lab "$(DESTDIR)/usr/local/bin/cluster-load-lab"

manifest-demo: build
	./bin/cluster-load-lab manifest \
	  --host postgres.default.svc.cluster.local \
	  --user postgres \
	  --driver pgsql
