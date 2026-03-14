package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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

	"smith/internal/source/model"
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
	args = normalizeCLIArgs(args)

	if wantsHelp(args) {
		printHelp(stdout)
		return 0
	}

	if len(args) > 0 && args[0] == "agent-chat" {
		return runAgentChat(args[1:], stdin, stdout, stderr)
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
		prdMode      bool
		promptFile   string
		outPath      string
		agentCmd     string
		stories      int
		fromMarkdown string
		fromJSON     string
		toMarkdown   string
	)
	fs.BoolVar(&prdMode, "prd", false, "Run PRD workflows")
	fs.StringVar(&promptFile, "prompt", "", "Prompt file to send directly to the agent")
	fs.StringVar(&outPath, "out", outDefault, "PRD output path (file or directory)")
	fs.StringVar(&agentCmd, "agent-cmd", agentDefault, "Agent command (supports {prompt} placeholder)")
	fs.IntVar(&stories, "stories", storyDefault, "Required story count when composing a PRD prompt")
	fs.StringVar(&fromMarkdown, "from-markdown", "", "Import a markdown PRD file into canonical JSON")
	fs.StringVar(&fromJSON, "from-json", "", "Read canonical PRD JSON from a file")
	fs.StringVar(&toMarkdown, "to-markdown", "", "Export canonical PRD JSON to markdown")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(stdout)
			return 0
		}
		fmt.Fprintln(stderr, err.Error())
		printHelp(stderr)
		return 2
	}

	if stories <= 0 {
		fmt.Fprintln(stderr, "--stories must be > 0")
		return 2
	}

	if err := validatePRDWorkflowFlags(promptFile, fromMarkdown, fromJSON, toMarkdown); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		printHelp(stderr)
		return 2
	}

	positional := fs.Args()
	if prdMode && len(positional) > 0 && strings.EqualFold(strings.TrimSpace(positional[0]), "validate") {
		return runValidateWorkflow(positional[1:], stdout, stderr)
	}
	if strings.TrimSpace(fromMarkdown) != "" {
		return runImportWorkflow(fromMarkdown, outPath, stdout, stderr)
	}
	if strings.TrimSpace(fromJSON) != "" || strings.TrimSpace(toMarkdown) != "" {
		return runExportWorkflow(fromJSON, toMarkdown, stdout, stderr)
	}

	prdRequest := strings.TrimSpace(strings.Join(positional, " "))
	if strings.TrimSpace(promptFile) == "" && prdRequest == "" {
		fmt.Fprintln(stderr, "either provide a PRD request, use --prompt, import markdown, export json, or validate a PRD")
		printHelp(stderr)
		return 2
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
		if strings.TrimSpace(outPath) == "" {
			outPath = defaultPRDOutputPath
		}
		absOutPath, outAsDirectory, err := prepareOutputPath(outPath)
		if err != nil {
			fmt.Fprintf(stderr, "prepare prd output failed: %v\n", err)
			return 1
		}
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

	if strings.TrimSpace(outPath) == "" {
		outPath = defaultPRDOutputPath
	}
	absOutPath, outAsDirectory, err := prepareOutputPath(outPath)
	if err != nil {
		fmt.Fprintf(stderr, "prepare prd output failed: %v\n", err)
		return 1
	}
	if resolved, ok := resolvePRDPath(absOutPath, outAsDirectory); ok {
		fmt.Fprintf(stdout, "PRD JSON saved to %s\n", resolved)
		return 0
	}
	fmt.Fprintf(stdout, "PRD generation completed; expected output under %s\n", absOutPath)
	return 0
}

func validatePRDWorkflowFlags(promptFile, fromMarkdown, fromJSON, toMarkdown string) error {
	if strings.TrimSpace(promptFile) != "" && strings.TrimSpace(fromMarkdown) != "" {
		return errors.New("--prompt cannot be combined with --from-markdown")
	}
	if strings.TrimSpace(promptFile) != "" && strings.TrimSpace(fromJSON) != "" {
		return errors.New("--prompt cannot be combined with --from-json")
	}
	if strings.TrimSpace(fromMarkdown) != "" && strings.TrimSpace(fromJSON) != "" {
		return errors.New("--from-markdown cannot be combined with --from-json")
	}
	if strings.TrimSpace(fromMarkdown) != "" && strings.TrimSpace(toMarkdown) != "" {
		return errors.New("--from-markdown cannot be combined with --to-markdown")
	}
	if strings.TrimSpace(toMarkdown) != "" && strings.TrimSpace(fromJSON) == "" {
		return errors.New("--to-markdown requires --from-json")
	}
	return nil
}

