package handler

import "testing"

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		want string // "" 表示通过
	}{
		{name: "过短(9位)", pw: "Abc123456", want: "密码至少10位"},
		{name: "过短(3位数字)", pw: "123", want: "密码至少10位"},
		{name: "纯数字", pw: "1234567890", want: "密码需同时包含字母和数字"},
		{name: "纯字母", pw: "abcdefghij", want: "密码需同时包含字母和数字"},
		{name: "合法(字母+数字)", pw: "StrongPass123", want: ""},
		{name: "合法(带符号)", pw: "Abc@2024!!", want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := validatePasswordStrength(c.pw)
			if got != c.want {
				t.Errorf("validatePasswordStrength(%q) = %q, want %q", c.pw, got, c.want)
			}
		})
	}
}
