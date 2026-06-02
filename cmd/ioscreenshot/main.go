// Command ioscreenshot regenerates the checked-in io-example.png asset.
//
// It launches the deterministic iodemo screenshot mode in an isolated tmux
// session, opens a fresh Ghostty window attached to that session, captures that
// window with macOS screencapture, and atomically replaces the target PNG.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type config struct {
	output        string
	root          string
	ghosttyApp    string
	clock         string
	width         int
	height        int
	verify        bool
	keepArtifacts bool
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var cfg config
	flag.StringVar(&cfg.ghosttyApp, "ghostty-app", envOr("IO_GHOSTTY_APP", envOr("TOAST_GHOSTTY_APP", "/Applications/Ghostty.app")), "Ghostty .app path")
	flag.StringVar(&cfg.clock, "clock", envOr("IO_EXAMPLE_CLOCK", "14:11"), "fixed HH:MM clock shown in the screenshot")
	flag.IntVar(&cfg.width, "width", 100, "terminal width in columns")
	flag.IntVar(&cfg.height, "height", 40, "terminal height in rows")
	flag.BoolVar(&cfg.verify, "verify", false, "fail when the regenerated PNG differs from the committed file")
	flag.BoolVar(&cfg.keepArtifacts, "keep-artifacts", false, "keep temporary binaries and raw screenshots")
	flag.Parse()

	cfg.output = "io-example.png"
	if flag.NArg() > 0 {
		cfg.output = flag.Arg(0)
	}

	root, err := gitRoot(ctx)
	if err != nil {
		fatal(err)
	}
	cfg.root = root
	if !filepath.IsAbs(cfg.output) {
		cfg.output = filepath.Join(root, cfg.output)
	}

	if err := generate(ctx, cfg); err != nil {
		fatal(err)
	}
	if cfg.verify {
		if err := verifyCommitted(ctx, cfg.root, cfg.output); err != nil {
			fatal(err)
		}
	}
}

