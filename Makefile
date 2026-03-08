BINARY := highliner
CMD    := ./cmd/highliner

.PHONY: run build clean release-dry

run:
	go run $(CMD)

build:
	go build -o $(BINARY) $(CMD)

clean:
	rm -f $(BINARY)

# Test release build locally (requires goreleaser)
release-dry:
	goreleaser release --snapshot --clean
