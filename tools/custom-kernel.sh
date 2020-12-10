cat <<EOF | sudo tee /etc/sysconfig/clock
ZONE="Asia/Tokyo"
UTC=true
EOF

sudo ln -sf /usr/share/zoneinfo/Asia/Tokyo /etc/localtime

sudo amazon-linux-extras install -y kernel-ng
sudo reboot

#
# reboot
#

sudo yum -y update

sudo amazon-linux-extras install -y epel
sudo yum-config-manager --disable epel

sudo yum install -y amazon-efs-utils

# Install BCC
sudo yum install -y bison clang-devel clang-libs clang cmake3 elfutils-libelf-devel flex gcc-c++ gcc git llvm-devel llvm-static llvm ncurses-devel python-netaddr zlib-devel
sudo yum --enablerepo=epel install -y iperf
sudo yum install -y http://repo.iovisor.org/yum/extra/mageia/cauldron/x86_64/netperf-2.7.0-1.mga6.x86_64.rpm
sudo pip3 install pyroute2
sudo yum install -y kernel-devel-$(uname -r) libstdc++-static
cd /var/tmp
git clone -b v0.16.0 https://github.com/iovisor/bcc.git
mkdir -p /var/tmp/bcc/build
cd /var/tmp/bcc/build
cmake3 ..
make
sudo make install

uname -r
# => 5.4.74-36.135.amzn2.x86_64

sudo mkdir -p /mnt/efs

efs_id=fs-d0cc26f0

echo "$efs_id:/ /mnt/efs efs _netdev 0 0" | sudo tee -a /etc/fstab
sudo reboot

#
# reboot
#

cd
mkdir reproduce
cd reproduce

cat <<EOF > efs_open.c
#include <err.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
  int fd;

  if (argc < 2) {
    printf("Usage: %s <file_path>\n", argv[0]);
    return 0;
  }

  while (1) {
    fd = open(argv[1], O_RDONLY);
    if (fd == -1) {
      err(EXIT_FAILURE, "Failed to open()");
    }
    if (close(fd) == -1) {
      err(EXIT_FAILURE, "Failed to close()");
    }
  }
  return 0;
}
EOF

cat <<'EOF' > run.sh
#!/bin/bash
set -eu
trap 'kill $(jobs -p)' EXIT
if [[ $# != 1 ]]; then
  echo "Usage: $0 <file_path>" >&2
  exit 0
fi
gcc ./efs_open.c -o ./efs_open >&2
./efs_open "$1" &
echo "$!"
./efs_open "$1" &
echo "$!"
sleep 60
kill %1
kill %2
EOF

chmod +x run.sh
sudo touch /mnt/efs/foo

sudo yum groupinstall -y "Development Tools"
sudo yum install -y ncurses-devel hmaccalc zlib-devel binutils-devel elfutils-libelf-devel openssl-devel

cd

cat <<'EOF' > install-kernel.sh
#!/bin/bash
set -eux

dir="$(pwd)"

if [[ $# < 2 ]]; then
  echo "Usage: $0 <kernel version> <ENA version> [torvalds]" >&2
  exit 0
fi

version="$1"
ena_version="$2"

if [[ $# = 3 ]] && [[ $3 = 'torvalds' ]]; then
  url="https://git.kernel.org/torvalds/t/linux-$version.tar.gz"
  wget "$url"
  tar xvf "linux-$version.tar.gz"
else
  major="$(echo "$version" | cut -d. -f1)"
  url="https://cdn.kernel.org/pub/linux/kernel/v$major.x/linux-$version.tar.xz"
  wget "$url"
  tar xvf "linux-$version.tar.xz"
fi

cd "$dir/linux-$version"
cp "/boot/config-$(uname -r)" .config
yes '' | make "-j$(($(nproc) + 1))"
make modules_install
make headers_install
make install

ena_url="https://github.com/amzn/amzn-drivers/archive/ena_linux_$ena_version.tar.gz"

cd "$dir"
wget "$ena_url"
tar xvzf "ena_linux_$ena_version.tar.gz"
cd "$dir/amzn-drivers-ena_linux_$ena_version/kernel/linux/ena"
BUILD_KERNEL="$version" make "-j$(($(nproc) + 1))"
mkdir -p "/lib/modules/$version/kernel/drivers/amazon/net/ena"
cp ena.ko "/lib/modules/$version/kernel/drivers/amazon/net/ena"
depmod -F "/boot/System.map-$version" "$version"

gzip Module.symvers -c > "/boot/symvers-$version.gz"
cp "$dir/linux-$version/.config" "/boot/config-$version"
chmod +x "/boot/vmlinuz-$version"
grubby --set-default="/boot/vmlinuz-$version"

shutdown -h now
EOF

chmod +x install-kernel.sh

#
# Snapshot as AMI
#
