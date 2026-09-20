package store

import (
	"IntegTERM/internal/credentials"
	"IntegTERM/internal/model"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type memoryCredentials struct {
	mu          sync.Mutex
	values      map[string][]byte
	puts        int
	failPut     int
	failGet     error
	corruptRead bool
	deletes     int
}

func newMemoryCredentials() *memoryCredentials {
	return &memoryCredentials{values: make(map[string][]byte)}
}
func (m *memoryCredentials) Get(ref string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failGet != nil {
		return nil, m.failGet
	}
	data, ok := m.values[ref]
	if !ok {
		return nil, credentials.ErrNotFound
	}
	if m.corruptRead {
		return []byte(`{"password":"incorrect readback"}`), nil
	}
	return append([]byte(nil), data...), nil
}
func (m *memoryCredentials) Put(ref string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	if m.failPut > 0 && m.puts == m.failPut {
		return errors.New("injected credential write failure")
	}
	if _, exists := m.values[ref]; exists {
		return errors.New("immutable credential ref was overwritten")
	}
	m.values[ref] = append([]byte(nil), data...)
	return nil
}
func (m *memoryCredentials) Delete(ref string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes++
	delete(m.values, ref)
	return nil
}
func newTestStore(dir string) *Store { return NewWithCredentials(dir, newMemoryCredentials()) }

func storedRecords(t *testing.T, dir, name string) []diskRecord {
	t.Helper()
	records, err := readJSON[[]diskRecord](filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return records
}
func storedReference(t *testing.T, dir, name string, index int) string {
	t.Helper()
	fields, err := credentialMetadata(storedRecords(t, dir, name)[index])
	if err != nil {
		t.Fatal(err)
	}
	return fields.Reference
}
func assertNoPlaintext(t *testing.T, dir, name string, secrets ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(data, []byte(secret)) {
			t.Fatalf("%s contains plaintext credential", name)
		}
	}
	for _, record := range storedRecords(t, dir, name) {
		if _, ok := record["password"]; ok {
			t.Fatalf("%s retained password field", name)
		}
		if _, ok := record["ppkPassphrase"]; ok {
			t.Fatalf("%s retained passphrase field", name)
		}
	}
}

func TestSitesAndTabsPersistOnlyVerifiedCredentialReferences(t *testing.T) {
	dir := t.TempDir()
	backend := newMemoryCredentials()
	s := NewWithCredentials(dir, backend)
	sites := []model.Site{{ID: "site", Host: "test.invalid", Password: "site-password-test-only", PPKPassphrase: "站台金鑰密語-test"}}
	tabs := []model.Tab{{ID: "tab", SiteID: "site", Password: "tab-password-test-only", PPKPassphrase: "分頁金鑰密語-test"}}
	if err := s.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTabs(tabs); err != nil {
		t.Fatal(err)
	}
	assertNoPlaintext(t, dir, "sites.json", sites[0].Password, sites[0].PPKPassphrase)
	assertNoPlaintext(t, dir, "tabs.json", tabs[0].Password, tabs[0].PPKPassphrase)
	if ref := storedReference(t, dir, "sites.json", 0); len(ref) != len("credential-v1-")+64 {
		t.Fatalf("unexpected reference length: %d", len(ref))
	}
	actualSites, err := s.LoadSites()
	if err != nil || !reflect.DeepEqual(actualSites, sites) {
		t.Fatalf("sites roundtrip mismatch: err=%v", err)
	}
	actualTabs, err := s.LoadTabs()
	if err != nil || !reflect.DeepEqual(actualTabs, tabs) {
		t.Fatalf("tabs roundtrip mismatch: err=%v", err)
	}
}

func TestCredentialReuseAndImmutableReplacement(t *testing.T) {
	dir := t.TempDir()
	backend := newMemoryCredentials()
	s := NewWithCredentials(dir, backend)
	sites := []model.Site{{ID: "site", Password: "old-password-test"}}
	if err := s.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	oldRef := storedReference(t, dir, "sites.json", 0)
	sites[0].Name = "renamed"
	if err := s.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	if storedReference(t, dir, "sites.json", 0) != oldRef || backend.puts != 1 {
		t.Fatal("unchanged credentials created unnecessary entries")
	}
	sites[0].Password = "new-password-test"
	if err := s.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	if storedReference(t, dir, "sites.json", 0) == oldRef {
		t.Fatal("credential change reused a mutable reference")
	}
	value, err := s.readCredential(oldRef)
	if err != nil || value.Password != "old-password-test" {
		t.Fatal("old JSON backup no longer resolves its original credential")
	}
	if err := s.SaveSites(nil); err != nil {
		t.Fatal(err)
	}
	if backend.deletes != 0 {
		t.Fatal("deletion invalidated possible backup references")
	}
	if _, err := s.readCredential(oldRef); err != nil {
		t.Fatal("old reference unexpectedly removed")
	}
}

func TestCredentialReferencesSurviveDataDirectoryMove(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "before")
	after := filepath.Join(root, "after")
	backend := newMemoryCredentials()
	s := NewWithCredentials(dir, backend)
	if err := s.SaveSites([]model.Site{{ID: "site", Password: "moving-secret-test"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, after); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewWithCredentials(after, backend).LoadSites()
	if err != nil || len(loaded) != 1 || loaded[0].Password != "moving-secret-test" {
		t.Fatalf("reference tied to old path: %v", err)
	}
}

func TestLegacyCredentialsMigrateAtomicallyAndPreserveMetadata(t *testing.T) {
	for _, name := range []string{"sites.json", "tabs.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			backend := newMemoryCredentials()
			s := NewWithCredentials(dir, backend)
			original := []byte(`[{"id":"a","password":"legacy-password-test","ppkPassphrase":"legacy-passphrase-test","futureField":{"preserve":true}},{"id":"b","password":""}]`)
			if err := os.WriteFile(filepath.Join(dir, name), original, 0600); err != nil {
				t.Fatal(err)
			}
			if name == "sites.json" {
				records, err := s.LoadSites()
				if err != nil || records[0].Password != "legacy-password-test" {
					t.Fatalf("migration failed: %v", err)
				}
			} else {
				records, err := s.LoadTabs()
				if err != nil || records[0].PPKPassphrase != "legacy-passphrase-test" {
					t.Fatalf("migration failed: %v", err)
				}
			}
			assertNoPlaintext(t, dir, name, "legacy-password-test", "legacy-passphrase-test")
			if _, ok := storedRecords(t, dir, name)[0]["futureField"]; !ok {
				t.Fatal("migration dropped unknown metadata")
			}
			if backend.puts != 1 {
				t.Fatalf("unexpected credential writes: %d", backend.puts)
			}
			if name == "sites.json" {
				_, _ = s.LoadSites()
			} else {
				_, _ = s.LoadTabs()
			}
			if backend.puts != 1 {
				t.Fatal("repeated load created duplicate credentials")
			}
		})
	}
}

