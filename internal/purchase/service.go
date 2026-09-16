package purchase

import "context"

const ProUnlockProductID = "pro_unlock"

type State struct {
	ProductID     string
	PlanName      string
	Source        string
	StatusMessage string
	ProUnlock     bool
	CanPurchase   bool
	CanRestore    bool
}

type Provider interface {
	CurrentState(ctx context.Context) (State, error)
	Refresh(ctx context.Context) (State, error)
	PurchaseProUnlock(ctx context.Context) (State, error)
	RestorePurchases(ctx context.Context) (State, error)
}

type Service struct {
	provider Provider
}

func NewService() *Service {
	return &Service{provider: newProvider()}
}

func (s *Service) CurrentState(ctx context.Context) (State, error) {
	return s.provider.CurrentState(ctx)
}

func (s *Service) Refresh(ctx context.Context) (State, error) {
	return s.provider.Refresh(ctx)
}

func (s *Service) PurchaseProUnlock(ctx context.Context) (State, error) {
	return s.provider.PurchaseProUnlock(ctx)
}

func (s *Service) RestorePurchases(ctx context.Context) (State, error) {
	return s.provider.RestorePurchases(ctx)
}
