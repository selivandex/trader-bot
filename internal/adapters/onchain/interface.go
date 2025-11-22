package onchain

import (
	"context"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// OnChainProvider is interface for on-chain data sources
// Supports multiple providers: Whale Alert, Blockchain.com, Etherscan, etc
type OnChainProvider interface {
	// GetName returns provider name
	GetName() string

	// IsEnabled returns whether provider is enabled
	IsEnabled() bool

	// FetchRecentTransactions fetches large transactions from blockchain
	FetchRecentTransactions(ctx context.Context, minValueUSD int) ([]models.WhaleTransaction, error)

	// SupportedChains returns list of supported blockchains
	SupportedChains() []string

	// GetCost returns approximate cost per request
	GetCost() float64
}

// OnChainAggregator combines multiple on-chain providers
type OnChainAggregator struct {
	providers []OnChainProvider
}

// NewOnChainAggregator creates new aggregator
func NewOnChainAggregator(providers []OnChainProvider) *OnChainAggregator {
	return &OnChainAggregator{providers: providers}
}

// FetchFromAll fetches from all enabled providers and merges results
func (oa *OnChainAggregator) FetchFromAll(ctx context.Context, minValueUSD int) ([]models.WhaleTransaction, error) {
	allTransactions := []models.WhaleTransaction{}
	seenHashes := make(map[string]bool)

	for _, provider := range oa.providers {
		if !provider.IsEnabled() {
			continue
		}

		txs, err := provider.FetchRecentTransactions(ctx, minValueUSD)
		if err != nil {
			// Log error but continue with other providers
			continue
		}

		// Deduplicate by hash
		for _, tx := range txs {
			if !seenHashes[tx.TxHash] {
				allTransactions = append(allTransactions, tx)
				seenHashes[tx.TxHash] = true
			}
		}
	}

	return allTransactions, nil
}

// GetEnabledProviders returns list of enabled providers
func (oa *OnChainAggregator) GetEnabledProviders() []OnChainProvider {
	enabled := []OnChainProvider{}
	for _, p := range oa.providers {
		if p.IsEnabled() {
			enabled = append(enabled, p)
		}
	}
	return enabled
}
