#rm  ./cmd/cni/macvlan/macvlan
#rm  ./cmd/ipam/ipam
#rm habridge
export CGO_ENABLED="1"
go build -o habridge ./cmd/
md5sum habridge
docker build -t 192.168.29.235:30443/k8s-deploy/habridge:v1.5 .
docker save -o habridge.tar 192.168.29.235:30443/k8s-deploy/habridge:v1.5
