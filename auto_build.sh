#rm  ./cmd/cni/macvlan/macvlan
#rm  ./cmd/ipam/ipam
#rm habridge
export CGO_ENABLED="1"
go build -o habridge ./cmd/
md5sum habridge
docker build -t 10.100.100.200/k8s-deploy/habridge:v1.0 .
docker push 10.100.100.200/k8s-deploy/habridge:v1.0