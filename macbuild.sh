go test $(go list ./... | grep -v /vendor/ | grep -v /template/|grep -v /build/) -cover \
 && VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') \
 && GIT_COMMIT=$(git rev-list -1 HEAD) \
 && CGO_ENABLED=0 go build --ldflags "-s -w \
    -X github.com/ngduchai/faas-cli/version.GitCommit=${GIT_COMMIT} \
    -X github.com/ngduchai/faas-cli/version.Version=${VERSION}" \
    -a -installsuffix cgo -o rts-cli
