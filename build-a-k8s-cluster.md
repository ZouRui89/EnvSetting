# setup go env
```
yum install wget git gcc -y
yum install -y epel-release
yum install -y yum-utils device-mapper-persistent-data lvm2 net-tools conntrack-tools wget
wget https://storage.googleapis.com/golang/go1.12.7.linux-amd64.tar.gz
tar -xzf go1.12.7.linux-amd64.tar.gz -C /usr/local
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
      "10.128.0.21",
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

## install etcd
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
  --initial-advertise-peer-urls https://10.128.0.21:2380 \\
  --listen-peer-urls https://10.128.0.21:2380 \\
  --listen-client-urls https://10.128.0.21:2379,http://127.0.0.1:2379 \\
  --advertise-client-urls https://10.128.0.21:2379 \\
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

## install flannel
```
wget https://github.com/coreos/flannel/releases/download/v0.9.1/flannel-v0.9.1-linux-amd64.tar.gz
mkdir flannel
tar -xzvf flannel-v0.9.1-linux-amd64.tar.gz -C flannel
sudo cp flannel/{flanneld,mk-docker-opts.sh} /usr/local/bin
```

```
etcdctl --endpoints=https://10.128.0.21:2379 \
  --ca-file=/etc/kubernetes/ssl/ca.pem \
  --cert-file=/etc/kubernetes/ssl/kubernetes.pem \
  --key-file=/etc/kubernetes/ssl/kubernetes-key.pem \
  mkdir /kubernetes/network

etcdctl --endpoints=https://10.128.0.21:2379 \
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
  -etcd-endpoints=https://10.128.0.21:2379 \\
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

