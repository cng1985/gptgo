package skill

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	apperr "gptgo/internal/platform/errors"
)

const (
	maxScriptBytes = 32 * 1024
	maxInputBytes  = 8000
	maxOutputBytes = 8 * 1024
	defaultTimeout = 10
	maxTimeout     = 20
)

type execSpec struct {
	Name    string
	Runtime string
	Script  string
	Timeout int
	Input   string
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if remain := b.max - b.buf.Len(); remain > 0 {
		chunk := p
		if len(chunk) > remain {
			chunk = chunk[:remain]
			b.cut = true
		}
		_, _ = b.buf.Write(chunk)
	} else if len(p) > 0 {
		b.cut = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	text := b.buf.String()
	if b.cut {
		text += "\n…已截断"
	}
	return text
}

func execute(ctx context.Context, spec execSpec) (SkillRunResult, error) {
	var result SkillRunResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
	spec.Runtime = strings.TrimSpace(spec.Runtime)
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		spec.Name = "draft"
	}
	if spec.Timeout <= 0 {
		spec.Timeout = defaultTimeout
	}
	if spec.Timeout > maxTimeout {
		spec.Timeout = maxTimeout
	}
	bin, ext, err := resolveRuntime(spec.Runtime)
	if err != nil {
		result.ExitCode = -1
		result.Stderr = err.Error()
		return result, nil
	}
	script := spec.Script
	if strings.TrimSpace(script) == "" {
		return result, apperr.Invalid("脚本不能为空")
	}
	if strings.ContainsRune(script, 0) || len(script) > maxScriptBytes {
		return result, apperr.Invalid("脚本不能超过 32 KB，且不能包含空字符")
	}
	input := spec.Input
	if strings.TrimSpace(input) == "" {
		input = "{}"
	}
	dir, err := os.MkdirTemp("", "chatbi-skill-")
	if err != nil {
		return result, apperr.Internal(err)
	}
	defer os.RemoveAll(dir)
	scriptPath := filepath.Join(dir, "main"+ext)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return result, apperr.Internal(err)
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(spec.Timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, bin, scriptPath)
	cmd.Dir = dir
	cmd.Env = cleanEnv(dir, spec.Name)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr limitedBuffer
	stdout.max = maxOutputBytes
	stderr.max = maxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	prepareCommand(cmd)
	started := time.Now()
	runErr := cmd.Run()
	result.DurationMS = time.Since(started).Milliseconds()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if runCtx.Err() != nil {
		result.OK = false
		result.ExitCode = -1
		note := fmt.Sprintf("脚本超过 %d 秒，已停止", spec.Timeout)
		if strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = note
		} else {
			result.Stderr = strings.TrimSpace(result.Stderr) + "\n" + note
		}
		return result, nil
	}
	if runErr == nil {
		result.OK = true
		result.ExitCode = 0
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		if strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = "脚本没有成功结束"
		}
		return result, nil
	}
	result.ExitCode = -1
	if strings.TrimSpace(result.Stderr) == "" {
		result.Stderr = "无法启动脚本"
	}
	return result, nil
}

func resolveRuntime(runtime string) (string, string, error) {
	switch runtime {
	case "python":
		bin, err := lookAny("python3", "python")
		return bin, ".py", err
	case "node":
		bin, err := lookAny("node")
		return bin, ".js", err
	case "bash":
		bin, err := lookAny("bash")
		return bin, ".sh", err
	default:
		return "", "", fmt.Errorf("只支持 python、node、bash")
	}
}

func lookAny(names ...string) (string, error) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("本机没有 %s，无法执行这个技能", strings.Join(names, " 或 "))
}

func cleanEnv(dir, name string) []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/bin:/usr/bin:/bin"
	}
	env := []string{
		"PATH=" + path,
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + dir,
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"SKILL_NAME=" + name,
		"PYTHONDONTWRITEBYTECODE=1",
	}
	if tz := os.Getenv("TZ"); tz != "" {
		env = append(env, "TZ="+tz)
	}
	return env
}

func clipRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "\n…已截断"
}
