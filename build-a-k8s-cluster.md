# setup go env
```
apt-get install wget git gcc -y
apt-get install -y epel-release
apt-get install -y yum-utils device-mapper-persistent-data lvm2 net-tools conntrack-tools wget
wget https://storage.googleapis.com/golang/go1.15.1.linux-amd64.tar.gz
tar -xzf go1.15.1.linux-amd64.tar.gz -C /usr/local
sed -i '11i\export GOROOT=/usr/local/go\nexport GOBIN=$GOROOT/bin\nexport PATH=$PATH:$GOBIN\nexport GOPATH=/home/gopath\n' /etc/profile
source /etc/profile
go version
```

# make sure network is available

## turn off the firewall
```
systemctl disable firewalld && systemctl stop firewalld
```

## Disabling SELinux
```
setenforce 0
sed -i 's/SELINUX=enforcing/SELINUX=disabled/g' /etc/selinux/config
```

# ensure iptables 
```
cat <<EOF >  /usr/lib/sysctl.d/00-system.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
EOF

sysctl --system

echo 1 > /proc/sys/net/ipv4/ip_forward
```

## reload system service
```
systemctl daemon-reload
```
# CA
## install cfssl 
```
wget https://pkg.cfssl.org/R1.2/cfssl_linux-amd64
chmod +x cfssl_linux-amd64
sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl

wget https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
chmod +x cfssljson_linux-amd64
sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson

wget https://pkg.cfssl.org/R1.2/cfssl-certinfo_linux-amd64
chmod +x cfssl-certinfo_linux-amd64
sudo mv cfssl-certinfo_linux-amd64 /usr/local/bin/cfssl-certinfo

export PATH=/usr/local/bin:$PATH
```

## use cfssl to generate CA
### ca
ca-config.json：可以定义多个 profiles，分别指定不同的过期时间、使用场景等参数；后续在签名证书时使用某个 profile；   
signing：表示该证书可用于签名其它证书；生成的 ca.pem 证书中 CA=TRUE；   
server auth：表示 client 可以用该 CA 对 server 提供的证书进行验证；   
client auth：表示 server 可以用该 CA 对 client 提供的证书进行验证；  

```
mkdir /root/ssl
cd /root/ssl
cat > ca-config.json << EOF
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "kubernetes": {
        "usages": [
            "signing",
            "key encipherment",
            "server auth",
            "client auth"
        ],
        "expiry": "8760h"
      }
    }
  }
}
EOF
```

“CN”：Common Name，kube-apiserver 从证书中提取该字段作为请求的用户名 (User Name)；浏览器使用该字段验证网站是否合法；   
“O”：Organization，kube-apiserver 从证书中提取该字段作为请求用户所属的组 (Group)；  
```
cat > ca-csr.json << EOF
{
  "CN": "kubernetes",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "CN",
      "ST": "BeiJing",
      "L": "BeiJing",
      "O": "k8s",
      "OU": "System"
    }
  ]
}
EOF
```
```
cfssl gencert -initca ca-csr.json | cfssljson -bare ca
```

### kubernetes
```
cat > kubernetes-csr.json << EOF
{
   "CN": "kubernetes",
   "hosts": [
      "127.0.0.1",
      "10.128.0.26",
      "10.254.0.1",
      "kubernetes",
      "kubernetes.default",
      "kubernetes.default.svc",
      "kubernetes.default.svc.cluster",
      "kubernetes.default.svc.cluster.local"
    ],
   "key": {
        "algo": "rsa",
        "size": 2048
    },
   "names": [
        {
            "C": "CN",
            "ST": "BeiJing",
            "L": "BeiJing",
            "O": "k8s",
            "OU": "System"
        }
    ]
}
EOF
```
```
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=kubernetes kubernetes-csr.json | cfssljson -bare kubernetes
```

### admin
```
cat > admin-csr.json << EOF
{
  "CN": "admin",
  "hosts": [],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "CN",
      "ST": "BeiJing",
      "L": "BeiJing",
      "O": "system:masters",
      "OU": "System"
    }
  ]
}
EOF
```
```
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=kubernetes admin-csr.json | cfssljson -bare admin
```

