module ha-bridge

go 1.15

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628

require (
	github.com/google/gopacket v1.1.19
	github.com/spf13/pflag v1.0.3
	github.com/vishvananda/netlink v1.1.0 // indirect
	k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/klog/v2 v2.6.0
	kubevirt.io/client-go v0.19.0
)
