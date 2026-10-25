package handlers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credentials live outside settings/transfer exports; the installation key must
// be backed up with the data volume, never with the exported database JSON.
type NordAuthStore struct {
	DB   *sql.DB
	aead cipher.AEAD
}
type nordAuthState struct {
	Token   string    `json:"token"`
	Renew   string    `json:"renew"`
	Expires time.Time `json:"expires"`
}

func NewNordAuthStore(db *sql.DB, dir string) (*NordAuthStore, error) {
	path := filepath.Join(dir, "nordvpn-auth.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		_, err = f.Write(key)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	} else if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &NordAuthStore{DB: db, aead: aead}, nil
}
func (s *NordAuthStore) Save(ctx context.Context, v nordAuthState) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	encrypted := s.aead.Seal(nonce, nonce, raw, []byte("nordvpnAuth:v1"))
	_, err = s.DB.ExecContext(ctx, `INSERT INTO nordvpnAuth(id,encrypted) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET encrypted=excluded.encrypted`, encrypted)
	return err
}
func (s *NordAuthStore) Load(ctx context.Context) (nordAuthState, error) {
	var encrypted []byte
	var state nordAuthState
	err := s.DB.QueryRowContext(ctx, `SELECT encrypted FROM nordvpnAuth WHERE id=1`).Scan(&encrypted)
	if err != nil {
		return state, err
	}
	n := s.aead.NonceSize()
	if len(encrypted) < n {
		return state, errors.New("invalid NordVPN credential record")
	}
	raw, err := s.aead.Open(nil, encrypted[:n], encrypted[n:], []byte("nordvpnAuth:v1"))
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}

var nordAuthPersistence struct {
	sync.RWMutex
	store *NordAuthStore
}

func persistNordClient(ctx context.Context, c *NordClient) error {
	nordAuthPersistence.RLock()
	store := nordAuthPersistence.store
	nordAuthPersistence.RUnlock()
	if store == nil {
		return nil
	}
	c.mu.Lock()
	state := nordAuthState{c.token, c.renewToken, c.expiresAt}
	c.mu.Unlock()
	return store.Save(ctx, state)
}
func StartNordAuthPersistence(ctx context.Context, store *NordAuthStore) error {
	state, err := store.Load(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	nordAuthPersistence.Lock()
	nordAuthPersistence.store = store
	nordAuthPersistence.Unlock()
	if err == nil {
		nordManager.mu.Lock()
		nordManager.client = &NordClient{token: state.Token, renewToken: state.Renew, expiresAt: state.Expires}
		nordManager.mu.Unlock()
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				nordManager.mu.RLock()
				c := nordManager.client
				nordManager.mu.RUnlock()
				if c != nil {
					renewCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					_ = c.RenewIfDue(renewCtx)
					cancel()
				}
			}
		}
	}()
	return nil
}
