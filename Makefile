.PHONY: build run clean

build:
	go build -o bin/github-sync .

run:
	go run . sync

clean:
	rm -rf bin/
