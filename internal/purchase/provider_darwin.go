//go:build darwin

package purchase

import "context"

type darwinProvider struct{}

func newProvider() Provider {
	return darwinProvider{}
}

func (darwinProvider) CurrentState(context.Context) (State, error) {
	return bridgeCurrentState()
}

func (darwinProvider) Refresh(context.Context) (State, error) {
	return bridgeRefreshState()
}

func (darwinProvider) PurchaseProUnlock(context.Context) (State, error) {
	return bridgePurchaseProUnlock()
}

func (darwinProvider) RestorePurchases(context.Context) (State, error) {
	return bridgeRestorePurchases()
}
