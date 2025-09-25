
all:
	go generate ./...
	go get ./...
	go install ./...
	cd extensions/sample; make

# Build gnostic executable for tests
gnostic:
	go build -o gnostic .

test: gnostic
	# since some tests call separately-built binaries, clear the cache to ensure all get run
	go clean -testcache
	PATH=$(PWD):$$PATH go test ./... -v
