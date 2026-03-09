package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPRDAgentCommand = "codex --yolo --skip-git-repo-check {prompt}"
	defaultPRDOutputPath   = ".agents/tasks/prd.json"
	defaultPRDStoryCount   = 5
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		printHelp(stdout)
		return 0
	}

	hasPRDMode := hasFlag(args, "--prd") || hasFlag(args, "--prompt")
	if !hasPRDMode {
		fmt.Fprintln(stderr, "smith currently supports PRD mode only. Use --prd or --prompt.")
		printHelp(stderr)
		return 2
	}

	outDefault := defaultPRDPathFromEnv()
	agentDefault := defaultPRDAgentFromEnv()
	storyDefault := defaultPRDStoriesFromEnv()

	fs := flag.NewFlagSet("smith", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		prdRequest string
		promptFile string
		outPath    string
		agentCmd   string
		stories    int
	)
	fs.StringVar(&prdRequest, "prd", "", "Feature request text for PRD generation")
	fs.StringVar(&promptFile, "prompt", "", "Prompt file to send directly to the agent")
	fs.StringVar(&outPath, "out", outDefault, "PRD output path (file or directory)")
	fs.StringVar(&agentCmd, "agent-cmd", agentDefault, "Agent command (supports {prompt} placeholder)")
	fs.IntVar(&stories, "stories", storyDefault, "Required story count when composing a PRD prompt")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(stdout)
			return 0
		}
		fmt.Fprintln(stderr, err.Error())
		printHelp(stderr)
		return 2
	}

	if strings.TrimSpace(prdRequest) == "" && strings.TrimSpace(promptFile) == "" && len(fs.Args()) > 0 {
		prdRequest = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(promptFile) == "" && strings.TrimSpace(prdRequest) == "" {
		fmt.Fprintln(stderr, "either --prd \"<request>\" or --prompt <file> is required")
		printHelp(stderr)
		return 2
	}
	if stories <= 0 {
		fmt.Fprintln(stderr, "--stories must be > 0")
		return 2
	}

	if strings.TrimSpace(outPath) == "" {
		outPath = defaultPRDOutputPath
	}
	absOutPath, outAsDirectory, err := prepareOutputPath(outPath)
	if err != nil {
		fmt.Fprintf(stderr, "prepare prd output failed: %v\n", err)
		return 1
	}

	promptPath := strings.TrimSpace(promptFile)
	if promptPath != "" {
		absPrompt, err := filepath.Abs(promptPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve prompt file failed: %v\n", err)
			return 1
		}
		if _, err := os.Stat(absPrompt); err != nil {
			fmt.Fprintf(stderr, "prompt file failed: %v\n", err)
			return 1
		}
		promptPath = absPrompt
	} else {
		prompt := buildPRDPrompt(strings.TrimSpace(prdRequest), absOutPath, stories, outAsDirectory)
		promptPath, err = writePromptFile(prompt)
		if err != nil {
			fmt.Fprintf(stderr, "write prompt failed: %v\n", err)
			return 1
		}
	}

	if err := ensureAgentCommand(strings.TrimSpace(agentCmd)); err != nil {
		fmt.Fprintf(stderr, "invalid --agent-cmd: %v\n", err)
		return 2
	}

	rendered := renderAgentCommand(strings.TrimSpace(agentCmd), promptPath)
	cmd := exec.Command("sh", "-lc", rendered)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "agent command failed: %v\n", err)
		return 1
	}

	if resolved, ok := resolvePRDPath(absOutPath, outAsDirectory); ok {
		fmt.Fprintf(stdout, "PRD JSON saved to %s\n", resolved)
		return 0
	}
	fmt.Fprintf(stdout, "PRD generation completed; expected output under %s\n", absOutPath)
	return 0
}

func defaultPRDPathFromEnv() string {
	if candidate := strings.TrimSpace(os.Getenv("SMITH_PRD_PATH")); candidate != "" {
		return candidate
	}
	if candidate := strings.TrimSpace(os.Getenv("SMITH_LOOP_PRD_PATH")); candidate != "" {
		return candidate
	}
	return defaultPRDOutputPath
}

