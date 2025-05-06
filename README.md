## Description

### Objective: Make a Pump.Fun new pairs sniper which buys new tokens above $8k market cap.
##### Language: GO (https://github.com/gagliardetto/solana-go)
1. Using Geyser GRPC, subscribe to the PF program and monitor for new mints.

2. Parse the new mint transaction > to get the market cap and account keys required to make a buy transaction.

3. Build the buy transaction.

4. Submit the TX.

5. You want this to have the lowest latency as possible so avoid multiple RPC calls.

6. We just want: Monitor > Parse > Build TX > Submit TX.

GRPC: http://pomaded-lithotomies-xfbhnqagbt-dedicated-bypass.helius-rpc.com:2052/

X-TOKEN: c64985b5-6ff0-4a6c-8ee2-2daf72546f39

RPC: https://pomaded-lithotomies-xfbhnqagbt-dedicated.helius-rpc.com/?api-key=37ba4475-8fa3-4491-875f-758894981943
