package s3

import (
	"context"
	"errors"
	"sync"
)

type StoredObject struct {
	ObjectKey     string
	Content       []byte
	SHA256Hex     string
	EncryptionAlg string
}

type ConsentBlobStore struct {
	mu      sync.RWMutex
	objects map[string]StoredObject
}

func NewConsentBlobStore() *ConsentBlobStore {
	return &ConsentBlobStore{
		objects: make(map[string]StoredObject),
	}
}

func (s *ConsentBlobStore) PutEncryptedPDF(_ context.Context, objectKey string, content []byte, sha256Hex string) error {
	if objectKey == "" || len(content) == 0 || sha256Hex == "" {
		return errors.New("invalid S3 consent artifact parameters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]byte, len(content))
	copy(copied, content)
	s.objects[objectKey] = StoredObject{
		ObjectKey:     objectKey,
		Content:       copied,
		SHA256Hex:     sha256Hex,
		EncryptionAlg: "AES-256-GCM",
	}
	return nil
}

func (s *ConsentBlobStore) GetObject(_ context.Context, objectKey string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.objects[objectKey]
	if !ok {
		return nil, false
	}
	copied := make([]byte, len(obj.Content))
	copy(copied, obj.Content)
	return copied, true
}