func defaultPRDAgentFromEnv() string {
	if candidate := strings.TrimSpace(os.Getenv("SMITH_PRD_AGENT_CMD")); candidate != "" {
		return candidate
	}
	if candidate := strings.TrimSpace(os.Getenv("SMITH_CODEX_CLI_CMD")); candidate != "" {
		return candidate
	}
	return defaultPRDAgentCommand
}

func defaultPRDStoriesFromEnv() int {
	for _, key := range []string{"SMITH_PRD_STORY_COUNT", "SMITH_LOOP_PRD_STORY_COUNT"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultPRDStoryCount
}

func prepareOutputPath(path string) (string, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}

	info, statErr := os.Stat(absPath)
	if statErr == nil {
		if info.IsDir() {
			return absPath, true, nil
		}
		if strings.EqualFold(filepath.Ext(absPath), ".json") {
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return "", false, err
			}
			return absPath, false, nil
		}
		return "", false, fmt.Errorf("output path exists and is not a json file or directory: %s", absPath)
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return "", false, statErr
	}
	if strings.EqualFold(filepath.Ext(absPath), ".json") {
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", false, err
		}
		return absPath, false, nil
	}
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return "", false, err
	}
	return absPath, true, nil
}

func resolvePRDPath(path string, isDirectory bool) (string, bool) {
	if !isDirectory {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
		return "", false
	}
	matches, err := filepath.Glob(filepath.Join(path, "*.json"))
	if err != nil {
		return "", false
	}
	files := make([]string, 0, len(matches))
	for _, candidate := range matches {
		info, statErr := os.Stat(candidate)
		if statErr != nil || info.IsDir() {
			continue
		}
		files = append(files, candidate)
	}
	if len(files) == 0 {
		return "", false
	}
	sort.Strings(files)
	return files[0], true
}

func writePromptFile(prompt string) (string, error) {
	promptsDir := filepath.Join(".smith", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("prd-%d.md", time.Now().UTC().UnixNano())
	promptPath := filepath.Join(promptsDir, fileName)
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return "", err
	}
	return filepath.Abs(promptPath)
}

func buildPRDPrompt(request, outPath string, stories int, outIsDirectory bool) string {
	lines := []string{
		"You are an autonomous coding agent.",
		"Use the $prd skill to create a Product Requirements Document in JSON.",
	}
	if outIsDirectory {
		lines = append(lines,
			"Save the PRD as JSON in directory: "+outPath,
			"Filename rules: prd-<short-slug>.json using 1-3 meaningful words.",
			"Examples: prd-workflow-engine.json, prd-runtime-pods.json",
		)
	} else {
		lines = append(lines, "Save the PRD to: "+outPath)
	}
	lines = append(lines,
		"Do NOT implement anything.",
		fmt.Sprintf("Include exactly %d user stories in the stories array.", stories),
		"After creating the PRD, end with:",
		"PRD JSON saved to <path>. Close this chat and launch the Smith build loop.",
		"",
		"User request:",
		strings.TrimSpace(request),
	)
	return strings.Join(lines, "\n")
}

func ensureAgentCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("command is empty")
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return errors.New("command is invalid")
	}
	if _, err := exec.LookPath(fields[0]); err != nil {
		return fmt.Errorf("%q not found in PATH", fields[0])
	}
	return nil
}

func renderAgentCommand(command, promptPath string) string {
	if strings.Contains(command, "{prompt}") {
		return strings.ReplaceAll(command, "{prompt}", shellQuote(promptPath))
	}
	return fmt.Sprintf("cat %s | %s", shellQuote(promptPath), command)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func hasFlag(args []string, key string) bool {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == key {
			return true
		}
		if strings.HasPrefix(arg, key+"=") {
			return true
		}
	}
	return false
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "-h", "--help", "help":
			return true
		}
	}
	return false
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "smith - local PRD launcher")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  smith --prd \"<feature request>\" [--out path] [--stories N] [--agent-cmd \"...\"]")
	fmt.Fprintln(w, "  smith --prompt <prompt-file> [--out path] [--agent-cmd \"...\"]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Notes:")
	fmt.Fprintln(w, "  --prd composes a Ralph-style prompt that invokes the $prd skill.")
	fmt.Fprintln(w, "  --prompt sends an existing prompt file directly to the agent command.")
	fmt.Fprintln(w, "  Default --out is .agents/tasks/prd.json.")
}
