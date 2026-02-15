.PHONY: build run clean lint

build:
	go build -o bin/github-sync .

run:
	go run . sync

lint:
	golangci-lint run

clean:
	rm -rf bin/
