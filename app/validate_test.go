package app

import "testing"

func TestValidateRegister(t *testing.T) {
	if fields := validateRegister("林夏", "lin@example.com", "correct-horse", "correct-horse"); len(fields) != 0 {
		t.Fatalf("valid form returned %v", fields)
	}
	fields := validateRegister("  ", "not-an-email", "1234567", "1234567")
	if fields["name"] == "" || fields["email"] == "" || fields["password"] == "" {
		t.Fatalf("missing field errors: %v", fields)
	}
	long := make([]rune, 33)
	for i := range long {
		long[i] = '名'
	}
	fields = validateRegister(string(long), "lin@example.com", "correct-horse", "nope")
	if fields["name"] == "" || fields["confirm"] == "" {
		t.Fatalf("length and confirm errors: %v", fields)
	}
}

func TestValidateLogin(t *testing.T) {
	if fields := validateLogin("lin@example.com", "x"); len(fields) != 0 {
		t.Fatalf("unexpected %v", fields)
	}
	fields := validateLogin("", "")
	if fields["email"] == "" || fields["password"] == "" {
		t.Fatalf("missing %v", fields)
	}
}
