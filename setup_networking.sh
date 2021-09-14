#!/bin/bash


sudo ip netns delete userappnet
sudo ip netns add userappnet
sudo ip link add userapp0 link enp3s0 type macvlan mode bridge
sudo ip link set userapp0 netns userappnet
sudo ip netns exec userappnet ip link set userapp0 up
sudo ip netns exec userappnet ip addr add 10.0.8.29/24 dev userapp0
sudo ip netns exec userappnet ip route add default via 10.0.8.1 dev userapp0

# sudo ip netns delete userapp


# export CNI_PATH=/opt/cni/bin
# export CNI_CONTAINERID=$(printf '%x%x%x%x' $RANDOM $RANDOM $RANDOM $RANDOM)
# export CNI_CONTAINERID=userapp
# export CNI_COMMAND=ADD
# export CNI_NETNS=/var/run/netns/${CNI_CONTAINERID}

# sudo ip netns add ${CNI_CONTAINERID}

# export CNI_IFNAME="eth0"
# sudo -E /opt/cni/bin/bridge < 20-bridge.conf
# export CNI_IFNAME="eth0"
# sudo -E /opt/cni/bin/macvlan < 10-macvlan.conf
# export CNI_IFNAME="lo"
# sudo -E /opt/cni/bin/loopback < 99-loopback.conf


# POD_IP=$(sudo ip netns exec ${CNI_CONTAINERID} ip -4 addr show enp3s0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
# echo $CNI_CONTAINERID
# echo $POD_IP
