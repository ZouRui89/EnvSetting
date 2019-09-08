# setup go env
    1  yum install wget git gcc -y
    yum install -y epel-release
    yum install -y yum-utils device-mapper-persistent-data lvm2 net-tools conntrack-tools wget
    2  wget https://storage.googleapis.com/golang/go1.12.7.linux-amd64.tar.gz
    3  sudo tar -xzf go1.12.7.linux-amd64.tar.gz -C /usr/local
    4    # sed -i '11i\export GOROOT=/usr/local/go\nexport GOBIN=$GOROOT/bin\nexport PATH=$PATH:$GOBIN\nexport GOPATH=/home/gopath\n' /etc/profile
    5  sed -i '11i\export GOROOT=/usr/local/go\nexport GOBIN=$GOROOT/bin\nexport PATH=$PATH:$GOBIN\nexport GOPATH=/home/gopath\n' /etc/profile
    6  source /etc/profile
    7  go version
 [root@instance-1 aprilandchoco]# go version
go version go1.12.7 linux/amd64

# make sure network is available

# turn off the firewall
systemctl disable firewalld && systemctl stop firewalld

# Disabling SELinux
setenforce 0
sed -i 's/SELINUX=enforcing/SELINUX=disabled/g' /etc/selinux/config

# ensure iptables 
cat <<EOF >  /usr/lib/sysctl.d/00-system.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
EOF

sysctl --system

echo 1 > /proc/sys/net/ipv4/ip_forward

# reload system service
systemctl daemon-reload

# install cfssl 
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

# use cfssl to generate ca


