// Package auth contém os mecanismos de autenticação, sem expor rotas por si só.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const SessionAge = 24 * time.Hour
const PasswordChangeAge = 10 * time.Minute

type Sessions struct {
	secret []byte
	now    func() time.Time
}

// LoadSessions preserva o segredo existente. Não substitui um arquivo inválido.
func LoadSessions(dataDir, configured string) (*Sessions, error) {
	if configured != "" {
		return &Sessions{secret: []byte(configured), now: time.Now}, nil
	}
	file := filepath.Join(dataDir, "jwt-secret")
	read := func() (*Sessions, error) {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		secret := strings.TrimSpace(string(data))
		if secret == "" {
			return nil, errors.New("arquivo jwt-secret vazio; conteúdo preservado")
		}
		return &Sessions{secret: []byte(secret), now: time.Now}, nil
	}
	session, err := read()
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	// Publicação atômica e exclusiva: leitores nunca observam um segredo parcial.
	temp, err := os.CreateTemp(dataDir, ".jwt-secret-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.WriteString(hex.EncodeToString(random)); err != nil {
		temp.Close()
		return nil, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	if err := os.Link(temp.Name(), file); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("persistir segredo JWT: %w", err)
	}
	return read()
}

func (s *Sessions) issue(claims jwt.MapClaims, age time.Duration) (string, error) {
	now := s.now()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(age).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// jose SignJWT emite apenas alg; remover typ para manter o header emitido.
	delete(token.Header, "typ")
	return token.SignedString(s.secret)
}
func (s *Sessions) Create(claims map[string]any) (string, error) {
	copied := jwt.MapClaims{"authenticated": true}
	for k, v := range claims {
		copied[k] = v
	}
	return s.issue(copied, SessionAge)
}
func (s *Sessions) CreatePasswordChange() (string, error) {
	return s.issue(jwt.MapClaims{"scope": "password-change"}, PasswordChangeAge)
}
func (s *Sessions) Verify(raw string) (jwt.MapClaims, error) {
	if raw == "" {
		return nil, errors.New("sessão ausente")
	}
	token, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return s.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(s.now), jwt.WithoutClaimsValidation())
	if err != nil || !token.Valid {
		return nil, errors.New("sessão inválida")
	}
	claims := token.Claims.(jwt.MapClaims)
	// jose valida exp/nbf quando presentes, mas não exige exp nem valida iat.
	for name := range map[string]bool{"exp": true, "nbf": true, "iat": true} {
		if value, ok := claims[name]; ok {
			if _, ok := value.(float64); !ok {
				return nil, errors.New("claim temporal inválida")
			}
		}
	}
	now := float64(s.now().Unix())
	if exp, ok := claims["exp"].(float64); ok && exp <= now {
		return nil, errors.New("sessão expirada")
	}
	if nbf, ok := claims["nbf"].(float64); ok && nbf > now {
		return nil, errors.New("sessão ainda não válida")
	}
	return claims, nil
}
func (s *Sessions) VerifyPasswordChange(raw string) bool {
	claims, err := s.Verify(raw)
	return err == nil && claims["scope"] == "password-change"
}

func SessionCookie(r *http.Request, token string, passwordChange, forceSecure bool) *http.Cookie {
	name := "auth_token"
	age := SessionAge
	if passwordChange {
		name = "password_change"
		age = PasswordChangeAge
	}
	return &http.Cookie{Name: name, Value: token, Path: "/", HttpOnly: true, Secure: forceSecure || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode, MaxAge: int(age.Seconds())}
}
