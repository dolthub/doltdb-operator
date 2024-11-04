load('ext://git_resource', 'git_checkout')
load('ext://namespace', 'namespace_create', 'namespace_inject')
load('ext://helm_remote', 'helm_remote')
load('ext://helm_resource', 'helm_resource', 'helm_repo')
git_checkout('REDACTED', checkout_dir="tilt_git/common_tilt")
_mydir = os.path.abspath(os.path.dirname(__file__))

load('tilt_git/common_tilt/Tiltfile', 'install_argocd', 'install_istio', 'install_vault', 'install_external_secrets', 'install_cert_manager', 'setup_harbor_repo', 'deploy_arrival_tools_tobor')


local_resource('go-compile-dolt-operator',
                'CGO_ENABLE=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o manager cmd/main.go',
                deps=["./cmd/main.go", "./api", "./internal" , "./pkg","./internal/controllers"])

#docker_build('localhost:5000/dolt-operator', '.', dockerfile_contents="""
#FROM alpine
#WORKDIR /
#COPY manager /manager .
#ENTRYPOINT ["/manager"]
#              """,
#              only=["./manager"])

docker_build('localhost:5000/dolt-operator', '.', dockerfile="Dockerfile.dev" )
k8s_yaml(kustomize('config/default'))
k8s_resource(workload='dolt-operator-controller-manager')

docker_build('localhost:5000/test-container', '.', dockerfile_contents="""
FROM golang:1.23
WORKDIR /
COPY go.mod go.mod
COPY go.sum go.sum
COPY api/ api/
COPY internal/controller/ internal/controller/
COPY pkg/ pkg/
RUN go mod download
ENTRYPOINT ["go", "test", "./internal/controller/..."]
              """,
              )
k8s_yaml('test/testcontainer.yaml')


 #go test $$(go list ./... | grep -v /e2e) -race -coverprofile cover.out  --timeout 1m
#local_resource('test-runner', cmd='go test cmd/testrunner/*', deps=["./cmd/testrunner", "./cmd/common"], resource_deps=["dolt-operator-controller-manager"])