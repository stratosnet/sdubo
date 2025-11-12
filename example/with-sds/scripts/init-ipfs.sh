#!/bin/bash
set -e

while [ ! -f /data/ipfs/config ]; do
  echo "Waiting for IPFS init..."
  sleep 1
done

SDS_IP=$(nslookup sds 2>/dev/null | \
         awk '/Name:/{p=1} p&&/Address/{print $2; exit}' | \
         head -n1)
if [ -z "$SDS_IP" ]; then
  echo "Error: Unable to resolve SDS IP address"
  exit 1
fi

echo "Running IPFS initialization..."
ipfs config Sds.RPC /ip4/${SDS_IP}/tcp/18281/http
ipfs config --json Sds.Enabled true

echo "IPFS initialization complete!"