### kube-proxy
```
cat > kube-proxy-csr.json << EOF
{
  "CN": "system:kube-proxy",
  "hosts": [],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "CN",
      "ST": "BeiJing",
      "L": "BeiJing",
      "O": "k8s",
      "OU": "System"
    }
  ]
}
EOF
```
```
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=kubernetes  kube-proxy-csr.json | cfssljson -bare kube-proxy
```

### distribute CA
```
mkdir -p /etc/kubernetes/ssl
cp *.pem /etc/kubernetes/ssl
```

# install etcd
```
wget https://github.com/coreos/etcd/releases/download/v3.2.12/etcd-v3.2.12-linux-amd64.tar.gz
tar -xvf etcd-v3.2.12-linux-amd64.tar.gz
sudo mv etcd-v3.2.12-linux-amd64/etcd* /usr/local/bin

sudo mkdir -p /var/lib/etcd
```

```
cat > etcd.service << EOF
[Unit]
Description=Etcd Server
After=network.target
After=network-online.target
Wants=network-online.target
Documentation=https://github.com/coreos

[Service]
Type=notify
WorkingDirectory=/var/lib/etcd/
ExecStart=/usr/local/bin/etcd \\
  --name etcd-zr \\
  --cert-file=/etc/kubernetes/ssl/kubernetes.pem \\
  --key-file=/etc/kubernetes/ssl/kubernetes-key.pem \\
  --peer-cert-file=/etc/kubernetes/ssl/kubernetes.pem \\
  --peer-key-file=/etc/kubernetes/ssl/kubernetes-key.pem \\
  --trusted-ca-file=/etc/kubernetes/ssl/ca.pem \\
  --peer-trusted-ca-file=/etc/kubernetes/ssl/ca.pem \\
  --initial-advertise-peer-urls https://10.128.0.26:2380 \\
  --listen-peer-urls https://10.128.0.26:2380 \\
  --listen-client-urls https://10.128.0.26:2379,http://127.0.0.1:2379 \\
  --advertise-client-urls https://10.128.0.26:2379 \\
  --initial-cluster-state new \\
  --data-dir=/var/lib/etcd
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

```
cp etcd.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable etcd
systemctl start etcd
systemctl status etcd
```

```
etcdctl \
   --ca-file=/etc/kubernetes/ssl/ca.pem \
   --cert-file=/etc/kubernetes/ssl/kubernetes.pem \
   --key-file=/etc/kubernetes/ssl/kubernetes-key.pem \
   cluster-health
```

# install flannel
```
wget https://github.com/coreos/flannel/releases/download/v0.9.1/flannel-v0.9.1-linux-amd64.tar.gz
mkdir flannel
tar -xzvf flannel-v0.9.1-linux-amd64.tar.gz -C flannel
sudo cp flannel/{flanneld,mk-docker-opts.sh} /usr/local/bin
```

```
etcdctl --endpoints=https://10.128.0.26:2379 \
  --ca-file=/etc/kubernetes/ssl/ca.pem \
  --cert-file=/etc/kubernetes/ssl/kubernetes.pem \
  --key-file=/etc/kubernetes/ssl/kubernetes-key.pem \
  mkdir /kubernetes/network

etcdctl --endpoints=https://10.128.0.26:2379 \
  --ca-file=/etc/kubernetes/ssl/ca.pem \
  --cert-file=/etc/kubernetes/ssl/kubernetes.pem \
  --key-file=/etc/kubernetes/ssl/kubernetes-key.pem \
  mk /kubernetes/network/config '{"Network":"172.30.0.0/16","SubnetLen":24,"Backend":{"Type":"vxlan"}}'
```

```
cat > flanneld.service << EOF
[Unit]
Description=Flanneld overlay address etcd agent
After=network.target
After=network-online.target
Wants=network-online.target
After=etcd.service
Before=docker.service

[Service]
Type=notify
ExecStart=/usr/local/bin/flanneld \\
  -etcd-cafile=/etc/kubernetes/ssl/ca.pem \\
  -etcd-certfile=/etc/kubernetes/ssl/kubernetes.pem \\
  -etcd-keyfile=/etc/kubernetes/ssl/kubernetes-key.pem \\
  -etcd-endpoints=https://10.128.0.26:2379 \\
  -etcd-prefix=/kubernetes/network
ExecStartPost=/usr/local/bin/mk-docker-opts.sh -k DOCKER_NETWORK_OPTIONS -d /run/flannel/docker
Restart=on-failure

