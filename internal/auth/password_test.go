package auth

import "testing"

func TestPasswordHashVerifiesOnlyCorrectPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was stored in plain text")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password did not verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("expected empty password to fail")
	}
}
