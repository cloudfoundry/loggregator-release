set -ex
go get -u -d $(awk '/require/{y=1;next}y' go.mod | grep -v ')' | grep -v pinned | grep -v replace | awk '{print $1}')
go mod tidy
go mod vendor
go test ./... -race
