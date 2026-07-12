package service

import "testing"

func TestNormalizePKUUsernameAcceptsStudentEmailAndID(t *testing.T) {
	tests := map[string]string{
		" 1234567890 ":               "1234567890",
		"1234567890@stu.pku.edu.cn":  "1234567890",
		"1234567890@STU.PKU.EDU.CN ": "1234567890",
		"teacher@pku.edu.cn":         "teacher",
		"unrelated@example.com":      "unrelated@example.com",
	}
	for input, want := range tests {
		if got := normalizePKUUsername(input); got != want {
			t.Errorf("normalizePKUUsername(%q) = %q, want %q", input, got, want)
		}
	}
}
