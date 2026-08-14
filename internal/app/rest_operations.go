package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const restOperationRetention = time.Hour

type RESTOperation struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type operationEnvelope struct {
	Operation RESTOperation `json:"operation"`
}

type operationsEnvelope struct {
	Operations []RESTOperation `json:"operations"`
}

func (a *App) createRESTOperation(kind string) RESTOperation {
	now := time.Now().Format(time.RFC3339)
	operation := RESTOperation{
		ID:        uuid.NewString(),
		Kind:      kind,
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	a.operationMu.Lock()
	if a.operations == nil {
		a.operations = make(map[string]RESTOperation)
	}
	a.operations[operation.ID] = operation
	a.operationMu.Unlock()
	return operation
}

func (a *App) runRESTOperation(operation RESTOperation, work func() error) {
	a.updateRESTOperation(operation.ID, "running", "")
	defer func() {
		if recovered := recover(); recovered != nil {
			a.updateRESTOperation(operation.ID, "failed", fmt.Sprint(recovered))
		}
	}()
	if err := work(); err != nil {
		a.updateRESTOperation(operation.ID, "failed", err.Error())
		return
	}
	a.updateRESTOperation(operation.ID, "done", "")
}

func (a *App) updateRESTOperation(id string, status string, errorMessage string) {
	a.operationMu.Lock()
	operation, ok := a.operations[id]
	if ok {
		operation.Status = status
		operation.Error = errorMessage
		operation.UpdatedAt = time.Now().Format(time.RFC3339)
		a.operations[id] = operation
	}
	a.operationMu.Unlock()
	if ok && (status == "done" || status == "failed") {
		time.AfterFunc(restOperationRetention, func() {
			a.operationMu.Lock()
			delete(a.operations, id)
			a.operationMu.Unlock()
		})
	}
}

func (a *App) getRESTOperation(id string) (RESTOperation, bool) {
	a.operationMu.RLock()
	defer a.operationMu.RUnlock()
	operation, ok := a.operations[id]
	return operation, ok
}

func (a *App) listRESTOperations() []RESTOperation {
	a.operationMu.RLock()
	defer a.operationMu.RUnlock()
	operations := make([]RESTOperation, 0, len(a.operations))
	for _, operation := range a.operations {
		operations = append(operations, operation)
	}
	return operations
}

func (a *App) handleRESTOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/operations/")
	if id == r.URL.Path || strings.TrimSpace(id) == "" {
		writeJSON(w, http.StatusOK, operationsEnvelope{Operations: a.listRESTOperations()})
		return
	}
	operation, ok := a.getRESTOperation(id)
	if !ok {
		writeError(w, http.StatusNotFound, "operation not found")
		return
	}
	writeJSON(w, http.StatusOK, operationEnvelope{Operation: operation})
}
