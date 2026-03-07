package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultServer = "http://127.0.0.1:8080"

type rootFlags struct {
	Server  string
	Token   string
	Config  string
	Context string
	Output  string
}

type runtimeConfig struct {
	Server string
	Token  string
	Output string
}

type fileConfig struct {
	CurrentContext string                   `json:"current_context"`
	Contexts       map[string]contextConfig `json:"contexts"`
}

type contextConfig struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) int {
	flags, rest, err := parseRootFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	cfg, err := resolveConfig(flags)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if len(rest) == 0 {
		printHelp(stdout)
		return 0
	}
	client := &apiClient{
		baseURL: strings.TrimRight(cfg.Server, "/"),
		token:   cfg.Token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	switch rest[0] {
	case "loop":
		return runLoop(client, cfg.Output, rest[1:], stdout, stderr)
	case "prd":
		return runPRD(client, cfg.Output, rest[1:], stdout, stderr)
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown resource %q\n", rest[0])
		printHelp(stderr)
		return 2
	}
}

func parseRootFlags(args []string) (rootFlags, []string, error) {
	home, _ := os.UserHomeDir()
	defaults := rootFlags{
		Config: filepath.Join(home, ".smith", "config.json"),
		Output: "text",
	}
	fs := flag.NewFlagSet("smithctl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&defaults.Server, "server", "", "Smith API server URL")
	fs.StringVar(&defaults.Token, "token", "", "Operator bearer token")
	fs.StringVar(&defaults.Config, "config", defaults.Config, "Path to smithctl config file")
	fs.StringVar(&defaults.Context, "context", "", "Named config context")
	fs.StringVar(&defaults.Output, "output", defaults.Output, "Output format: text|json")
	if err := fs.Parse(args); err != nil {
		return rootFlags{}, nil, err
	}
	if defaults.Output != "text" && defaults.Output != "json" {
		return rootFlags{}, nil, fmt.Errorf("invalid --output %q: expected text or json", defaults.Output)
	}
	return defaults, fs.Args(), nil
}

func resolveConfig(flags rootFlags) (runtimeConfig, error) {
	resolved := runtimeConfig{Server: defaultServer, Output: flags.Output}

	cfg, err := readFileConfig(flags.Config)
	if err != nil {
		return runtimeConfig{}, err
	}
	contextName := strings.TrimSpace(flags.Context)
	if contextName == "" {
		contextName = strings.TrimSpace(os.Getenv("SMITH_CONTEXT"))
	}
	if contextName == "" {
		contextName = strings.TrimSpace(cfg.CurrentContext)
	}
	if contextName == "" {
		contextName = "default"
	}
	if ctx, ok := cfg.Contexts[contextName]; ok {
		if strings.TrimSpace(ctx.Server) != "" {
			resolved.Server = strings.TrimSpace(ctx.Server)
		}
		if strings.TrimSpace(ctx.Token) != "" {
			resolved.Token = strings.TrimSpace(ctx.Token)
		}
	}

	if envServer := strings.TrimSpace(os.Getenv("SMITH_API_URL")); envServer != "" {
		resolved.Server = envServer
	}
	if envToken := strings.TrimSpace(os.Getenv("SMITH_OPERATOR_TOKEN")); envToken != "" {
		resolved.Token = envToken
	}

	if strings.TrimSpace(flags.Server) != "" {
		resolved.Server = strings.TrimSpace(flags.Server)
	}
	if strings.TrimSpace(flags.Token) != "" {
		resolved.Token = strings.TrimSpace(flags.Token)
	}
	return resolved, nil
}

func readFileConfig(path string) (fileConfig, error) {
	out := fileConfig{Contexts: map[string]contextConfig{}}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, nil
		}
		return fileConfig{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(content, &out); err != nil {
		return fileConfig{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if out.Contexts == nil {
		out.Contexts = map[string]contextConfig{}
	}
	return out, nil
}

func runLoop(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printLoopHelp(stdout)
		return 0
	}
	switch args[0] {
	case "list":
		return cmdLoopList(client, output, stdout, stderr)
	case "get":
		return cmdLoopGet(client, output, args[1:], stdout, stderr)
	case "create":
		return cmdLoopCreate(client, output, args[1:], stdout, stderr)
	case "ingest-github":
		return cmdLoopIngestGitHub(client, output, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown loop command %q\n", args[0])
		printLoopHelp(stderr)
		return 2
	}
}

func runPRD(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printPRDHelp(stdout)
		return 0
	}
	switch args[0] {
	case "submit":
		return cmdPRDSubmit(client, output, args[1:], stdout, stderr)
	case "create":
		return cmdPRDCreate(output, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown prd command %q\n", args[0])
		printPRDHelp(stderr)
		return 2
	}
}

func cmdLoopList(client *apiClient, output string, stdout, stderr io.Writer) int {
	var out any
	if err := client.doJSON(http.MethodGet, "/v1/loops", nil, &out); err != nil {
		fmt.Fprintf(stderr, "loop list failed: %v\n", err)
		return 1
	}
	printOutput(stdout, output, out)
	return 0
}

func cmdLoopGet(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: smithctl loop get <loop-id>")
		return 2
	}
	var out any
	if err := client.doJSON(http.MethodGet, "/v1/loops/"+args[0], nil, &out); err != nil {
		fmt.Fprintf(stderr, "loop get failed: %v\n", err)
		return 1
	}
	printOutput(stdout, output, out)
	return 0
}

func cmdLoopCreate(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath       string
		title          string
		description    string
		sourceType     string
		sourceRef      string
		idempotencyKey string
	)
	fs.StringVar(&filePath, "file", "", "JSON payload file for /v1/loops")
	fs.StringVar(&filePath, "f", "", "JSON payload file for /v1/loops")
	fs.StringVar(&title, "title", "", "Loop title")
	fs.StringVar(&description, "description", "", "Loop description")
	fs.StringVar(&sourceType, "source-type", "", "Loop source type")
	fs.StringVar(&sourceRef, "source-ref", "", "Loop source reference")
	fs.StringVar(&idempotencyKey, "idempotency-key", "", "Idempotency key")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	payload := map[string]any{}
	if strings.TrimSpace(filePath) != "" {
		bytes, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(stderr, "read file failed: %v\n", err)
			return 1
		}
		if err := json.Unmarshal(bytes, &payload); err != nil {
			fmt.Fprintf(stderr, "invalid json file: %v\n", err)
			return 1
		}
	} else {
		payload = map[string]any{
			"title":           title,
			"description":     description,
			"source_type":     sourceType,
			"source_ref":      sourceRef,
			"idempotency_key": idempotencyKey,
		}
	}
	var out any
	if err := client.doJSON(http.MethodPost, "/v1/loops", payload, &out); err != nil {
		fmt.Fprintf(stderr, "loop create failed: %v\n", err)
		return 1
	}
	printOutput(stdout, output, out)
	return 0
}

func cmdLoopIngestGitHub(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop ingest-github", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var filePath string
	fs.StringVar(&filePath, "file", "", "JSON file for /v1/ingress/github/issues")
	fs.StringVar(&filePath, "f", "", "JSON file for /v1/ingress/github/issues")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if strings.TrimSpace(filePath) == "" {
		fmt.Fprintln(stderr, "usage: smithctl loop ingest-github --file issues.json")
		return 2
	}
	payload, err := readJSONFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "read payload failed: %v\n", err)
		return 1
	}
	var out any
	if err := client.doJSON(http.MethodPost, "/v1/ingress/github/issues", payload, &out); err != nil {
		fmt.Fprintf(stderr, "loop ingest-github failed: %v\n", err)
		return 1
	}
	printOutput(stdout, output, out)
	return 0
}

