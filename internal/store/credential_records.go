package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// These fields exist only in the persistence format. API models continue to
// carry hydrated credentials and never expose a Keychain reference.
type credentialFields struct {
	Reference     string `json:"credentialRef,omitempty"`
	Password      string `json:"password,omitempty"`
	PPKPassphrase string `json:"ppkPassphrase,omitempty"`
}

type credentialValue struct {
	Password      string `json:"password,omitempty"`
	PPKPassphrase string `json:"ppkPassphrase,omitempty"`
}

func (c credentialValue) empty() bool { return c.Password == "" && c.PPKPassphrase == "" }

type diskRecord map[string]json.RawMessage

func credentialMetadata(record diskRecord) (credentialFields, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return credentialFields{}, err
	}
	var fields credentialFields
	if err = json.Unmarshal(data, &fields); err != nil {
		return fields, fmt.Errorf("invalid saved credential metadata: %w", err)
	}
	if fields.Reference != "" && (fields.Password != "" || fields.PPKPassphrase != "") {
		return fields, fmt.Errorf("saved record contains both a credential reference and plaintext credentials")
	}
	return fields, nil
}

func (s *Store) readCredential(ref string) (credentialValue, error) {
	if s.credentials == nil {
		return credentialValue{}, fmt.Errorf("credential backend is not configured")
	}
	data, err := s.credentials.Get(ref)
	if err != nil {
		return credentialValue{}, fmt.Errorf("read saved credential: %w", err)
	}
	var value credentialValue
	if err := json.Unmarshal(data, &value); err != nil || value.empty() {
		return credentialValue{}, fmt.Errorf("saved credential payload is invalid")
	}
	return value, nil
}

func (s *Store) createCredential(value credentialValue) (string, error) {
	if value.empty() {
		return "", nil
	}
	if s.credentials == nil {
		return "", fmt.Errorf("credential backend is not configured")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate credential reference: %w", err)
	}
	ref := "credential-v1-" + hex.EncodeToString(random)
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// References are immutable: changing a password creates a fresh entry, so an
	// older JSON file/backup can never begin resolving to a different password.
	if err := s.credentials.Put(ref, encoded); err != nil {
		return "", fmt.Errorf("save credential: %w", err)
	}
	verified, err := s.credentials.Get(ref)
	if err != nil {
		return "", fmt.Errorf("verify saved credential: %w", err)
	}
	if !bytes.Equal(encoded, verified) {
		return "", fmt.Errorf("saved credential verification failed")
	}
	return ref, nil
}

func stripCredentials(record diskRecord, ref string) {
	delete(record, "password")
	delete(record, "ppkPassphrase")
	delete(record, "credentialRef")
	if ref != "" {
		record["credentialRef"], _ = json.Marshal(ref)
	}
}

func hydrateRecord[T any](record diskRecord, value credentialValue) (T, error) {
	// Never put hydrated values back in the disk map that may be committed below.
	hydrated := make(diskRecord, len(record)+2)
	for key, data := range record {
		hydrated[key] = data
	}
	hydrated["password"], _ = json.Marshal(value.Password)
	hydrated["ppkPassphrase"], _ = json.Marshal(value.PPKPassphrase)
	data, err := json.Marshal(hydrated)
	var out T
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}

func loadCredentialRecords[T any](s *Store, path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []T{}, nil
	}
	if err != nil {
		return nil, err
	}
	// Validate all public fields before creating any Keychain entries.
	var validated []T
	if err := json.Unmarshal(data, &validated); err != nil {
		return nil, err
	}
	var records []diskRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	result := make([]T, 0, len(records))
	changed := false
	for index, record := range records {
		fields, err := credentialMetadata(record)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", index, err)
		}
		value := credentialValue{fields.Password, fields.PPKPassphrase}
		ref := fields.Reference
		if ref != "" {
			value, err = s.readCredential(ref)
			if err != nil {
				return nil, fmt.Errorf("record %d: %w", index, err)
			}
		} else if !value.empty() {
			ref, err = s.createCredential(value)
			if err != nil {
				return nil, fmt.Errorf("migrate record %d: %w", index, err)
			}
			changed = true
		}
		hydrated, err := hydrateRecord[T](record, value)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", index, err)
		}
		result = append(result, hydrated)
		if _, ok := record["password"]; ok {
			changed = true
		}
		if _, ok := record["ppkPassphrase"]; ok {
			changed = true
		}
		stripCredentials(record, ref)
	}
	if changed {
		// One atomic replacement occurs only after every Put/Get has succeeded.
		// On any error, callers receive no partial/empty successful result and the
		// original JSON bytes remain intact.
		if err := s.writeRecords(path, records); err != nil {
			return nil, fmt.Errorf("commit credential migration: %w", err)
		}
	}
	return result, nil
}

func saveCredentialRecords[T any](s *Store, path string, values []T) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var records []diskRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}
	oldRecords, err := readJSON[[]diskRecord](path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	oldByID := make(map[string]diskRecord, len(oldRecords))
	for _, record := range oldRecords {
		var id string
		if err := json.Unmarshal(record["id"], &id); err == nil && id != "" {
			oldByID[id] = record
		}
	}
	for index, record := range records {
		fields, err := credentialMetadata(record)
		if err != nil {
			return fmt.Errorf("record %d: %w", index, err)
		}
		// Model types have no credentialRef; accepting one through a future schema
		// change must not permit the caller to substitute an arbitrary saved secret.
		if fields.Reference != "" {
			return fmt.Errorf("credential references cannot be supplied by callers")
		}
		desired := credentialValue{fields.Password, fields.PPKPassphrase}
		ref := ""
		var id string
		_ = json.Unmarshal(record["id"], &id)
		if old, ok := oldByID[id]; ok {
			previous, err := credentialMetadata(old)
			if err != nil {
				return fmt.Errorf("existing record %d: %w", index, err)
			}
			if previous.Reference != "" {
				existing, err := s.readCredential(previous.Reference)
				if err != nil {
					return fmt.Errorf("existing record %d: %w", index, err)
				}
				if existing == desired {
					ref = previous.Reference
				}
			}
		}
		if ref == "" && !desired.empty() {
			ref, err = s.createCredential(desired)
			if err != nil {
				return fmt.Errorf("record %d: %w", index, err)
			}
		}
		stripCredentials(record, ref)
	}
	// No eager deletion: other sites/tabs, previous JSON snapshots, and backups
	// may still reference old entries. Failed writes can leave harmless orphaned
	// Keychain entries, but cannot invalidate the last committed file.
	return s.writeRecords(path, records)
}
