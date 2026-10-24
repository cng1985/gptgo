package app

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validateLogin(email, password string) map[string]string {
	fields := map[string]string{}
	if message := validateEmail(email); message != "" {
		fields["email"] = message
	}
	if password == "" {
		fields["password"] = "请填写密码"
	} else if hasControl(password) {
		fields["password"] = "密码包含无法使用的字符"
	}
	return fields
}

func validateRegister(name, email, password, confirm string) map[string]string {
	fields := map[string]string{}
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		fields["name"] = "请填写昵称"
	case utf8.RuneCountInString(name) > 32:
		fields["name"] = "昵称最多 32 个字"
	case hasControl(name):
		fields["name"] = "昵称不能包含换行或控制字符"
	}
	if message := validateEmail(email); message != "" {
		fields["email"] = message
	}
	switch {
	case password == "":
		fields["password"] = "请填写密码"
	case utf8.RuneCountInString(password) < 8:
		fields["password"] = "密码至少 8 位"
	case len(password) > 72:
		fields["password"] = "密码最长 72 字节"
	case hasControl(password):
		fields["password"] = "密码包含无法使用的字符"
	}
	if password != "" && confirm != password {
		fields["confirm"] = "两次密码不一致"
	}
	return fields
}

func validateEmail(email string) string {
	email = strings.TrimSpace(email)
	switch {
	case email == "":
		return "请填写邮箱"
	case len(email) > 254:
		return "邮箱过长"
	case strings.ContainsAny(email, " \t\r\n"):
		return "邮箱格式不正确"
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(addr.Address, email) {
		return "邮箱格式不正确"
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || parts[0] == "" || !strings.Contains(parts[1], ".") {
		return "邮箱格式不正确"
	}
	return ""
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
