package app

import (
	"IntegTERM/internal/credentials"
	"IntegTERM/internal/store"
	"sync"
)

// Tests share a fake namespace to model GUI/service instances and moved files.
// Neither construction nor credential reads/writes touch the user's Keychain.
var appTestCredentials = &appMemoryCredentials{values: make(map[string][]byte)}

type appMemoryCredentials struct {
	mu     sync.Mutex
	values map[string][]byte
}

func (m *appMemoryCredentials) Get(ref string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.values[ref]
	if !ok {
		return nil, credentials.ErrNotFound
	}
	return append([]byte(nil), data...), nil
}
func (m *appMemoryCredentials) Put(ref string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[ref] = append([]byte(nil), data...)
	return nil
}
func (m *appMemoryCredentials) Delete(ref string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, ref)
	return nil
}
func newAppTestStore(dir string) *store.Store {
	return store.NewWithCredentials(dir, appTestCredentials)
}
