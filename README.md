# EASYBLC
This is an easy BTC implement by Go, ant it's a good model for learning blockchain.

## Properties
- [x] blockchain/block
- [x] merkle tree/merkle proof
- [x] base58/base58check
- [x] script P2PKH
- [x] wallet/wallet managr
- [x] UTXO
- [x] pow/difficulty adjust
- [x] transaction pool with priority queue
- [x] p2p network(gossip)
- [x] web server
- [x] command
- [x] rpc server
- [ ] Bloom filter
- [ ] smart contract
- [ ] DAG

## Usage
### config
```yaml
# http server port that provides services externally
http_server_port: 8080

# rpc server port that provides services externally
rpc_server_port: 8081

# the path of leveldb that stores blockchain
leveldb_path: "."

# peer type, 0:FULL 1:Miner 2:SPV
type: 1

# peer port
port: 3006

# peer exposed endpoint
endpoint: localhost:3006

# bootstrap peers
bootstrap_peers:
#- "localhost:3007"
#- "localhost:3008"

# peer sync block interval
block_sync_interval: 0

# address to receive coinbase reward
reward_address: "1QCZvtyk5YeqxXVuqDZNa9toxiP1icDW1f"
```

### start

There are two ways to start a blockchain:  one peer based on command or many peers based on gossip network.

1. one peer based on command

   ```go
   cd cmd/command && go run main.go
   ```

2. many peers based on gossip network

   ```go
   cd cmd/peer && go run main.go
   ```

## blockchain with two blocks
```
blockchain:
        Height:1
        Hash:00000000b03232b62dc54641c91efb399ce8b3055454836db84ec4284e75178f
        PreHash:0000000023b635b886536e6b14c6ccf13fe62f21d12f532f23c210a06cd2deb6
        Time:1648305961951000400
        Nonce:2305843009842186445
        MerkelRoot:905c3afdf14af63902dd36f21c01975fe718ecdaecfb6a1386553922ed6ebc66
        Bits:486604799
        Transactions:
                hash:2a4816c1388b9e8d64673be49845f71fffe1c67b168b407d49309b35667f40d0
                Vins:
                        txid: 0000000000000000000000000000000000000000000000000000000000000000
                        vout: 4294967295
                        coinbaseDataSize: 52
                        coinbaseData: 2022-03-26 22:46:01.9510004 +0800 CST m=+0.043698501
                Vouts:
                        index: 0
                        value: 5000000050
                        address: 13dSQiFAuVUF8T461xDXFWmXMsv8c1w68E
                        scriptPubKey: 0dff83020102ff840001ff82000022ff81030101024f7001ff820001020104436f6465010600010444617461010a00000029ff840005010e00010d00010b01141cd46fb7edaf208389a35b31e1a8f65d12c230f900010c00010a00

                hash:55248c6c4be962e7559f170849545e1501f3beb133cb008c752b77ddfae00db7
                Vins:
                        txid: 85195c04bd91893f6866e4227c244c173baac586215c341c6c388d31ea52d517
                        vout: 0
                        scriptSig: 0dff83020102ff840001ff82000022ff81030101024f7001ff820001020104436f6465010600010444617461010a000000ff96ff840002010b014104639a6183e8afc02c27ca414cdb283c60633ed7b8401e31a143043a147dcdf1eb202575e546a9591383099a52691383879823c9a09d5933681ef2b1212a1e18df00010b01473045022100fd194eba00fc5abbd7ce8f84e8bc67e52a6302111e8096af38c6aa96d3fa933e02201225b61c5f3efde9f1f007b469557933e39e22d94de7467478ed50e5ade5b0b600
                Vouts:
                        index: 0
                        value: 100000000
                        address: 18VkrmcFLGYoL7VBTcA4ezVSBZT3isZiFc
                        scriptPubKey: 0dff83020102ff840001ff82000022ff81030101024f7001ff820001020104436f6465010600010444617461010a00000029ff840005010e00010d00010b011452392367ee6093e30a858cefd1efe6c839d694ed00010c00010a00
                        index: 1
                        value: 4899999950
                        address: 1FnWFrzaLV9QhJnu7QDM7QZj59AxUz6dSP
                        scriptPubKey: 0dff83020102ff840001ff82000022ff81030101024f7001ff820001020104436f6465010600010444617461010a00000029ff840005010e00010d00010b0114a22ce6b0a70a1a7fd8fbd2b14cfcee6445887e4f00010c00010a00

        Height:0
        Hash:0000000023b635b886536e6b14c6ccf13fe62f21d12f532f23c210a06cd2deb6
        PreHash:
        Time:1648293507383649400
        Nonce:452081398
        MerkelRoot:8060d58e4f63b0a76f425de8ef3592c9cab0a50865957560070cb0bf5bef2d7d
        Bits:486604799
        Transactions:
                hash:85195c04bd91893f6866e4227c244c173baac586215c341c6c388d31ea52d517
                Vins:
                        txid: 0000000000000000000000000000000000000000000000000000000000000000
                        vout: 4294967295
                        coinbaseDataSize: 33
                        coinbaseData: 挖矿不容易，且挖且珍惜
                Vouts:
                        index: 0
                        value: 5000000000
                        address: 13dSQiFAuVUF8T461xDXFWmXMsv8c1w68E
                        scriptPubKey: 0dff83020102ff840001ff82000022ff81030101024f7001ff820001020104436f6465010600010444617461010a00000029ff840005010e00010d00010b01141cd46fb7edaf208389a35b31e1a8f65d12c230f900010c00010a00
```