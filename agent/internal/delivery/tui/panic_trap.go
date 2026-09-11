package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// AppVersion versión actual del agente SOLV TUI.
const AppVersion = "1.0.0"

// RestoreTTY envía las secuencias de escape ANSI en orden estricto para restaurar el TTY del operador (F2).
func RestoreTTY() {
	// Secuencias explícitas en este orden:
	// 1. \x1b[?1000l\x1b[?1002l\x1b[?1006l (apagar tracking de mouse)
	// 2. \x1b[?25h                         (restaurar cursor visible)
	// 3. \x1b[0m                           (reset de atributos de texto)
	// 4. \x1b[?1049l                       (salir de alternate screen buffer)
	seq := "\x1b[?1000l\x1b[?1002l\x1b[?1006l\x1b[?25h\x1b[0m\x1b[?1049l"
	_, _ = os.Stdout.WriteString(seq)
	_ = os.Stdout.Sync()
}

// GetCrashLogPath retorna la ruta de crash.log en ~/.solv/crash.log.
func GetCrashLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = "."
	}
	solvDir := filepath.Join(homeDir, ".solv")
	_ = os.MkdirAll(solvDir, 0700)
	return filepath.Join(solvDir, "crash.log")
}

// AppendCrashLog registra timestamp, versión y stack trace en ~/.solv/crash.log (F2).
func AppendCrashLog(r interface{}, stack []byte) string {
	logPath := GetCrashLogPath()
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return logPath
	}
	defer f.Close()

	nowStr := time.Now().Format("2006-01-02T15:04:05-07:00")
	header := fmt.Sprintf("=== %s version=%s ===\n", nowStr, AppVersion)
	_, _ = f.WriteString(header)
	if r != nil {
		_, _ = f.WriteString(fmt.Sprintf("panic: %v\n", r))
	}
	if len(stack) > 0 {
		_, _ = f.Write(stack)
		_, _ = f.WriteString("\n")
	}
	return logPath
}

// HandlePanic atrapa un pánico fatal del run principal, restaura el TTY, escribe a crash.log
// y emite aviso de una línea en stderr terminando con código 1 sin re-paniquear (F2).
func HandlePanic(r interface{}) {
	RestoreTTY()
	stack := debug.Stack()
	logPath := AppendCrashLog(r, stack)
	fmt.Fprintf(os.Stderr, "SOLV TUI terminó inesperadamente. Registro de error guardado en %s\n", logPath)
	os.Exit(1)
}

// SafeGo ejecuta una goroutine protegida por un recover que registra errores en crash.log
// sin derribar el TTY del operador ni matar el proceso de la TUI.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				_ = AppendCrashLog(r, stack)
			}
		}()
		fn()
	}()
}
