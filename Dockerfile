from quay.io/jonnrb/go as build-egress
add go.* ./
run go mod download
add . ./
run CGO_ENABLED=0 go get ./cmd/init

from alpine:3.8 as build-base
run apk add --no-cache \
  bash bison bzip2 flex gcc libc-dev libmnl-dev linux-headers lz4-dev lzo-dev \
  make openssl-dev pkgconfig

from build-base as build-iptables
workdir /
env SRC https://www.netfilter.org/projects/iptables/files/iptables-1.8.2.tar.bz2
env SRC_HASH a3778b50ed1a3256f9ca975de82c2204e508001fc2471238c8c97f3d1c4c12af
add $SRC /iptables.tar.bz2
run echo "$SRC_HASH  iptables.tar.bz2" |sha256sum -c - \
 && tar xjf iptables.tar.bz2 \
 && cd iptables* \
 && ./configure --enable-static --disable-shared --disable-nftables \
 && make \
 && make install

from build-base as build-iproute2
env IPROUTE2_SRC https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/snapshot/iproute2-4.19.0.tar.gz
env IPROUTE2_HASH 1f31fc30d030c6e532ff6a59f7bbe3a6fa929f790c600f05c966f358f693a0c7
add $IPROUTE2_SRC /iproute2.tar.gz
run echo "$IPROUTE2_HASH  iproute2.tar.gz" |sha256sum -c - \
 && tar xf iproute2.tar.gz \
 && cd iproute2-* \
 && ./configure \
 && make \
 && make install

from build-base as build-openvpn
env OPENVPN_SRC https://swupdate.openvpn.net/community/releases/openvpn-2.4.6.tar.gz
env OPENVPN_HASH 738dbd37fcf8eb9382c53628db22258c41ba9550165519d9200e8bebaef4cbe2
add $OPENVPN_SRC /openvpn.tar.gz
run echo "$OPENVPN_HASH  openvpn.tar.gz" |sha256sum -c - \
 && tar xf openvpn.tar.gz \
 && cd openvpn* \
 && ./configure --disable-plugin-auth-pam --enable-iproute2 --disable-server \
 && make \
 && make install

from gcr.io/distroless/static as egress
copy --from=build-iptables /usr/local/sbin/xtables-legacy-multi /sbin/iptables
copy --from=build-base /lib/libc* /lib/ld* /lib/
copy --from=build-egress /go/bin/init /init
expose 8080
healthcheck --interval=10s --timeout=5s cmd ["/init", "-health_check"]
entrypoint ["/init"]

from egress as egress-openvpn
copy --from=build-iproute2 /etc/iproute2 /etc/iproute2
copy --from=build-iproute2 /sbin/ip /sbin/ip
copy --from=build-base /usr/lib/libmnl.so* /lib/
copy --from=build-openvpn /usr/local/sbin/openvpn /sbin/openvpn
copy --from=build-base /lib/libcrypto.so* /lib/libssl.so* /usr/lib/liblzo2.so* \
  /usr/lib/liblz4.so* /lib/libz.so* /lib/
entrypoint ["/init", "-c", "openvpn /data/openvpn.conf"]