func cmdPRDSubmit(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prd submit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath  string
		format    string
		sourceRef string
	)
	fs.StringVar(&filePath, "file", "", "PRD file path")
	fs.StringVar(&filePath, "f", "", "PRD file path")
	fs.StringVar(&format, "format", "", "markdown|json")
	fs.StringVar(&sourceRef, "source-ref", "", "PRD source reference")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if strings.TrimSpace(filePath) == "" {
		fmt.Fprintln(stderr, "usage: smithctl prd submit --file prd.md [--format markdown|json]")
		return 2
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "read file failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(format) == "" {
		if strings.HasSuffix(strings.ToLower(filePath), ".json") {
			format = "json"
		} else {
			format = "markdown"
		}
	}
	payload := map[string]any{"format": format, "source_ref": sourceRef}
	switch format {
	case "markdown", "md":
		payload["markdown"] = string(content)
	case "json", "structured":
		var parsed any
		if err := json.Unmarshal(content, &parsed); err != nil {
			fmt.Fprintf(stderr, "invalid json prd payload: %v\n", err)
			return 1
		}
		asMap, ok := parsed.(map[string]any)
		if !ok {
			fmt.Fprintln(stderr, "json prd payload must be an object")
			return 1
		}
		if tasks, ok := asMap["tasks"]; ok {
			payload["tasks"] = tasks
		}
	default:
		fmt.Fprintln(stderr, "--format must be markdown or json")
		return 2
	}
	var out any
	if err := client.doJSON(http.MethodPost, "/v1/ingress/prd", payload, &out); err != nil {
		fmt.Fprintf(stderr, "prd submit failed: %v\n", err)
		return 1
	}
	printOutput(stdout, output, out)
	return 0
}

func cmdPRDCreate(output string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: smithctl prd create [name]")
		return 2
	}
	name := "Smith PRD"
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		name = strings.TrimSpace(args[0])
	}
	template := fmt.Sprintf("# %s\n\n## Tasks\n- [ ] Describe first deliverable\n- [ ] Describe second deliverable\n", name)
	if output == "json" {
		printOutput(stdout, output, map[string]string{"markdown": template})
		return 0
	}
	fmt.Fprint(stdout, template)
	return 0
}

func (c *apiClient) doJSON(method, path string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, c.baseURL+path, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func readJSONFile(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func printOutput(w io.Writer, format string, value any) {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(value)
		return
	}
	formatted, _ := json.MarshalIndent(value, "", "  ")
	fmt.Fprintln(w, string(formatted))
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "smithctl - Smith operator CLI")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  smithctl [--server URL] [--token TOKEN] [--output text|json] <resource> <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Resources:")
	fmt.Fprintln(w, "  loop    Manage loop resources")
	fmt.Fprintln(w, "  prd     Manage PRD resources")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  smithctl loop list")
	fmt.Fprintln(w, "  smithctl loop get loop-abc123")
	fmt.Fprintln(w, "  smithctl loop create --title \"Fix drift\" --source-type github_issue --source-ref org/repo#1")
	fmt.Fprintln(w, "  smithctl loop ingest-github --file issues.json")
	fmt.Fprintln(w, "  smithctl prd submit --file docs/prd1.md")
}

func printLoopHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: smithctl loop <command>")
	fmt.Fprintln(w, "Commands: list, get, create, ingest-github")
}

func printPRDHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: smithctl prd <command>")
	fmt.Fprintln(w, "Commands: create, submit")
}