func generate(ctx context.Context, cfg config) error {
	if runtime.GOOS != "darwin" {
		return errors.New("io-example.png generation requires macOS")
	}
	if cfg.width < 20 || cfg.height < 10 {
		return fmt.Errorf("terminal size %dx%d is too small", cfg.width, cfg.height)
	}
	if info, err := os.Stat(cfg.ghosttyApp); err != nil || !info.IsDir() {
		if err != nil {
			return fmt.Errorf("Ghostty app %q: %w", cfg.ghosttyApp, err)
		}
		return fmt.Errorf("Ghostty app %q is not a directory", cfg.ghosttyApp)
	}

	tools, err := requireTools("clang", "env", "go", "open", "screencapture", "tmux")
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "io-example-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if cfg.keepArtifacts {
		fmt.Fprintf(os.Stderr, "ioscreenshot: keeping artifacts in %s\n", tmp)
	} else {
		defer os.RemoveAll(tmp)
	}

	demoPath := filepath.Join(tmp, "iodemo")
	if err := run(ctx, cfg.root, tools["go"], "build", "-o", demoPath, "./cmd/iodemo"); err != nil {
		return err
	}
	finderPath, err := buildWindowFinder(ctx, tools["clang"], tmp)
	if err != nil {
		return err
	}

	socketName := "ioexample-" + strconv.Itoa(os.Getpid())
	sessionName := "ioexample"
	targetPane := sessionName + ":0.0"
	title := "io"
	terminalW := strconv.Itoa(cfg.width)
	terminalH := strconv.Itoa(cfg.height)
	shellCommand := strings.Join([]string{
		"exec",
		shellQuote(tools["env"]),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"IO_TUI_CLOCK=" + shellQuote(cfg.clock),
		shellQuote(demoPath),
		"--screenshot",
		"--width", shellQuote(terminalW),
		"--height", shellQuote(terminalH),
		"--window-title", shellQuote("io"),
	}, " ")

	if err := run(ctx, cfg.root, tools["tmux"], "-L", socketName, "new-session", "-d", "-s", sessionName, "-x", terminalW, "-y", terminalH, "-c", cfg.root, "/bin/sh"); err != nil {
		return err
	}
	defer func() {
		_ = exec.Command(tools["tmux"], "-L", socketName, "kill-server").Run()
	}()
	if err := run(ctx, cfg.root, tools["tmux"], "-L", socketName, "set-option", "-t", sessionName, "status", "off"); err != nil {
		return err
	}
	enableTmuxTruecolor(tools["tmux"], socketName)

	initialCommand := strings.Join([]string{
		shellQuote(tools["tmux"]),
		"-L", shellQuote(socketName),
		"attach-session",
		"-t", shellQuote(sessionName),
	}, " ")
	if err := run(ctx, cfg.root, tools["open"],
		"-na", cfg.ghosttyApp,
		"--args",
		"--title="+title,
		"--working-directory="+cfg.root,
		"--initial-command="+initialCommand,
		"--window-width="+terminalW,
		"--window-height="+terminalH,
		"--window-save-state=never",
		"--window-inherit-working-directory=false",
		"--tab-inherit-working-directory=false",
		"--window-inherit-font-size=false",
		"--confirm-close-surface=false",
		"--quit-after-last-window-closed=true",
		"--shell-integration=none",
		"--macos-applescript=true",
	); err != nil {
		return err
	}

	windowID, err := waitForWindow(ctx, finderPath, title)
	if err != nil {
		return err
	}
	if err := run(ctx, cfg.root, tools["tmux"], "-L", socketName, "send-keys", "-t", targetPane, shellCommand, "Enter"); err != nil {
		return err
	}
	if err := waitForPane(ctx, tools["tmux"], socketName, targetPane); err != nil {
		return err
	}
	time.Sleep(350 * time.Millisecond)

	rawPath := filepath.Join(tmp, "io-example.raw.png")
	if err := run(ctx, cfg.root, tools["screencapture"], "-x", "-o", "-t", "png", "-l", windowID, rawPath); err != nil {
		return err
	}
	if err := validatePNG(rawPath); err != nil {
		return err
	}
	if err := replaceFile(rawPath, cfg.output); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "ioscreenshot: wrote %s\n", cfg.output)
	return nil
}

func enableTmuxTruecolor(tmuxPath, socketName string) {
	_ = exec.Command(tmuxPath, "-L", socketName, "set-option", "-gq", "terminal-overrides", ",*:Tc").Run()
	_ = exec.Command(tmuxPath, "-L", socketName, "set-option", "-gq", "terminal-features", ",*:RGB").Run()
}

func requireTools(names ...string) (map[string]string, error) {
	tools := make(map[string]string, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			return nil, fmt.Errorf("required tool %q not found in PATH", name)
		}
		tools[name] = path
	}
	return tools, nil
}

func buildWindowFinder(ctx context.Context, clangPath, dir string) (string, error) {
	sourcePath := filepath.Join(dir, "ghostty-window-id.c")
	binaryPath := filepath.Join(dir, "ghostty-window-id")
	if err := os.WriteFile(sourcePath, []byte(windowFinderSource), 0o644); err != nil {
		return "", fmt.Errorf("write window finder source: %w", err)
	}
	if err := run(ctx, dir, clangPath, "-framework", "ApplicationServices", "-framework", "CoreFoundation", "-o", binaryPath, sourcePath); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func waitForWindow(ctx context.Context, finderPath, title string) (string, error) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		out, err := output(ctx, "", finderPath, title)
		if err == nil {
			if id := strings.TrimSpace(string(out)); id != "" {
				return id, nil
			}
		}
		if err := sleep(ctx, 200*time.Millisecond); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("timed out waiting for Ghostty window titled %q", title)
}