[Install]
WantedBy=multi-user.target
RequiredBy=docker.service
EOF
```

```
sudo mv flanneld.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable flanneld
sudo systemctl start flanneld
systemctl status flanneld
```

# get kubectl
```
wget https://dl.k8s.io/v1.12.4/kubernetes-client-linux-amd64.tar.gz
tar -xzvf kubernetes-client-linux-amd64.tar.gz

sudo cp kubernetes/client/bin/kube* /usr/local/bin/
chmod a+x /usr/local/bin/kube*
export PATH=/root/local/bin:$PATH
```

# setup config
```
# 设置集群参数,--server指定Master节点ip
kubectl config set-cluster kubernetes \
  --certificate-authority=/etc/kubernetes/ssl/ca.pem \
  --embed-certs=true \
  --server=https://10.128.0.26:10285

# 设置客户端认证参数
kubectl config set-credentials admin \
  --client-certificate=/etc/kubernetes/ssl/admin.pem \
  --embed-certs=true \
  --client-key=/etc/kubernetes/ssl/admin-key.pem

# 设置上下文参数
kubectl config set-context kubernetes \
  --cluster=kubernetes \
  --user=admin

# 设置默认上下文
kubectl config use-context kubernetes
```

## create bootstrap.kubeconfig 
kubelet访问kube-apiserver的时候是通过bootstrap.kubeconfig进行用户验证。  
```
#生成token 变量
export BOOTSTRAP_TOKEN=$(head -c 16 /dev/urandom | od -An -t x | tr -d ' ')

cat > token.csv <<EOF
${BOOTSTRAP_TOKEN},kubelet-bootstrap,10001,"system:kubelet-bootstrap"
EOF

mv token.csv /etc/kubernetes/

# 设置集群参数--server为master节点ip
kubectl config set-cluster kubernetes \
  --certificate-authority=/etc/kubernetes/ssl/ca.pem \
  --embed-certs=true \
  --server=https://10.128.0.26:10285 \
  --kubeconfig=bootstrap.kubeconfig

# 设置客户端认证参数
kubectl config set-credentials kubelet-bootstrap \
  --token=${BOOTSTRAP_TOKEN} \
  --kubeconfig=bootstrap.kubeconfig

# 设置上下文参数
kubectl config set-context default \
  --cluster=kubernetes \
  --user=kubelet-bootstrap \
  --kubeconfig=bootstrap.kubeconfig

# 设置默认上下文
kubectl config use-context default --kubeconfig=bootstrap.kubeconfig

mv bootstrap.kubeconfig /etc/kubernetes/
```

## create kube-proxy.kubeconfig
```
# 设置集群参数 --server参数为master ip
kubectl config set-cluster kubernetes \
  --certificate-authority=/etc/kubernetes/ssl/ca.pem \
  --embed-certs=true \
  --server=https://10.128.0.26:10285 \
  --kubeconfig=kube-proxy.kubeconfig

# 设置客户端认证参数
kubectl config set-credentials kube-proxy \
  --client-certificate=/etc/kubernetes/ssl/kube-proxy.pem \
  --client-key=/etc/kubernetes/ssl/kube-proxy-key.pem \
  --embed-certs=true \
  --kubeconfig=kube-proxy.kubeconfig

# 设置上下文参数
kubectl config set-context default \
  --cluster=kubernetes \
  --user=kube-proxy \
  --kubeconfig=kube-proxy.kubeconfig

# 设置默认上下文
kubectl config use-context default --kubeconfig=kube-proxy.kubeconfig
mv kube-proxy.kubeconfig /etc/kubernetes/
```

# setup master
```
wget https://dl.k8s.io/v1.12.4/kubernetes-server-linux-amd64.tar.gz
tar -xzvf kubernetes-server-linux-amd64.tar.gz
cp -r kubernetes/server/bin/{kube-apiserver,kube-controller-manager,kube-scheduler,kubectl,kube-proxy,kubelet} /usr/local/bin/
```

## kube-apiserver
```
cat > kube-apiserver.service << EOF
[Unit]
Description=Kubernetes API Server
Documentation=https://github.com/GoogleCloudPlatform/kubernetes
After=network.target
After=etcd.service

