package auth

import "testing"

func TestHashVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password must verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashPasswordSaltsDiffer(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("hashes of the same password must differ (random salt)")
	}
}

func TestVerifyPasswordRejectsGarbage(t *testing.T) {
	if VerifyPassword("not-a-hash", "x") {
		t.Fatal("malformed hash must not verify")
	}
	if VerifyPassword("", "x") {
		t.Fatal("empty hash must not verify")
	}
}
