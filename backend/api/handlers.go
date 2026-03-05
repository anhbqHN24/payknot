package api

import (
	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
)

func isValidWalletAddress(value string) bool {
	if value == "" {
		return false
	}
	_, err := solana.PublicKeyFromBase58(value)
	return err == nil
}

func isValidReference(reference string) bool {
	_, err := uuid.Parse(reference)
	return err == nil
}