func runImportWorkflow(markdownPath, outPath string, stdout, stderr io.Writer) int {
	data, resolvedPath, err := readInputFile(markdownPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	prd, report := model.ValidatePRDMarkdown(data)
	if !report.Valid {
		return writeValidationReport(report, stdout, stderr)
	}

	if strings.TrimSpace(outPath) == "" {
		outPath = defaultPRDOutputPath
	}
	absOutPath, err := ensureFileOutputPath(outPath, ".json")
	if err != nil {
		fmt.Fprintf(stderr, "prepare prd output failed: %v\n", err)
		return 1
	}
	rendered, err := json.MarshalIndent(prd, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "marshal imported prd failed: %v\n", err)
		return 1
	}
	rendered = append(rendered, '\n')
	if err := os.WriteFile(absOutPath, rendered, 0o644); err != nil {
		fmt.Fprintf(stderr, "write prd json failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "PRD JSON saved to %s\n", absOutPath)
	fmt.Fprintf(stderr, "Imported markdown PRD from %s\n", resolvedPath)
	return 0
}

func runExportWorkflow(fromJSON, toMarkdown string, stdout, stderr io.Writer) int {
	if strings.TrimSpace(fromJSON) == "" {
		fmt.Fprintln(stderr, "--from-json is required for markdown export")
		return 2
	}
	if strings.TrimSpace(toMarkdown) == "" {
		fmt.Fprintln(stderr, "--to-markdown is required for markdown export")
		return 2
	}

	data, _, err := readInputFile(fromJSON)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	rendered, report := model.ExportPRDJSONToMarkdown(data)
	if !report.Valid {
		return writeValidationReport(report, stdout, stderr)
	}

	absOutPath, err := ensureFileOutputPath(toMarkdown, ".md")
	if err != nil {
		fmt.Fprintf(stderr, "prepare markdown output failed: %v\n", err)
		return 1
	}
	if err := os.WriteFile(absOutPath, []byte(rendered+"\n"), 0o644); err != nil {
		fmt.Fprintf(stderr, "write prd markdown failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "PRD markdown saved to %s\n", absOutPath)
	return 0
}

func runValidateWorkflow(args []string, stdout, stderr io.Writer) int {
	target := defaultPRDPathFromEnv()
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		target = args[0]
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "validate accepts at most one PRD path")
		return 2
	}

	data, resolvedPath, err := readInputFile(target)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	report, err := validateArtifact(data, resolvedPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	return writeValidationReport(report, stdout, stderr)
}

func ensureFileOutputPath(path string, extension string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(absPath), extension) {
		return "", fmt.Errorf("output path must end with %s: %s", extension, absPath)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", err
	}
	return absPath, nil
}

func readInputFile(path string) ([]byte, string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, "", fmt.Errorf("resolve input path failed: %w", err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("read input file failed: %w", err)
	}
	return data, absPath, nil
}

func validateArtifact(data []byte, path string) (model.PRDValidationReport, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		_, report := model.ValidatePRDJSON(data)
		return report, nil
	case ".md", ".markdown":
		_, report := model.ValidatePRDMarkdown(data)
		return report, nil
	default:
		return model.PRDValidationReport{}, fmt.Errorf("unsupported PRD format %q; use .json, .md, or .markdown", filepath.Ext(path))
	}
}

func writeValidationReport(report model.PRDValidationReport, stdout, stderr io.Writer) int {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "marshal validation report failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(payload))
	if report.Valid {
		return 0
	}
	return 1
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

func normalizeCLIArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	valueFlags := map[string]struct{}{
		"--prompt":        {},
		"--out":           {},
		"--agent-cmd":     {},
		"--stories":       {},
		"--from-markdown": {},
		"--from-json":     {},
		"--to-markdown":   {},
	}

	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if _, ok := valueFlags[arg]; ok {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if hasValueFlagPrefix(arg, valueFlags) || arg == "--prd" || arg == "-h" || arg == "--help" {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func hasValueFlagPrefix(arg string, flags map[string]struct{}) bool {
	for key := range flags {
		if strings.HasPrefix(arg, key+"=") {
			return true
		}
	}
	return false
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
	fmt.Fprintln(w, "  smith --prd --from-markdown <path> [--out .agents/tasks/prd.json]")
	fmt.Fprintln(w, "  smith --prd --from-json <path> --to-markdown <path>")
	fmt.Fprintln(w, "  smith --prd validate [path]")
	fmt.Fprintln(w, "  smith agent-chat [--agent-cmd \"...\"]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Notes:")
	fmt.Fprintln(w, "  --prd selects PRD workflows and composes a Ralph-style prompt for generation.")
	fmt.Fprintln(w, "  --prompt sends an existing prompt file directly to the agent command.")
	fmt.Fprintln(w, "  validate prints machine-readable JSON diagnostics and exits non-zero when the PRD is not ready.")
	fmt.Fprintln(w, "  agent-chat provides a structured JSON bridge for API integration.")
}

func runAgentChat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent-chat", flag.ContinueOnError)
	var agentCmd string
	fs.StringVar(&agentCmd, "agent-cmd", defaultPRDAgentFromEnv(), "Agent command")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	sendEvent := func(evt any) {
		data, _ := json.Marshal(evt)
		fmt.Fprintln(stdout, string(data))
	}

	isACP := strings.Contains(agentCmd, "acp")
	sendEvent(map[string]any{"type": "status", "text": "Agent bridge active", "acp": isACP})

	// Start the agent
	fields := strings.Fields(agentCmd)
	if len(fields) == 0 {
		sendEvent(map[string]string{"type": "error", "text": "empty agent command"})
		return 1
	}

	cmd := exec.Command(fields[0], fields[1:]...)
	agentStdin, _ := cmd.StdinPipe()
	agentStdout, _ := cmd.StdoutPipe()
	agentStderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		sendEvent(map[string]string{"type": "error", "text": "failed to start agent: " + err.Error()})
		return 1
	}

	// Helper to send JSON-RPC to agent
	sendRPC := func(method string, params any, id any) {
		req := map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
		}
		if id != nil {
			req["id"] = id
		}
		data, _ := json.Marshal(req)
		_, _ = agentStdin.Write(data)
		_, _ = agentStdin.Write([]byte("\n"))
	}

	// Handle ACP Handshake
	var sessionID string
	if isACP {
		sendRPC("initialize", map[string]any{
			"capabilities": map[string]any{"text": true},
			"client_info":  map[string]any{"name": "smith-console", "version": "1.0.0"},
		}, 1)
	}

	// Proxy stdin to agent (user prompts)
	go func() {
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			text := scanner.Text()
			if isACP && sessionID != "" {
				sendRPC("session/prompt", map[string]any{
					"session_id": sessionID,
					"message": map[string]any{
						"role": "user",
						"parts": []map[string]any{
							{"content_type": "text/plain", "content": text},
						},
					},
				}, time.Now().UnixNano())
			} else if !isACP {
				_, _ = io.WriteString(agentStdin, text+"\n")
			}
		}
		_ = agentStdin.Close()
	}()

	// Proxy stderr to logs
	go func() {
		s := bufio.NewScanner(agentStderr)
		for s.Scan() {
			sendEvent(map[string]string{"type": "log", "level": "stderr", "text": s.Text()})
		}
	}()

	// Proxy stdout to client
	var accumulated bytes.Buffer
	done := make(chan struct{})
	go func() {
		s := bufio.NewScanner(agentStdout)
		for s.Scan() {
			text := s.Text()
			accumulated.WriteString(text + "\n")

			var rpc map[string]any
			if json.Unmarshal([]byte(text), &rpc) == nil {
				// Handle ACP responses
				if isACP {
					method, _ := rpc["method"].(string)
					id, _ := rpc["id"]

					// Capability negotiation response
					if id == float64(1) {
						sendRPC("session/new", map[string]any{
							"working_directory": "/workspace",
						}, 2)
					} else if id == float64(2) {
						if res, ok := rpc["result"].(map[string]any); ok {
							sessionID, _ = res["session_id"].(string)
							sendEvent(map[string]string{"type": "status", "text": "Agent session established: " + sessionID})
						}
					}

					// Notifications
					if method == "session/update" {
						if params, ok := rpc["params"].(map[string]any); ok {
							if msg, ok := params["message"].(map[string]any); ok {
								if parts, ok := msg["parts"].([]any); ok && len(parts) > 0 {
									if part, ok := parts[0].(map[string]any); ok {
										content, _ := part["content"].(string)
										sendEvent(map[string]string{"type": "output", "text": content})
									}
								}
							}
						}
					}
				}
				sendEvent(map[string]any{"type": "rpc", "data": rpc})
			} else {
				sendEvent(map[string]string{"type": "output", "text": text})
			}
		}
		close(done)
	}()

	err := cmd.Wait()
	<-done

	if err != nil {
		sendEvent(map[string]string{"type": "status", "text": "Agent exited", "error": err.Error()})
	} else {
		sendEvent(map[string]string{"type": "status", "text": "Agent completed"})
	}

	// Final JSON-PRD extraction attempt
	outputStr := accumulated.String()
	var jsonStr string
	depth := 0
	start := -1
	for i, r := range outputStr {
		if r == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if r == '}' {
			depth--
			if depth == 0 && start != -1 {
				candidate := outputStr[start : i+1]
				var prd map[string]any
				if json.Unmarshal([]byte(candidate), &prd) == nil {
					if _, hasProj := prd["project"]; hasProj {
						jsonStr = candidate
					}
				}
			}
		}
	}

	if jsonStr != "" {
		sendEvent(map[string]any{"type": "final_prd", "content": json.RawMessage(jsonStr)})
	}

	return 0
}