[Service]
ExecStart=/usr/local/bin/kube-apiserver \\
  --logtostderr=true \\
  --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota,NodeRestriction \\
  --advertise-address=10.128.0.26 \\
  --insecure-bind-address=10.128.0.26 \\
  --bind-address=0.0.0.0 \\
  --insecure-port=6443 \\
  --secure-port=10285 \\
  --authorization-mode=Node,RBAC \\
  --runtime-config=rbac.authorization.k8s.io/v1alpha1 \\
  --kubelet-https=true \\
  --enable-bootstrap-token-auth \\
  --token-auth-file=/etc/kubernetes/token.csv \\
  --service-cluster-ip-range=10.254.0.0/16 \\
  --service-node-port-range=8400-10000 \\
  --tls-cert-file=/etc/kubernetes/ssl/kubernetes.pem \\
  --tls-private-key-file=/etc/kubernetes/ssl/kubernetes-key.pem \\
  --client-ca-file=/etc/kubernetes/ssl/ca.pem \\
  --service-account-key-file=/etc/kubernetes/ssl/ca-key.pem \\
  --etcd-cafile=/etc/kubernetes/ssl/ca.pem \\
  --etcd-certfile=/etc/kubernetes/ssl/kubernetes.pem \\
  --etcd-keyfile=/etc/kubernetes/ssl/kubernetes-key.pem \\
  --etcd-servers=https://10.128.0.26:2379 \\
  --enable-swagger-ui=true \\
  --allow-privileged=true \\
  --apiserver-count=3 \\
  --audit-log-maxage=30 \\
  --audit-log-maxbackup=3 \\
  --audit-log-maxsize=100 \\
  --audit-log-path=/var/lib/audit.log \\
  --event-ttl=1h \\
  --v=2
Restart=on-failure
RestartSec=5
Type=notify
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```
```
cp kube-apiserver.service /etc/systemd/system/

systemctl daemon-reload
systemctl enable kube-apiserver
systemctl start kube-apiserver
systemctl status kube-apiserver
```
## kube-controller-manager
```
cat > kube-controller-manager.service << EOF
[Unit]
Description=Kubernetes Controller Manager
Documentation=https://github.com/GoogleCloudPlatform/kubernetes

[Service]
ExecStart=/usr/local/bin/kube-controller-manager \\
  --logtostderr=true  \\
  --address=127.0.0.1 \\
  --master=http://10.128.0.26:6443 \\
  --allocate-node-cidrs=true \\
  --service-cluster-ip-range=10.254.0.0/16 \\
  --cluster-cidr=172.30.0.0/16 \\
  --cluster-name=kubernetes \\
  --cluster-signing-cert-file=/etc/kubernetes/ssl/ca.pem \\
  --cluster-signing-key-file=/etc/kubernetes/ssl/ca-key.pem \\
  --service-account-private-key-file=/etc/kubernetes/ssl/ca-key.pem \\
  --root-ca-file=/etc/kubernetes/ssl/ca.pem \\
  --leader-elect=true \\
  --v=2
Restart=on-failure
LimitNOFILE=65536
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```
```
cp kube-controller-manager.service /etc/systemd/system/

systemctl daemon-reload
systemctl enable kube-controller-manager
systemctl start kube-controller-manager
```

## kube-scheduler
```
cat > kube-scheduler.service << EOF
[Unit]
Description=Kubernetes Scheduler
Documentation=https://github.com/GoogleCloudPlatform/kubernetes

[Service]
ExecStart=/usr/local/bin/kube-scheduler \\
  --logtostderr=true \\
  --address=127.0.0.1 \\
  --master=http://10.128.0.26:6443 \\
  --port=10251 \\
  --leader-elect=true \\
  --v=2
Restart=on-failure
LimitNOFILE=65536
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```
```
cp kube-scheduler.service /etc/systemd/system/

systemctl daemon-reload
systemctl enable kube-scheduler
systemctl start kube-scheduler
```
```
kubectl get componentstatuses
kubectl create clusterrolebinding kubelet-bootstrap --clusterrole=system:node-bootstrapper --user=kubelet-bootstrap
```

# setup node
## install docker
```
wget https://download.docker.com/linux/static/stable/x86_64/docker-17.12.0-ce.tgz
tar -xvf docker-17.12.0-ce.tgz
cp docker/docker* /usr/local/bin

cat > docker.service << EOF
[Unit]
Description=Docker Application Container Engine
Documentation=http://docs.docker.io

