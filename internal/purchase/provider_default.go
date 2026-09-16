//go:build !darwin

package purchase

import (
	"context"
	"fmt"
)

type unsupportedProvider struct{}

func newProvider() Provider {
	return unsupportedProvider{}
}

func (unsupportedProvider) CurrentState(context.Context) (State, error) {
	return unsupportedState("只有 macOS 版本支援 App 內購"), nil
}

func (unsupportedProvider) Refresh(context.Context) (State, error) {
	return unsupportedState("只有 macOS 版本支援 App 內購"), nil
}

func (unsupportedProvider) PurchaseProUnlock(context.Context) (State, error) {
	return unsupportedState("只有 macOS 版本支援 App 內購"), fmt.Errorf("app 內購目前僅支援 macOS")
}

func (unsupportedProvider) RestorePurchases(context.Context) (State, error) {
	return unsupportedState("只有 macOS 版本支援 App 內購"), fmt.Errorf("app 內購目前僅支援 macOS")
}

func unsupportedState(message string) State {
	return State{
		ProductID:     ProUnlockProductID,
		PlanName:      "Free",
		Source:        "unsupported",
		StatusMessage: message,
		ProUnlock:     false,
		CanPurchase:   false,
		CanRestore:    false,
	}
}
