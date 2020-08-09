module go.jonnrb.io/egress

go 1.14

require (
	docker.io/go-docker v1.0.0
	github.com/Microsoft/go-winio v0.4.11 // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.7.4
	github.com/coreos/go-iptables v0.4.0 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v0.0.0-00010101000000-000000000000 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200626054723-37f83d1996bc
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/vishvananda/netlink v1.1.1-0.20200802231818-98629f7ffc4b
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200121082415-34d275377bf9
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
)

replace github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
