package mcp

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

func startStdio(ctx context.Context, command string, args []string) (*Session, func(), error) {
	if err := validateCommand(command, args); err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmtError("无法启动 MCP 命令")
	}
	session := newPipeSession(stdin, stdout)
	closeFn := func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	return session, closeFn, nil
}

func validateCommand(command string, args []string) error {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 200 {
		return fmtError("命令长度需要在 1 到 200 个字符之间")
	}
	if strings.Contains(command, "..") || strings.ContainsAny(command, ";&|<>$`\r\n") {
		return fmtError("命令包含不允许的字符")
	}
	if len(args) > 16 {
		return fmtError("参数不能超过 16 个")
	}
	for _, arg := range args {
		if len(arg) > 200 || strings.Contains(arg, "..") || strings.ContainsAny(arg, "\r\n\x00") {
			return fmtError("参数无效")
		}
	}
	return nil
}

type plainError string

func (e plainError) Error() string { return string(e) }

func fmtError(text string) error { return plainError(text) }
