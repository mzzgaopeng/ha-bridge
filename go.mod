module ha-bridge

go 1.15

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628

require (
	github.com/google/gopacket v1.1.19
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/vishvananda/netlink v1.1.0
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/klog/v2 v2.6.0
	k8s.io/sample-controller v0.20.4 // indirect
	kubevirt.io/client-go v0.19.0
)
