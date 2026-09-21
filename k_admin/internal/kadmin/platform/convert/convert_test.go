package convert

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSoffice mimics soffice: it honours --outdir and --convert-to pdf by
// materializing <inputbase>.pdf next to the input.
type fakeSoffice struct {
	mu       sync.Mutex
	failWith error
	output   string
	block    time.Duration
	active   int32
	maxSlots int32
	pdfMagic string
}

func (f *fakeSoffice) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	current := atomic.AddInt32(&f.active, 1)
	defer atomic.AddInt32(&f.active, -1)
	for {
		maxSlots := atomic.LoadInt32(&f.maxSlots)
		if current <= maxSlots || atomic.CompareAndSwapInt32(&f.maxSlots, maxSlots, current) {
			break
		}
	}
	if f.block > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(f.block):
		}
	}
	if f.failWith != nil {
		return []byte(f.output), f.failWith
	}
	outdir := ""
	input := ""
	for i, arg := range args {
		if arg == "--outdir" && i+1 < len(args) {
			outdir = args[i+1]
		}
	}
	for i := len(args) - 1; i >= 0; i-- {
		if !strings.HasPrefix(args[i], "-") {
			input = args[i]
			break
		}
	}
	if outdir == "" || input == "" {
		return []byte("fake soffice: missing arguments"), errors.New("fake soffice invoked incorrectly")
	}
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	magic := f.pdfMagic
	if magic == "" {
		magic = "%PDF-fake"
	}
	if err := os.WriteFile(filepath.Join(outdir, base+".pdf"), []byte(magic), 0o600); err != nil {
		return nil, err
	}
	return []byte("convert"), nil
}

func testConverter(t *testing.T, runner Runner, mutate func(*Config)) *Converter {
	t.Helper()
	config := Config{Bin: "soffice", Timeout: 2 * time.Second, MaxConcurrent: 2}
	if mutate != nil {
		mutate(&config)
	}
	return NewWithRunner(config, runner)
}

func TestToPDFReturnsProducedPDF(t *testing.T) {
	runner := &fakeSoffice{pdfMagic: "%PDF-1.7 custom"}
	converter := testConverter(t, runner, nil)
	pdf, err := converter.ToPDF(context.Background(), "试卷 final.docx", strings.NewReader("doc-payload"))
	if err != nil {
		t.Fatalf("ToPDF: %v", err)
	}
	if string(pdf) != "%PDF-1.7 custom" {
		t.Fatalf("unexpected pdf payload %q", pdf)
	}
}

func TestToPDFRejectsInvalidSources(t *testing.T) {
	converter := testConverter(t, &fakeSoffice{}, nil)
	for _, name := range []string{"noext", "archive.pdf"} {
		if _, err := converter.ToPDF(context.Background(), name, strings.NewReader("x")); !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("ToPDF(%q) error = %v, want ErrInvalidSource", name, err)
		}
	}
}

func TestToPDFMapsStartFailureToUnavailable(t *testing.T) {
	runner := &fakeSoffice{failWith: &exec.Error{Name: "soffice", Err: exec.ErrNotFound}}
	converter := testConverter(t, runner, nil)
	_, err := converter.ToPDF(context.Background(), "a.docx", strings.NewReader("x"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if probeErr := converter.Probe(context.Background()); !errors.Is(probeErr, ErrUnavailable) {
		t.Fatalf("Probe error = %v, want ErrUnavailable", probeErr)
	}
}

func TestToPDFMapsConversionFailure(t *testing.T) {
	runner := &fakeSoffice{failWith: errors.New("exit status 1"), output: "source corrupt"}
	converter := testConverter(t, runner, nil)
	_, err := converter.ToPDF(context.Background(), "a.pptx", strings.NewReader("x"))
	if !errors.Is(err, ErrConversionFailed) {
		t.Fatalf("error = %v, want ErrConversionFailed", err)
	}
	if !strings.Contains(err.Error(), "source corrupt") {
		t.Fatalf("error should carry soffice output, got %v", err)
	}
}

func TestToPDFEnforcesTimeout(t *testing.T) {
	runner := &fakeSoffice{block: time.Second}
	converter := testConverter(t, runner, func(config *Config) { config.Timeout = 30 * time.Millisecond })
	started := time.Now()
	_, err := converter.ToPDF(context.Background(), "a.docx", strings.NewReader("x"))
	if !errors.Is(err, ErrConversionFailed) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want conversion timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("timeout not enforced, elapsed %s", elapsed)
	}
}

func TestToPDFHonorsConcurrencyLimit(t *testing.T) {
	runner := &fakeSoffice{block: 20 * time.Millisecond}
	converter := testConverter(t, runner, func(config *Config) { config.MaxConcurrent = 1 })
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := converter.ToPDF(context.Background(), "a.docx", strings.NewReader("x")); err != nil {
				t.Errorf("ToPDF: %v", err)
			}
		}()
	}
	wg.Wait()
	if max := atomic.LoadInt32(&runner.maxSlots); max != 1 {
		t.Fatalf("max concurrent soffice runs = %d, want 1", max)
	}
}

func TestConfigFromEnv(t *testing.T) {
	config := ConfigFromEnv(func(key, fallback string) string {
		switch key {
		case "KADMIN_SOFFICE_BIN":
			return "/usr/lib/libreoffice/program/soffice"
		case "KADMIN_CONVERT_TIMEOUT":
			return "90s"
		case "KADMIN_CONVERT_CONCURRENCY":
			return "3"
		}
		return fallback
	})
	if config.Bin == "" || config.Timeout != 90*time.Second || config.MaxConcurrent != 3 {
		t.Fatalf("unexpected config: %#v", config)
	}
	fallback := ConfigFromEnv(func(key, val string) string { return val })
	if fallback.Bin != defaultBin || fallback.Timeout != defaultTimeout || fallback.MaxConcurrent != defaultConcurrency {
		t.Fatalf("unexpected defaults: %#v", fallback)
	}
}
