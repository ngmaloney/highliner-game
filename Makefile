BINARY := highliner
CMD    := ./cmd/highliner

.PHONY: run build clean

run:
	go run $(CMD)

build:
	go build -o $(BINARY) $(CMD)

clean:
	rm -f $(BINARY)
