# Quickstart

## Before start
Ensure environment variables exist
```
$ export NETWORK_ADDRESS=<your_public_ip>
$ export MNEMONIC_PHRASE="<your_mnemonic_phrase>"
```

## Start the docker compose
```
$ docker compose up -d
```

## Register SDS peer to meta node
```
$ docker compose exec -it sds ppd terminal

// In PPD terminal:
> rp
> exit
```

## Query SDS wallet address for IPFS node
```
$ docker compose logs ipfs | grep "Sds Node address"
```

You will see the message like this:
```
ipfs-1  | Sds Node address: st1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Please copy and paste the address and send it to us.

## Upload and download files
After the SDS wallet that in IPFS has ozone, you can upload and download files:
```
$ docker compose exec ipfs sh

// in ipfs docker container
# ipfs add <filepath>
```
