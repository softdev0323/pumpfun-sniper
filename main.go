// cmd/sniper/main.go
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	geyser "github.com/weeaa/goyser/yellowstone_geyser"
	yellowstone_geyser_pb "github.com/weeaa/goyser/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"google.golang.org/grpc/metadata"
)

const MinMarketCap = 8000.0

var PumpFunProgramID solana.PublicKey

func init() {
	var err error
	PumpFunProgramID, err = solana.PublicKeyFromBase58(
		"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P",
	)
	if err != nil {
		log.Fatalf("Invalid Pump.Fun program ID: %v", err)
	}
}

func main() {
	// base context
	ctx := context.Background()

	// 1) Load & validate env
	geyserEndpoint := os.Getenv("GEYSER_ENDPOINT")
	if geyserEndpoint == "" {
		geyserEndpoint = "http://pomaded-lithotomies-xfbhnqagbt-dedicated-bypass.helius-rpc.com:2052"
	}
	xToken := os.Getenv("X_TOKEN")
	if xToken == "" {
		xToken = "c64985b5-6ff0-4a6c-8ee2-2daf72546f39"
		// log.Fatal("X_TOKEN environment variable is required")
	}

	// build metadata and authenticated context
	md := metadata.Pairs("x-token", xToken)
	authCtx := metadata.NewOutgoingContext(ctx, md)

	// 2) Prepare RPC client & signer once
	rpcClient := rpc.New(
		"https://pomaded-lithotomies-xfbhnqagbt-dedicated.helius-rpc.com/" +
			"?api-key=37ba4475-8fa3-4491-875f-758894981943",
	)
	signer, err := loadSigner()
	if err != nil {
		log.Fatalf("failed to load signer: %v", err)
	}
	payer := signer.PublicKey()

	// 3) Keep reconnecting on disconnects
	for {
		// use authCtx so that New() also carries x-token
		client, err := geyser.New(authCtx, geyserEndpoint, md)
		if err != nil {
			log.Printf("❌ failed to connect to Geyser: %v — retry in 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Subscribe using authCtx
		streamName := "pumpfun-sniper"
		if err := client.AddStreamClient(authCtx, streamName, yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
			log.Fatalf("❌ AddStreamClient error: %v", err)
		}
		sc := client.GetStreamClient(streamName)
		updatesCh, errsCh := sc.Ch, sc.ErrCh

		fmt.Println("▶️  Listening for Pump.Fun mint events…")

		// Consume until an error occurs
	Consume:
		for {
			select {
			case update := <-updatesCh:
				// DEBUG: detect which one‐of is set
				if acct := update.GetAccount(); acct != nil {
					fmt.Println("DEBUG: got an Account update")
				}
				if tx := update.GetTransaction(); tx != nil {
					fmt.Println("DEBUG: got a Transaction update")
				}

				acct := update.GetAccount()
				if acct == nil {
					continue
				}
				ownerPK := solana.PublicKeyFromBytes(acct.Account.Owner)
				if !ownerPK.Equals(PumpFunProgramID) || !isNewMint(acct.Account.Data) {
					continue
				}

				mintPK := solana.PublicKeyFromBytes(acct.Account.Pubkey)
				supply := parseSupply(acct.Account.Data)
				price := estimatePrice()
				cap := float64(supply) * price
				if cap < MinMarketCap {
					continue
				}

				log.Printf("🆕  Sniping %s – supply=%d, cap=%.2f…", mintPK, supply, cap)
				tx, err := buildBuyTX(ctx, rpcClient, payer, mintPK, signer)
				if err != nil {
					log.Printf("✖️ buildBuyTX error: %v", err)
					continue
				}
				sig, err := rpcClient.SendTransaction(ctx, tx)
				if err != nil {
					log.Printf("✖️ send tx error: %v", err)
				} else {
					log.Printf("✅ Bought %s – sig: %s", mintPK, sig)
				}

			case streamErr := <-errsCh:
				log.Printf("⚠️  stream error: %v", streamErr)
				break Consume
			}
		}

		client.Close()
		log.Println("🔄 Reconnecting in 5s…")
		time.Sleep(5 * time.Second)
	}
}

func loadSigner() (solana.PrivateKey, error) {
	path := os.Getenv("KEYPAIR_PATH")
	if path == "" {
		home := os.Getenv("HOME")
		path = filepath.Join(home, "solana", "id.json")
	}
	return solana.PrivateKeyFromSolanaKeygenFile(path)
}

func isNewMint(data []byte) bool {
	// TODO: inspect the SPL Mint account header for fresh initialization
	return true
}

func parseSupply(data []byte) uint64 {
	if len(data) < 44 {
		return 0
	}
	// SPL Mint: supply at bytes 36..44 (little-endian u64)
	return binary.LittleEndian.Uint64(data[36:44])
}

func estimatePrice() float64 {
	// TODO: derive the token’s initial USD price via PDA or metadata
	return 0.01
}

func buildBuyTX(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	mint solana.PublicKey,
	signer solana.PrivateKey,
) (*solana.Transaction, error) {
	ata, _, err := solana.FindAssociatedTokenAddress(payer, mint)
	if err != nil {
		return nil, err
	}
	_ = ata

	// TODO: replace with the actual Pump.Fun buy instruction(s)
	instructions := []solana.Instruction{
		// &pumpfun.InstructionBuy{…},
	}

	recent, err := rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(payer),
	)
	if err != nil {
		return nil, err
	}
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer) {
			return &signer
		}
		return nil
	})
	return tx, nil
}
