run: check test

test:
    go test -race ./...

cover: test
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out
    rm coverage.out

check:
    go vet ./...
    gofmt -s -l . | grep -v vendor | tee /dev/stderr
    go build ./...