package companion

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	secrets := []string{
		"ghp_abcdefghijklmnopqrstuvwxyz0123",
		"AKIAABCDEFGHIJKLMNOP",
		"sk-proj-abcdefghijklmnopqrstuvwx",
		"hunter2pass",
		"eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0.SflKxwRJSMeKKF2QT4",
		"s3cretpw",
	}
	in := "ok line\nGITHUB=ghp_abcdefghijklmnopqrstuvwxyz0123\nkey AKIAABCDEFGHIJKLMNOP\nOPENAI sk-proj-abcdefghijklmnopqrstuvwx\nDB_PASSWORD=hunter2pass\nAuthorization: Bearer eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0.SflKxwRJSMeKKF2QT4\nurl https://bob:s3cretpw@host/x\n"
	out, n := RedactSecrets([]byte(in))
	for _, s := range secrets {
		if strings.Contains(string(out), s) {
			t.Fatalf("leaked %s in %s", s, out)
		}
	}
	if n == 0 || !strings.Contains(string(out), "ok line") || !strings.Contains(string(out), "DB_PASSWORD=[REDACTED]") {
		t.Fatal(string(out))
	}
	if clean, c := RedactSecrets([]byte("PASS: TestFoo (0.01s)\n")); c != 0 || string(clean) != "PASS: TestFoo (0.01s)\n" {
		t.Fatal("redacted normal test output")
	}
}

func TestRedactsStructuredAndOpaqueCredentials(t *testing.T) {
	for _, input := range []string{
		`{"access_token":"opaque-value","refresh_token":"opaque-refresh"}`,
		`{'api_key': 'opaque-value'}`,
		`Authorization: Bearer tiny`,
		`Cookie: session=opaque-value; csrf=opaque-refresh`,
		`Set-Cookie: session=opaque-value; HttpOnly`,
		`backend sb_secret_abcdefghijklmnopqrstuvwxyz`,
		`-----BEGIN PRIVATE KEY-----` + "\nopaque-value\n" + `-----END PRIVATE KEY-----`,
	} {
		out, count := RedactSecrets([]byte(input))
		if count < 1 {
			t.Fatalf("no credential redacted: %q", input)
		}
		for _, value := range []string{"opaque-value", "opaque-refresh", "tiny", "sb_secret_abcdefghijklmnopqrstuvwxyz"} {
			if strings.Contains(string(out), value) {
				t.Fatalf("structured credential not redacted")
			}
		}
		second, again := RedactSecrets(out)
		if again != 0 || string(second) != string(out) {
			t.Fatalf("redaction must be idempotent: fixture=%q once=%q twice=%q count=%d", input, out, second, again)
		}
	}
}

func TestRedactsUnterminatedQuotedCredentialFragments(t *testing.T) {
	for _, quote := range []string{`"`, `'`} {
		for _, suffix := range []string{"", "\nnormal output"} {
			out, count := RedactSecrets([]byte("password=" + quote + "alpha beta gamma" + suffix))
			if count != 1 || strings.Contains(string(out), "alpha") || strings.Contains(string(out), "beta") || strings.Contains(string(out), "gamma") {
				t.Fatal("truncated quoted credential leaked")
			}
		}
	}
}
