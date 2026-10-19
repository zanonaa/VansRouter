package auth

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSessions(t *testing.T) {
	s, err := LoadSessions(t.TempDir(), "segredo-de-teste")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1800000000, 0)
	s.now = func() time.Time { return now }
	token, err := s.Create(map[string]any{"oidc": true, "oidcName": "Operador"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.Verify(token)
	if err != nil || claims["authenticated"] != true || claims["oidcName"] != "Operador" {
		t.Fatalf("claims: %v erro: %v", claims, err)
	}
	grant, err := s.CreatePasswordChange()
	if err != nil {
		t.Fatal(err)
	}
	if !s.VerifyPasswordChange(grant) || s.VerifyPasswordChange(token) {
		t.Fatal("escopo do grant divergente")
	}
	other, _ := LoadSessions(t.TempDir(), "outro-segredo")
	if _, err := other.Verify(token); err == nil {
		t.Fatal("assinatura incorreta aceita")
	}
	now = now.Add(PasswordChangeAge)
	if s.VerifyPasswordChange(grant) {
		t.Fatal("grant expirado aceito")
	}
	now = now.Add(SessionAge)
	if _, err := s.Verify(token); err == nil {
		t.Fatal("sessão expirada aceita")
	}
}
func TestSignatureAlgorithms(t *testing.T) {
	s, _ := LoadSessions(t.TempDir(), "segredo-de-teste")
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{"authenticated": true})
	raw, _ := token.SignedString(s.secret)
	if _, err := s.Verify(raw); err == nil {
		t.Fatal("algoritmo não emitido pelo backend aceito")
	}
	if _, err := s.Verify("invalido"); err == nil {
		t.Fatal("token inválido aceito")
	}
}
func TestSecretPersistence(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadSessions(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	token, _ := first.Create(nil)
	second, err := LoadSessions(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Verify(token); err != nil {
		t.Fatal("segredo não preservado")
	}
	info, err := os.Stat(filepath.Join(dir, "jwt-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissões: %v", info.Mode())
	}
	if err := os.WriteFile(filepath.Join(dir, "jwt-secret"), []byte(" "), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSessions(dir, ""); err == nil {
		t.Fatal("arquivo vazio substituído")
	}
}
func TestConcurrentSecretCreation(t *testing.T) {
	dir := t.TempDir()
	sessions := make([]*Sessions, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range sessions {
		wg.Add(1)
		go func(i int) { defer wg.Done(); sessions[i], errs[i] = LoadSessions(dir, "") }(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	token, _ := sessions[0].Create(nil)
	for _, s := range sessions {
		if _, err := s.Verify(token); err != nil {
			t.Fatal("segredos diferentes na inicialização concorrente")
		}
	}
}
func TestCookie(t *testing.T) {
	r := httptest.NewRequest("POST", "http://localhost/api/auth/login", nil)
	c := SessionCookie(r, "token", false, false)
	if c.Name != "auth_token" || c.Path != "/" || !c.HttpOnly || c.MaxAge != 86400 || c.Secure || c.SameSite != 2 {
		t.Fatal("cookie de sessão divergente")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	c = SessionCookie(r, "grant", true, false)
	if c.Name != "password_change" || c.MaxAge != 600 || !c.Secure {
		t.Fatal("cookie de grant divergente")
	}
}
func TestPasswords(t *testing.T) {
	if StrongPassword("curta") || StrongPassword("change-me-in-production") || !StrongPassword("senha-longa-para-teste") {
		t.Fatal("política divergente")
	}
	if _, ok := InitialPassword("curta", "production"); ok {
		t.Fatal("senha fraca permitida em produção")
	}
	if p, ok := InitialPassword("", "production"); !ok || !IsPlaceholder(p) {
		t.Fatal("compatibilidade inicial divergente")
	}
	password := strings.Repeat("a", 80)
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(password, hash, "") || VerifyPassword("outra", hash, "") {
		t.Fatal("bcrypt divergente")
	}
	if !VerifyPassword("inicial", "", "inicial") || VerifyPassword("", "", "") {
		t.Fatal("senha inicial divergente")
	}
}
func TestLimiter(t *testing.T) {
	l := NewLimiter()
	now := time.Unix(1800000000, 0)
	l.now = func() time.Time { return now }
	for _, duration := range []int{30, 120, 600, 1800, 1800} {
		for i := 0; i < 5; i++ {
			l.Fail("ip")
		}
		locked, retry := l.Check("ip")
		if !locked || retry != duration {
			t.Fatalf("bloqueio: %v %d", locked, retry)
		}
		now = now.Add(time.Duration(duration) * time.Second)
		if locked, _ := l.Check("ip"); locked {
			t.Fatal("bloqueio não expirou")
		}
	}
	l.Success("ip")
	if locked, _ := l.Check("ip"); locked {
		t.Fatal("sucesso não limpou estado")
	}
	l.Fail("ip")
	now = now.Add(time.Hour + time.Millisecond)
	if left := l.Fail("ip"); left != 4 {
		t.Fatal("janela não reiniciou")
	}
}