func TestMigrationFailurePreservesOriginalJSONAndReturnsNoPartialData(t *testing.T) {
	for _, failure := range []string{"second-put", "readback-error", "readback-mismatch", "json-commit"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			backend := newMemoryCredentials()
			s := NewWithCredentials(dir, backend)
			original := []byte("[\n {\"id\":\"a\",\"password\":\"first-secret-test\"},\n {\"id\":\"b\",\"password\":\"second-secret-test\"}\n]")
			path := filepath.Join(dir, "sites.json")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "second-put":
				backend.failPut = 2
			case "readback-error":
				backend.failGet = credentials.ErrDenied
			case "readback-mismatch":
				backend.corruptRead = true
			case "json-commit":
				s.writeRecords = func(string, any) error { return errors.New("injected atomic write failure") }
			}
			records, err := s.LoadSites()
			if err == nil || records != nil {
				t.Fatal("failed migration returned successful or partial records")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatal("failed migration changed original JSON bytes")
			}
			if backend.deletes != 0 {
				t.Fatal("failed migration deleted potentially referenced credentials")
			}
		})
	}
}

func TestFailedSaveDoesNotChangePreviousCredentialOrJSON(t *testing.T) {
	dir := t.TempDir()
	backend := newMemoryCredentials()
	s := NewWithCredentials(dir, backend)
	original := []model.Site{{ID: "site", Password: "old-secret-test"}}
	if err := s.SaveSites(original); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "sites.json"))
	oldRef := storedReference(t, dir, "sites.json", 0)
	s.writeRecords = func(string, any) error { return errors.New("injected commit failure") }
	if err := s.SaveSites([]model.Site{{ID: "site", Password: "new-secret-test"}}); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "sites.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed save replaced committed JSON")
	}
	value, err := s.readCredential(oldRef)
	if err != nil || value.Password != "old-secret-test" {
		t.Fatal("failed save changed original secret")
	}
	loaded, err := NewWithCredentials(dir, backend).LoadSites()
	if err != nil || !reflect.DeepEqual(loaded, original) {
		t.Fatalf("committed data no longer readable: %v", err)
	}
}

func TestMissingDeniedOrCorruptCredentialNeverLoadsAsEmptySuccess(t *testing.T) {
	for _, failure := range []string{"missing", "denied", "corrupt"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			backend := newMemoryCredentials()
			s := NewWithCredentials(dir, backend)
			if err := s.SaveSites([]model.Site{{ID: "site", Password: "real-secret-test"}}); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(dir, "sites.json"))
			ref := storedReference(t, dir, "sites.json", 0)
			switch failure {
			case "missing":
				delete(backend.values, ref)
			case "denied":
				backend.failGet = credentials.ErrDenied
			case "corrupt":
				backend.values[ref] = []byte(`{}`)
			}
			records, err := s.LoadSites()
			if err == nil || records != nil {
				t.Fatal("credential failure silently cleared credentials")
			}
			if failure == "missing" && !errors.Is(err, credentials.ErrNotFound) {
				t.Fatalf("missing-credential cause was lost: %v", err)
			}
			if failure == "denied" && !errors.Is(err, credentials.ErrDenied) {
				t.Fatalf("denied cause was lost: %v", err)
			}
			after, _ := os.ReadFile(filepath.Join(dir, "sites.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("credential load failure altered JSON")
			}
		})
	}
}

func TestConcurrentMigrationCreatesOnlyOneReferencePerRecord(t *testing.T) {
	dir := t.TempDir()
	backend := newMemoryCredentials()
	original := []byte(`[{"id":"site","password":"shared-secret-test"}]`)
	if err := os.WriteFile(filepath.Join(dir, "sites.json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loaded, err := NewWithCredentials(dir, backend).LoadSites()
			if err != nil || len(loaded) != 1 || loaded[0].Password != "shared-secret-test" {
				t.Errorf("concurrent migration failed: %v", err)
			}
		}()
	}
	wg.Wait()
	if backend.puts != 1 {
		t.Fatalf("migration was not serialized: %d credential writes", backend.puts)
	}
}

func TestNilBackendFailsClosedForCredentials(t *testing.T) {
	s := NewWithCredentials(t.TempDir(), nil)
	if err := s.SaveSites([]model.Site{{Password: "secret-test"}}); err == nil {
		t.Fatal("nil backend used insecure storage")
	}
	if err := s.SaveSites([]model.Site{{ID: "no-secret"}}); err != nil {
		t.Fatal(err)
	}
}

var _ credentials.Backend = (*memoryCredentials)(nil)
