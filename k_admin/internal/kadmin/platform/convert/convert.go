// Package convert renders office documents to PDF by driving a local
// LibreOffice (soffice) binary, replacing the legacy JODConverter pipeline.
// Jobs are bounded by a semaphore (LibreOffice is heavy per process) and a
// per-conversion timeout; every job runs with its own user-profile directory
// so concurrent soffice instances do not fight over the default profile lock.
// When soffice cannot be started the error wraps ErrUnavailable so callers
// can report "转档服务不可用" per the original upload logic.
package convert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrUnavailable means soffice could not be started at all (binary
	// missing or not executable) — the service as a whole is unavailable.
	ErrUnavailable = errors.New("libreoffice is unavailable")
	// ErrConversionFailed means soffice ran but did not produce a PDF
	// (source malformed, crashed on the document, or timed out).
	ErrConversionFailed = errors.New("libreoffice conversion failed")
	// ErrInvalidSource means the input cannot be converted (missing or
	// PDF-alias extension).
	ErrInvalidSource = errors.New("invalid document for conversion")
)

const (
	defaultBin         = "soffice"
	defaultTimeout     = 2 * time.Minute
	defaultConcurrency = 2
)

type Config struct {
	// Bin is the soffice executable (default "soffice" from PATH).
	Bin string
	// Timeout bounds a single conversion (default 2m).
	Timeout time.Duration
	// MaxConcurrent bounds parallel soffice processes (default 2).
	MaxConcurrent int
}

// ConfigFromEnv reads KADMIN_SOFFICE_BIN / KADMIN_CONVERT_TIMEOUT /
// KADMIN_CONVERT_CONCURRENCY with production-safe defaults.
func ConfigFromEnv(getenv func(key, fallback string) string) Config {
	config := Config{
		Bin:     getenv("KADMIN_SOFFICE_BIN", defaultBin),
		Timeout: parseDuration(getenv("KADMIN_CONVERT_TIMEOUT", defaultTimeout.String())),
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	config.MaxConcurrent = parsePositiveInt(getenv("KADMIN_CONVERT_CONCURRENCY", "2"), defaultConcurrency)
	return config
}

type Runner interface {
	// CombinedOutput runs the command and returns its combined output.
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Converter struct {
	config Config
	runner Runner
	slots  chan struct{}
}

func New(config Config) *Converter {
	return NewWithRunner(config, execRunner{})
}

func NewWithRunner(config Config, runner Runner) *Converter {
	if config.Bin == "" {
		config.Bin = defaultBin
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = defaultConcurrency
	}
	return &Converter{
		config: config,
		runner: runner,
		slots:  make(chan struct{}, config.MaxConcurrent),
	}
}

// Probe reports whether soffice can start, for startup health logging.
func (c *Converter) Probe(ctx context.Context) error {
	if _, err := c.runner.CombinedOutput(ctx, c.config.Bin, "--version"); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// ToPDF renders the given document to PDF bytes. sourceName only contributes
// its extension (which selects the LibreOffice import filter); the payload is
// re-materialized under a fixed name inside a private temp directory.
func (c *Converter) ToPDF(ctx context.Context, sourceName string, source io.Reader) ([]byte, error) {
	extension := strings.ToLower(filepath.Ext(sourceName))
	if extension == "" || extension == ".pdf" {
		return nil, fmt.Errorf("%w: need an office document extension, got %q", ErrInvalidSource, extension)
	}
	workDir, err := os.MkdirTemp("", "kadmin-convert-*")
	if err != nil {
		return nil, fmt.Errorf("create convert workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	inputPath := filepath.Join(workDir, "source"+extension)
	payload, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("read conversion source: %w", err)
	}
	if err := os.WriteFile(inputPath, payload, 0o600); err != nil {
		return nil, fmt.Errorf("stage conversion source: %w", err)
	}

	select {
	case c.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("conversion cancelled while queued: %w", ctx.Err())
	}
	defer func() { <-c.slots }()

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	profileURI := "file:///" + filepath.ToSlash(filepath.Join(workDir, "profile"))
	output, err := c.runner.CombinedOutput(ctx, c.config.Bin,
		"--headless", "--norestore", "--nolockcheck",
		"-env:UserInstallation="+profileURI,
		"--convert-to", "pdf",
		"--outdir", workDir,
		inputPath,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%w: timed out after %s", ErrConversionFailed, c.config.Timeout)
		}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return nil, fmt.Errorf("%w: %v: %s", ErrConversionFailed, err, tail(output))
	}

	pdfPath := filepath.Join(workDir, "source.pdf")
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("%w: no PDF produced: %s", ErrConversionFailed, tail(output))
	}
	return pdf, nil
}

func tail(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 400 {
		text = text[len(text)-400:]
	}
	return text
}

type execRunner struct{}

func (execRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func parseDuration(value string) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func parsePositiveInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	number := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		number = number*10 + int(r-'0')
		if number > 64 {
			return fallback
		}
	}
	if number <= 0 {
		return fallback
	}
	return number
}
