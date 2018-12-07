egress
======

Since I use containers in a weird way (for home networking), I need some weird
glue to make network egress work for containers and other clients alike. Most
container networking assumes you either want to limit access between containers
or control access to the internet. I need those things a bit, but what I
*really* need is the ability to direct default-route-bound traffic to the uplink
of my choosing, whether that be Wireguard, OpenVPN, or just a plain old physical
NIC.

I've been using `cmd/init` on a pure Docker system for a while and it works
pretty well in that scenario, but I'm going to try and make this work on a
Kubernetes system, which requires a bit more thought to do correctly.

The resulting containers are built in
[https://git.jonnrb.com/git/network\_containers](https://git.jonnrb.com/git/network_containers).
