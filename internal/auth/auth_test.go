package auth

import "testing"

func TestHashTokenStable(t *testing.T) {
	a := HashToken("secret")
	b := HashToken("secret")
	if a != b {
		t.Fatal("hash must be deterministic")
	}
	if a == HashToken("other") {
		t.Fatal("different tokens must hash differently")
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex must be 64 chars, got %d", len(a))
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if a == "" {
		t.Fatal("token must not be empty")
	}
}

func TestBearer(t *testing.T) {
	h := header{"Authorization": "Bearer abc"}
	if got := bearer(h); got != "abc" {
		t.Fatalf("got %q, want abc", got)
	}
	h2 := header{"Authorization": "xyz"}
	if got := bearer(h2); got != "xyz" {
		t.Fatalf("bare token: got %q, want xyz", got)
	}
	if got := bearer(header{}); got != "" {
		t.Fatalf("missing: got %q, want empty", got)
	}
}

type header map[string]string

func (h header) Get(k string) string { return h[k] }