func waitForPane(ctx context.Context, tmuxPath, socketName, targetPane string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		out, err := output(ctx, "", tmuxPath, "-L", socketName, "capture-pane", "-p", "-t", targetPane)
		if err == nil {
			pane := string(out)
			if strings.Contains(pane, "IO-LINK") && strings.Contains(pane, "Message io") {
				return nil
			}
		}
		if err := sleep(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
	return errors.New("timed out waiting for iodemo screenshot content")
}

func validatePNG(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open captured PNG: %w", err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("decode captured PNG: %w", err)
	}
	if cfg.Width < 800 || cfg.Height < 600 {
		return fmt.Errorf("captured PNG is unexpectedly small: %dx%d", cfg.Width, cfg.Height)
	}
	return nil
}

func replaceFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	tmp := filepath.Join(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-"+strconv.Itoa(os.Getpid()))
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source PNG: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open temp output PNG: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy PNG: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close temp output PNG: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("replace output PNG: %w", err)
	}
	return nil
}

func verifyCommitted(ctx context.Context, root, outputPath string) error {
	rel, err := filepath.Rel(root, outputPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("cannot verify %s because it is outside git root %s", outputPath, root)
	}
	workingDirty, err := gitDiff(ctx, root, false, rel)
	if err != nil {
		return err
	}
	indexDirty, err := gitDiff(ctx, root, true, rel)
	if err != nil {
		return err
	}
	if workingDirty || indexDirty {
		return fmt.Errorf("generated %s differs from the committed version; commit the regenerated image, then push again", rel)
	}
	return nil
}

func gitDiff(ctx context.Context, root string, cached bool, rel string) (bool, error) {
	args := []string{"diff", "--quiet"}
	if cached {
		args = append(args, "--cached")
	}
	args = append(args, "--", rel)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func gitRoot(ctx context.Context) (string, error) {
	out, err := output(ctx, "", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", errors.New("git rev-parse returned an empty root")
	}
	return root, nil
}

func run(ctx context.Context, dir, name string, args ...string) error {
	out, err := output(ctx, dir, name, args...)
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

func output(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ioscreenshot:", err)
	os.Exit(1)
}

const windowFinderSource = `
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>
#include <stdio.h>

static int cfstring_contains_cstr(CFStringRef value, const char *needle) {
    if (value == NULL || needle == NULL) {
        return 0;
    }
    CFStringRef needleValue = CFStringCreateWithCString(NULL, needle, kCFStringEncodingUTF8);
    if (needleValue == NULL) {
        return 0;
    }
    CFRange found = CFStringFind(value, needleValue, kCFCompareCaseInsensitive);
    CFRelease(needleValue);
    return found.location != kCFNotFound;
}

static int cfstring_equals_cstr(CFStringRef value, const char *needle) {
    if (value == NULL || needle == NULL) {
        return 0;
    }
    CFStringRef needleValue = CFStringCreateWithCString(NULL, needle, kCFStringEncodingUTF8);
    if (needleValue == NULL) {
        return 0;
    }
    CFComparisonResult result = CFStringCompare(value, needleValue, kCFCompareCaseInsensitive);
    CFRelease(needleValue);
    return result == kCFCompareEqualTo;
}

int main(int argc, char **argv) {
    if (argc != 2) {
        return 2;
    }

    CFArrayRef windows = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID
    );
    if (windows == NULL) {
        return 1;
    }

    CFIndex count = CFArrayGetCount(windows);
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);
        CFStringRef owner = (CFStringRef)CFDictionaryGetValue(window, kCGWindowOwnerName);
        CFStringRef name = (CFStringRef)CFDictionaryGetValue(window, kCGWindowName);
        CFNumberRef number = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowNumber);

        if (number == NULL || !cfstring_contains_cstr(owner, "ghostty") || !cfstring_equals_cstr(name, argv[1])) {
            continue;
        }

        uint32_t windowID = 0;
        if (CFNumberGetValue(number, kCFNumberSInt32Type, &windowID)) {
            printf("%u\n", windowID);
            CFRelease(windows);
            return 0;
        }
    }

    CFRelease(windows);
    return 0;
}
`