[Service]
Environment="PATH=/usr/local/bin:/bin:/sbin:/usr/bin:/usr/sbin"
EnvironmentFile=-/run/flannel/subnet.env
EnvironmentFile=-/run/flannel/docker
ExecStart=/usr/local/bin/dockerd \\
  --exec-opt native.cgroupdriver=cgroupfs \\
  --log-level=error \\
  --log-driver=json-file \\
  --storage-driver=overlay \\
  \$DOCKER_NETWORK_OPTIONS
ExecReload=/bin/kill -s HUP \$MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
Delegate=yes
KillMode=process

[Install]
WantedBy=multi-user.target
EOF

cp docker.service /etc/systemd/system/docker.service

systemctl daemon-reload
systemctl enable docker
systemctl start docker
systemctl status docker
```

## kube-proxy && kubelet
```
wget https://dl.k8s.io/v1.12.4/kubernetes-server-linux-amd64.tar.gz
tar -xzvf kubernetes-server-linux-amd64.tar.gz
cp -r kubernetes/server/bin/{kube-proxy,kubelet} /usr/local/bin/
```
```
sudo mkdir /var/lib/kubelet

cat > kubelet.service << EOF
[Unit]
Description=Kubernetes Kubelet
Documentation=https://github.com/GoogleCloudPlatform/kubernetes
After=docker.service
Requires=docker.service

[Service]
WorkingDirectory=/var/lib/kubelet
ExecStart=/usr/local/bin/kubelet \\
  --address=10.128.0.26 \\
  --hostname-override=10.128.0.26 \\
  --pod-infra-container-image=registry.access.redhat.com/rhel7/pod-infrastructure:latest \\
  --experimental-bootstrap-kubeconfig=/etc/kubernetes/bootstrap.kubeconfig \\
  --kubeconfig=/etc/kubernetes/kubelet.kubeconfig \\
  --cert-dir=/etc/kubernetes/ssl \\
  --container-runtime=docker \\
  --cluster-dns=10.254.0.2 \\
  --cluster-domain=cluster.local \\
  --hairpin-mode promiscuous-bridge \\
  --allow-privileged=true \\
  --serialize-image-pulls=false \\
  --register-node=true \\
  --logtostderr=true \\
  --cgroup-driver=cgroupfs  \\
  --v=2

Restart=on-failure
KillMode=process
LimitNOFILE=65536
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```
```
cp kubelet.service /etc/systemd/system/kubelet.service

systemctl daemon-reload
systemctl enable kubelet
systemctl start kubelet
systemctl status kubelet
```
```
[root@instance-1 aprilandchoco]# kubectl get csr
NAME                                                   AGE   REQUESTOR           CONDITION
node-csr-Z2BO4sfU7zUgLq6rXFcbA3mrWyrFoF8EWGEV_rroa8U   32s   kubelet-bootstrap   Pending
[root@instance-1 aprilandchoco]# kubectl certificate approve node-csr-Z2BO4sfU7zUgLq6rXFcbA3mrWyrFoF8EWGEV_rroa8U
certificatesigningrequest.certificates.k8s.io/node-csr-Z2BO4sfU7zUgLq6rXFcbA3mrWyrFoF8EWGEV_rroa8U approved
[root@instance-1 aprilandchoco]# kubectl get csr
NAME                                                   AGE   REQUESTOR           CONDITION
node-csr-Z2BO4sfU7zUgLq6rXFcbA3mrWyrFoF8EWGEV_rroa8U   67s   kubelet-bootstrap   Approved
```

```
sudo mkdir -p /var/lib/kube-proxy
```

```
cat > kube-proxy.service << EOF
[Unit]
Description=Kubernetes Kube-Proxy Server
Documentation=https://github.com/GoogleCloudPlatform/kubernetes
After=network.target

[Service]
WorkingDirectory=/var/lib/kube-proxy
ExecStart=/usr/local/bin/kube-proxy \\
  --bind-address=10.128.0.26 \\
  --hostname-override=10.128.0.26 \\
  --cluster-cidr=10.254.0.0/16 \\
  --kubeconfig=/etc/kubernetes/kube-proxy.kubeconfig \\
  --logtostderr=true \\
  --v=2
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```
```
cp kube-proxy.service /etc/systemd/system/

systemctl daemon-reload
systemctl enable kube-proxy
systemctl start kube-proxy
systemctl status kube-proxy
```
