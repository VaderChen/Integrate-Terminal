//go:build darwin

package purchase

/*
#cgo darwin LDFLAGS: -L${SRCDIR}/native -lintegtermstorekit2 -framework Foundation -framework StoreKit -framework AppKit
#include <stdlib.h>

char *PurchaseBridgeCurrentStateJSON(void);
char *PurchaseBridgeRefreshStateJSON(void);
char *PurchaseBridgePurchaseProUnlockJSON(void);
char *PurchaseBridgeRestorePurchasesJSON(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

type bridgePayload struct {
	ProductID     string `json:"productId"`
	PlanName      string `json:"planName"`
	Source        string `json:"source"`
	StatusMessage string `json:"statusMessage"`
	ProUnlock     bool   `json:"proUnlock"`
	CanPurchase   bool   `json:"canPurchase"`
	CanRestore    bool   `json:"canRestore"`
	Error         string `json:"error"`
}

func bridgeCurrentState() (State, error) {
	return decodeBridgePayload(C.PurchaseBridgeCurrentStateJSON())
}

func bridgeRefreshState() (State, error) {
	return decodeBridgePayload(C.PurchaseBridgeRefreshStateJSON())
}

func bridgePurchaseProUnlock() (State, error) {
	return decodeBridgePayload(C.PurchaseBridgePurchaseProUnlockJSON())
}

func bridgeRestorePurchases() (State, error) {
	return decodeBridgePayload(C.PurchaseBridgeRestorePurchasesJSON())
}

func decodeBridgePayload(raw *C.char) (State, error) {
	if raw == nil {
		return State{}, fmt.Errorf("StoreKit bridge returned no payload")
	}
	defer C.free(unsafe.Pointer(raw))

	var payload bridgePayload
	if err := json.Unmarshal([]byte(C.GoString(raw)), &payload); err != nil {
		return State{}, fmt.Errorf("decode StoreKit payload: %w", err)
	}

	state := State{
		ProductID:     fallbackString(payload.ProductID, ProUnlockProductID),
		PlanName:      fallbackString(payload.PlanName, "Free"),
		Source:        fallbackString(payload.Source, "storekit"),
		StatusMessage: payload.StatusMessage,
		ProUnlock:     payload.ProUnlock,
		CanPurchase:   payload.CanPurchase,
		CanRestore:    payload.CanRestore,
	}
	if payload.Error != "" {
		return state, fmt.Errorf("%s", payload.Error)
	}
	return state, nil
}

func fallbackString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
