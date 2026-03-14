package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultServer = "http://127.0.0.1:8080"

var (
	Version   = "v0.0.0"
	GitCommit = "unknown"
)

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

type stringMapFlag map[string]string

func (s *stringMapFlag) String() string {
	if s == nil || len(*s) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*s))
	for k, v := range *s {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (s *stringMapFlag) Set(value string) error {
	parts := strings.SplitN(strings.TrimSpace(value), "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("expected key=value")
	}
	if *s == nil {
		*s = map[string]string{}
	}
	(*s)[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	return nil
}

type skillFlag []map[string]any

func (s *skillFlag) String() string {
	return ""
}

func (s *skillFlag) Set(value string) error {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return fmt.Errorf("skill spec is required")
	}
	spec := map[string]any{}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid skill field %q (expected key=value)", strings.TrimSpace(part))
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		switch key {
		case "name", "source", "version", "mount_path":
			if val == "" {
				return fmt.Errorf("skill %s is required", key)
			}
			spec[key] = val
		case "read_only":
			parsed, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("skill read_only must be true|false")
			}
			spec[key] = parsed
		default:
			return fmt.Errorf("unsupported skill field %q", key)
		}
	}
	if _, ok := spec["name"]; !ok {
		return fmt.Errorf("skill name is required")
	}
	if _, ok := spec["source"]; !ok {
		return fmt.Errorf("skill source is required")
	}
	*s = append(*s, spec)
	return nil
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
	case "version":
		if cfg.Output == "json" {
			printOutput(stdout, cfg.Output, map[string]string{
				"version":    Version,
				"git_commit": GitCommit,
			})
		} else {
			fmt.Fprintf(stdout, "smithctl version %s (%s)\n", Version, GitCommit)
		}
		return 0
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
	case "trace":
		return cmdLoopTrace(client, output, args[1:], stdout, stderr)
	case "create":
		return cmdLoopCreate(client, output, args[1:], stdout, stderr)
	case "logs":
		return cmdLoopLogs(client, output, args[1:], stdout, stderr)
	case "runtime":
		return cmdLoopRuntime(client, output, args[1:], stdout, stderr)
	case "cost":
		return cmdLoopCost(client, output, args[1:], stdout, stderr)
	case "attach":
		return cmdLoopAttach(client, output, args[1:], stdout, stderr)

	case "detach":
		return cmdLoopDetach(client, output, args[1:], stdout, stderr)
	case "command":
		return cmdLoopCommand(client, output, args[1:], stdout, stderr)
	case "cancel":
		return cmdLoopCancel(client, output, args[1:], stdout, stderr)
	case "ingest-github":
		return cmdLoopIngestGitHub(client, output, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown loop command %q\n", args[0])
		printLoopHelp(stderr)
		return 2
	}
}

func cmdLoopTrace(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop trace", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath string
		limit    int
	)
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	fs.IntVar(&limit, "limit", 500, "Maximum entries per trace section")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop trace failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop trace <loop-id> [<loop-id>...] [--limit N] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		var trace any
		path := "/v1/loops/" + loopID + "/trace?limit=" + strconv.Itoa(limit)
		if err := client.doJSON(http.MethodGet, path, nil, &trace); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"trace":   trace,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["trace"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
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
	fs := flag.NewFlagSet("loop get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var filePath string
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop get failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop get <loop-id> [<loop-id>...] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		var out any
		if err := client.doJSON(http.MethodGet, "/v1/loops/"+loopID, nil, &out); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"result":  out,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["result"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func cmdLoopCreate(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath         string
		title            string
		description      string
		providerID       string
		sourceType       string
		sourceRef        string
		idempotencyKey   string
		batchFile        string
		fromGitHub       string
		fromPRD          string
		format           string
		workspacePrompt  string
		workspacePRD     string
		workspacePRDFile string
		workspacePRDPath string
		envPreset        string
		envMiseFile      string
		envImageRef      string
		envImagePull     string
		envDockerCtx     string
		envDockerFile    string
		envDockerTarget  string
	)
	var envTools stringMapFlag
	var envBuildArgs stringMapFlag
	var skills skillFlag
	fs.StringVar(&filePath, "file", "", "JSON payload file for /v1/loops")
	fs.StringVar(&filePath, "f", "", "JSON payload file for /v1/loops")
	fs.StringVar(&title, "title", "", "Loop title")
	fs.StringVar(&description, "description", "", "Loop description")
	fs.StringVar(&providerID, "provider-id", "", "Loop provider id (defaults to server default)")
	fs.StringVar(&sourceType, "source-type", "", "Loop source type")
	fs.StringVar(&sourceRef, "source-ref", "", "Loop source reference")
	fs.StringVar(&idempotencyKey, "idempotency-key", "", "Idempotency key")
	fs.StringVar(&batchFile, "batch", "", "JSON array/object file for batch loop create")
	fs.StringVar(&batchFile, "batch-file", "", "JSON array/object file for batch loop create")
	fs.StringVar(&fromGitHub, "from-github", "", "JSON file for /v1/ingress/github/issues")
	fs.StringVar(&fromPRD, "from-prd", "", "PRD file path (markdown or json)")
	fs.StringVar(&format, "format", "", "PRD format override: markdown|json")
	fs.StringVar(&workspacePrompt, "workspace-prompt", "", "Prompt text for PRD generation/build workflows")
	fs.StringVar(&workspacePRD, "workspace-prd-json", "", "Inline PRD JSON payload to materialize in replica workspace")
	fs.StringVar(&workspacePRDFile, "workspace-prd-file", "", "PRD JSON file to materialize in replica workspace")
	fs.StringVar(&workspacePRDPath, "workspace-prd-path", ".agents/tasks/prd.json", "Workspace PRD path for supplied PRD JSON")
	fs.StringVar(&envPreset, "env-preset", "", "Environment preset name (standard|secure|performance|minimal)")
	fs.StringVar(&envMiseFile, "env-mise-file", "", "mise tool versions file path")
	fs.Var(&envTools, "env-tool", "mise tool pin in key=value format (repeatable)")
	fs.StringVar(&envImageRef, "env-image-ref", "", "Environment container image reference")
	fs.StringVar(&envImagePull, "env-image-pull-policy", "", "Environment image pull policy: Always|IfNotPresent|Never")
	fs.StringVar(&envDockerCtx, "env-docker-context", "", "Environment docker build context directory")
	fs.StringVar(&envDockerFile, "env-dockerfile", "", "Environment dockerfile path")
	fs.StringVar(&envDockerTarget, "env-docker-target", "", "Environment docker build target stage")
	fs.Var(&envBuildArgs, "env-build-arg", "Docker build arg in key=value format (repeatable)")
	fs.Var(&skills, "skill", "Skill mount spec: name=...,source=...[,version=...][,mount_path=/...][,read_only=true|false] (repeatable). If mount_path is omitted, Codex defaults to /smith/skills/<name>.")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	selectedModes := 0
	for _, value := range []string{filePath, batchFile, fromGitHub, fromPRD} {
		if strings.TrimSpace(value) != "" {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		fmt.Fprintln(stderr, "loop create flags are mutually exclusive: --file, --batch, --from-github, --from-prd")
		return 2
	}
	hasEnvironmentFlags := strings.TrimSpace(envPreset) != "" ||
		strings.TrimSpace(envMiseFile) != "" ||
		len(envTools) > 0 ||
		strings.TrimSpace(envImageRef) != "" ||
		strings.TrimSpace(envImagePull) != "" ||
		strings.TrimSpace(envDockerCtx) != "" ||
		strings.TrimSpace(envDockerFile) != "" ||
		strings.TrimSpace(envDockerTarget) != "" ||
		len(envBuildArgs) > 0
	hasWorkspaceFlags := strings.TrimSpace(workspacePrompt) != "" ||
		strings.TrimSpace(workspacePRD) != "" ||
		strings.TrimSpace(workspacePRDFile) != "" ||
		strings.TrimSpace(workspacePRDPath) != ".agents/tasks/prd.json"
	hasProviderFlags := strings.TrimSpace(providerID) != ""
	if selectedModes > 0 && (hasEnvironmentFlags || len(skills) > 0 || hasWorkspaceFlags || hasProviderFlags) {
		fmt.Fprintln(stderr, "environment/skill/workspace/provider flags are only supported with direct loop create (not --file/--batch/--from-github/--from-prd)")
		return 2
	}

	environment, envErr := buildEnvironmentPayload(envPreset, envMiseFile, envTools, envImageRef, envImagePull, envDockerCtx, envDockerFile, envDockerTarget, envBuildArgs)
	if envErr != nil {
		fmt.Fprintf(stderr, "loop create failed: %v\n", envErr)
		return 2
	}
	if strings.TrimSpace(fromGitHub) != "" {
		var payload any
		// Try to treat it as a file first
		if _, err := os.Stat(fromGitHub); err == nil {
			payload, err = readJSONFileAny(fromGitHub)
			if err != nil {
				fmt.Fprintf(stderr, "read github payload failed: %v\n", err)
				return 1
			}
			if _, ok := payload.([]any); ok {
				payload = map[string]any{"issues": payload}
			}
		} else {
			// Treat as issue ID or repo#ID
			issueRef := strings.TrimSpace(fromGitHub)
			repo := ""
			numberStr := issueRef
			if idx := strings.Index(issueRef, "#"); idx != -1 {
				repo = issueRef[:idx]
				numberStr = issueRef[idx+1:]
			}
			if repo == "" {
				// Try to infer repo from current git context
				repo = inferCurrentRepo()
			}
			if repo == "" {
				fmt.Fprintln(stderr, "repository could not be inferred; use 'repo#number' format for --from-github")
				return 1
			}
			num, err := strconv.Atoi(numberStr)
			if err != nil {
				fmt.Fprintf(stderr, "invalid github issue number %q: %v\n", numberStr, err)
				return 1
			}
			payload = map[string]any{
				"issues": []any{
					map[string]any{
						"repository": repo,
						"number":     num,
						"title":      fmt.Sprintf("Issue from %s#%d", repo, num),
					},
				},
			}
		}

		var out any
		if err := client.doJSON(http.MethodPost, "/v1/ingress/github/issues", payload, &out); err != nil {
			fmt.Fprintf(stderr, "loop create failed: %v\n", err)
			return 1
		}
		printOutput(stdout, output, out)
		return 0
	}

	if strings.TrimSpace(fromPRD) != "" {
		content, err := os.ReadFile(fromPRD)
		if err != nil {
			fmt.Fprintf(stderr, "read prd file failed: %v\n", err)
			return 1
		}
		prdFormat := strings.ToLower(strings.TrimSpace(format))
		if prdFormat == "" {
			if strings.HasSuffix(strings.ToLower(fromPRD), ".json") {
				prdFormat = "json"
			} else {
				prdFormat = "markdown"
			}
		}
		payload := map[string]any{
			"format":     prdFormat,
			"source_ref": sourceRef,
		}
		switch prdFormat {
		case "markdown", "md":
			payload["format"] = "markdown"
			payload["markdown"] = string(content)
		case "json", "structured":
			payload["format"] = "json"
			var parsed any
			if err := json.Unmarshal(content, &parsed); err != nil {
				fmt.Fprintf(stderr, "invalid json prd payload: %v\n", err)
				return 1
			}
			if asMap, ok := parsed.(map[string]any); ok {
				if tasks, hasTasks := asMap["tasks"]; hasTasks {
					payload["tasks"] = tasks
				} else {
					payload["tasks"] = []any{}
				}
			} else if asSlice, ok := parsed.([]any); ok {
				payload["tasks"] = asSlice
			} else {
				fmt.Fprintln(stderr, "json prd payload must be an object or array")
				return 1
			}
		default:
			fmt.Fprintln(stderr, "--format must be markdown or json")
			return 2
		}
		var out any
		if err := client.doJSON(http.MethodPost, "/v1/ingress/prd", payload, &out); err != nil {
			fmt.Fprintf(stderr, "loop create failed: %v\n", err)
			return 1
		}
		printOutput(stdout, output, out)
		return 0
	}
	var payload any
	if strings.TrimSpace(batchFile) != "" {
		rawPayload, err := readJSONFileAny(batchFile)
		if err != nil {
			fmt.Fprintf(stderr, "read batch file failed: %v\n", err)
			return 1
		}
		if _, ok := rawPayload.([]any); ok {
			payload = map[string]any{"loops": rawPayload}
		} else {
			payload = rawPayload
		}
	} else if strings.TrimSpace(filePath) != "" {
		rawPayload, err := readJSONFileAny(filePath)
		if err != nil {
			fmt.Fprintf(stderr, "read file failed: %v\n", err)
			return 1
		}
		payload = rawPayload
	} else {
		workspacePrompt = strings.TrimSpace(workspacePrompt)
		workspacePRD = strings.TrimSpace(workspacePRD)
		workspacePRDFile = strings.TrimSpace(workspacePRDFile)
		workspacePRDPath = strings.TrimSpace(workspacePRDPath)
		if workspacePRDFile != "" {
			content, err := os.ReadFile(workspacePRDFile)
			if err != nil {
				fmt.Fprintf(stderr, "loop create failed: read --workspace-prd-file: %v\n", err)
				return 1
			}
			workspacePRD = strings.TrimSpace(string(content))
		}
		metadata := map[string]string{}
		if workspacePrompt != "" {
			metadata["workspace_prompt"] = workspacePrompt
		}
		if workspacePRD != "" {
			if !json.Valid([]byte(workspacePRD)) {
				fmt.Fprintln(stderr, "loop create failed: --workspace-prd-json must be valid json")
				return 2
			}
			if workspacePRDPath == "" {
				workspacePRDPath = ".agents/tasks/prd.json"
			}
			metadata["workspace_prd_json"] = workspacePRD
			metadata["workspace_prd_path"] = workspacePRDPath
		}
		if sourceType == "" && (workspacePrompt != "" || workspacePRD != "") {
			sourceType = "prompt"
		}
		if sourceRef == "" && sourceType == "prompt" {
			sourceRef = "prompt:smithctl"
		}
		if title == "" {
			switch {
			case workspacePRD != "":
				title = "Loop from supplied PRD"
			case workspacePrompt != "":
				title = "Loop from prompt"
			}
		}
		if description == "" {
			switch {
			case workspacePRD != "":
				description = "Loop request from supplied PRD JSON"
			case workspacePrompt != "":
				description = workspacePrompt
			}
		}
		payload = map[string]any{
			"title":           title,
			"description":     description,
			"source_type":     sourceType,
			"source_ref":      sourceRef,
			"idempotency_key": idempotencyKey,
		}
		if strings.TrimSpace(providerID) != "" {
			payload.(map[string]any)["provider_id"] = strings.ToLower(strings.TrimSpace(providerID))
		}
		if environment != nil {
			payload.(map[string]any)["environment"] = environment
		}
		if len(skills) > 0 {
			payload.(map[string]any)["skills"] = []map[string]any(skills)
		}
		if len(metadata) > 0 {
			payload.(map[string]any)["metadata"] = metadata
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

func buildEnvironmentPayload(
	preset string,
	miseFile string,
	tools stringMapFlag,
	imageRef string,
	imagePull string,
	dockerContext string,
	dockerfilePath string,
	dockerTarget string,
	buildArgs stringMapFlag,
) (map[string]any, error) {
	preset = strings.TrimSpace(preset)
	miseFile = strings.TrimSpace(miseFile)
	imageRef = strings.TrimSpace(imageRef)
	imagePull = strings.TrimSpace(imagePull)
	dockerContext = strings.TrimSpace(dockerContext)
	dockerfilePath = strings.TrimSpace(dockerfilePath)
	dockerTarget = strings.TrimSpace(dockerTarget)

	hasMise := miseFile != "" || len(tools) > 0
	hasImage := imageRef != ""
	hasDocker := dockerContext != "" || dockerfilePath != "" || dockerTarget != "" || len(buildArgs) > 0
	modeCount := 0
	if hasMise {
		modeCount++
	}
	if hasImage {
		modeCount++
	}
	if hasDocker {
		modeCount++
	}
	if modeCount > 1 {
		return nil, fmt.Errorf("environment source conflict: specify only one of mise, container_image, or dockerfile")
	}
	if imagePull != "" && !hasImage {
		return nil, fmt.Errorf("--env-image-pull-policy requires --env-image-ref")
	}
	if (dockerContext == "") != (dockerfilePath == "") {
		return nil, fmt.Errorf("dockerfile mode requires both --env-docker-context and --env-dockerfile")
	}

	environment := map[string]any{}
	if preset != "" {
		environment["preset"] = preset
	}
	if hasMise {
		mise := map[string]any{}
		if miseFile != "" {
			mise["tool_versions_file"] = miseFile
		}
		if len(tools) > 0 {
			mise["tools"] = map[string]string(tools)
		}
		environment["mise"] = mise
	}
	if hasImage {
		image := map[string]any{"ref": imageRef}
		if imagePull != "" {
			image["pull_policy"] = imagePull
		}
		environment["container_image"] = image
	}
	if hasDocker {
		dockerfile := map[string]any{
			"context_dir":     dockerContext,
			"dockerfile_path": dockerfilePath,
		}
		if dockerTarget != "" {
			dockerfile["target"] = dockerTarget
		}
		if len(buildArgs) > 0 {
			dockerfile["build_args"] = map[string]string(buildArgs)
		}
		environment["dockerfile"] = dockerfile
	}
	if len(environment) == 0 {
		return nil, nil
	}
	return environment, nil
}

func cmdLoopLogs(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath string
		follow   bool
		limit    int
		interval time.Duration
	)
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	fs.BoolVar(&follow, "follow", false, "Follow loop journal entries")
	fs.BoolVar(&follow, "F", false, "Follow loop journal entries")
	fs.IntVar(&limit, "limit", 500, "Maximum journal entries to fetch")
	fs.DurationVar(&interval, "interval", 2*time.Second, "Poll interval when --follow is set")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop logs failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop logs <loop-id> [<loop-id>...] [--follow] [--file ids.json]")
		return 2
	}
	if follow {
		return followLoopLogs(client, output, loopIDs, int64(limit), interval, stdout, stderr)
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		var journal any
		path := "/v1/loops/" + loopID + "/journal?limit=" + strconv.Itoa(limit)
		if err := client.doJSON(http.MethodGet, path, nil, &journal); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"journal": journal,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["journal"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func cmdLoopAttach(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop attach", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath string
		actor    string
		follow   bool
		limit    int
		interval time.Duration
	)
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&actor, "actor", "operator", "Attach actor identifier")
	fs.BoolVar(&follow, "follow", true, "Follow logs after attach")
	fs.IntVar(&limit, "limit", 500, "Maximum journal entries to fetch")
	fs.DurationVar(&interval, "interval", 2*time.Second, "Poll interval when following")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop attach failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop attach <loop-id> [<loop-id>...] [--file ids.json]")
		return 2
	}

	attachResults := make([]map[string]any, 0, len(loopIDs))
	hardFailure := false
	softFailure := false
	for _, loopID := range loopIDs {
		payload := map[string]any{
			"actor":    actor,
			"terminal": "smithctl",
		}
		var out any
		err := client.doJSON(http.MethodPost, "/v1/loops/"+loopID+"/control/attach", payload, &out)
		if err == nil {
			attachResults = append(attachResults, map[string]any{
				"loop_id": loopID,
				"status":  "attached",
				"result":  out,
			})
			continue
		}
		if strings.Contains(err.Error(), "returned 404") {
			attachResults = append(attachResults, map[string]any{
				"loop_id": loopID,
				"status":  "not_supported",
				"error":   err.Error(),
			})
			softFailure = true
			continue
		}
		attachResults = append(attachResults, map[string]any{
			"loop_id": loopID,
			"status":  "error",
			"error":   err.Error(),
		})
		hardFailure = true
	}
	printOutput(stdout, output, map[string]any{"results": attachResults})
	if hardFailure {
		return 1
	}
	if follow {
		if softFailure {
			fmt.Fprintln(stderr, "attach endpoint unavailable for one or more loops; following journal instead")
		}
		return followLoopLogs(client, output, loopIDs, int64(limit), interval, stdout, stderr)
	}
	return 0
}

func cmdLoopCancel(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop cancel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath string
		reason   string
		actor    string
	)
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&reason, "reason", "cancelled via smithctl", "Cancellation reason")
	fs.StringVar(&actor, "actor", "operator", "Actor performing cancellation")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop cancel failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop cancel <loop-id> [<loop-id>...] [--reason text] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		payload := map[string]any{
			"loop_id":      loopID,
			"target_state": "cancelled",
			"reason":       reason,
			"actor":        actor,
		}
		var out any
		if err := client.doJSON(http.MethodPost, "/v1/control/override", payload, &out); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"result":  out,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["result"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func cmdLoopDetach(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop detach", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		filePath string
		actor    string
	)
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&actor, "actor", "operator", "Detach actor identifier")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop detach failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop detach <loop-id> [<loop-id>...] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		payload := map[string]any{"actor": actor}
		var out any
		err := client.doJSON(http.MethodPost, "/v1/loops/"+loopID+"/control/detach", payload, &out)
		if err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"result":  out,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["result"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func cmdLoopCommand(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	loopIDArg := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		loopIDArg = strings.TrimSpace(args[0])
		parseArgs = args[1:]
	}
	fs := flag.NewFlagSet("loop command", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		actor   string
		command string
	)
	fs.StringVar(&actor, "actor", "operator", "Actor issuing command")
	fs.StringVar(&command, "command", "", "Command payload")
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopID := loopIDArg
	if loopID == "" {
		if len(fs.Args()) != 1 {
			fmt.Fprintln(stderr, "usage: smithctl loop command <loop-id> --command \"pause|resume|...\"")
			return 2
		}
		loopID = strings.TrimSpace(fs.Args()[0])
	}
	if loopID == "" {
		fmt.Fprintln(stderr, "usage: smithctl loop command <loop-id> --command \"pause|resume|...\"")
		return 2
	}
	if strings.TrimSpace(command) == "" {
		fmt.Fprintln(stderr, "--command is required")
		return 2
	}
	var out any
	payload := map[string]any{
		"actor":   actor,
		"command": command,
	}
	if err := client.doJSON(http.MethodPost, "/v1/loops/"+loopID+"/control/command", payload, &out); err != nil {
		fmt.Fprintf(stderr, "loop command failed: %v\n", err)
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
	printOutput(stdout, output, normalizePRDSubmitOutput(out))
	return 0
}

func cmdPRDCreate(output string, args []string, stdout, stderr io.Writer) int {
	nameArg := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		nameArg = strings.TrimSpace(args[0])
		parseArgs = args[1:]
	}

	fs := flag.NewFlagSet("prd create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		templateName string
		outputPath   string
	)
	fs.StringVar(&templateName, "template", "default", "Template: default|feature|bugfix")
	fs.StringVar(&outputPath, "out", "", "Write scaffolded PRD to file")
	fs.StringVar(&outputPath, "output-file", "", "Write scaffolded PRD to file")
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintln(stderr, "usage: smithctl prd create [name] [--template default|feature|bugfix] [--out path]")
		return 2
	}
	name := "Smith PRD"
	if strings.TrimSpace(nameArg) != "" {
		name = strings.TrimSpace(nameArg)
	}

	template, err := renderPRDTemplate(strings.ToLower(strings.TrimSpace(templateName)), name)
	if err != nil {
		fmt.Fprintf(stderr, "prd create failed: %v\n", err)
		return 2
	}
	if strings.TrimSpace(outputPath) != "" {
		if err := os.WriteFile(outputPath, []byte(template), 0o644); err != nil {
			fmt.Fprintf(stderr, "write prd file failed: %v\n", err)
			return 1
		}
		meta := map[string]any{
			"template":    strings.ToLower(strings.TrimSpace(templateName)),
			"output_file": outputPath,
			"bytes":       len(template),
		}
		if output == "json" {
			meta["markdown"] = template
			printOutput(stdout, output, meta)
		} else {
			fmt.Fprintf(stdout, "wrote PRD template %q to %s\n", meta["template"], outputPath)
		}
		return 0
	}
	if output == "json" {
		printOutput(stdout, output, map[string]any{
			"template": strings.ToLower(strings.TrimSpace(templateName)),
			"markdown": template,
		})
		return 0
	}
	fmt.Fprint(stdout, template)
	return 0
}

func renderPRDTemplate(templateName, name string) (string, error) {
	switch templateName {
	case "", "default":
		return fmt.Sprintf("# %s\n\n## Context\nDescribe problem context.\n\n## Tasks\n- [ ] Describe first deliverable\n- [ ] Describe second deliverable\n", name), nil
	case "feature":
		return fmt.Sprintf("# %s\n\n## Goal\nDescribe user-facing outcome.\n\n## Scope\n- In scope:\n- Out of scope:\n\n## Tasks\n- [ ] API changes\n- [ ] Runtime changes\n- [ ] Validation and tests\n", name), nil
	case "bugfix":
		return fmt.Sprintf("# %s\n\n## Bug Summary\nDescribe failing behavior and impact.\n\n## Reproduction\n- [ ] Repro steps documented\n\n## Tasks\n- [ ] Root-cause analysis\n- [ ] Fix implementation\n- [ ] Regression test coverage\n", name), nil
	default:
		return "", fmt.Errorf("unknown template %q (expected default, feature, or bugfix)", templateName)
	}
}

func normalizePRDSubmitOutput(out any) any {
	asMap, ok := out.(map[string]any)
	if !ok {
		return out
	}
	results, ok := asMap["results"].([]any)
	if !ok {
		return out
	}
	loopIDs := make([]string, 0)
	validationErrors := make([]map[string]any, 0)
	for _, raw := range results {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if loopID, ok := item["loop_id"].(string); ok && strings.TrimSpace(loopID) != "" {
			loopIDs = append(loopIDs, strings.TrimSpace(loopID))
		}
		if status, _ := item["status"].(string); status == "error" {
			entry := map[string]any{
				"message": item["message"],
			}
			if sourceRef, ok := item["source_ref"]; ok {
				entry["source_ref"] = sourceRef
			}
			if idx, ok := item["item_index"]; ok {
				entry["item_index"] = idx
			}
			validationErrors = append(validationErrors, entry)
		}
	}
	asMap["loop_ids"] = loopIDs
	asMap["validation_errors"] = validationErrors
	return asMap
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

func readJSONFileAny(path string) (any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func collectLoopIDs(filePath string, args []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(args))
	add := func(raw string) {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, arg := range args {
		add(arg)
	}
	if strings.TrimSpace(filePath) == "" {
		return out, nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(content, &raw); err == nil {
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				if value, ok := item.(string); ok {
					add(value)
				}
			}
		case map[string]any:
			if ids, ok := typed["ids"].([]any); ok {
				for _, item := range ids {
					if value, ok := item.(string); ok {
						add(value)
					}
				}
			}
		}
		return out, nil
	}
	for _, line := range strings.Split(string(content), "\n") {
		add(line)
	}
	return out, nil
}

func followLoopLogs(client *apiClient, output string, loopIDs []string, limit int64, interval time.Duration, stdout, stderr io.Writer) int {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lastSeq := map[string]int64{}
	for _, loopID := range loopIDs {
		lastSeq[loopID] = -1
	}

	for {
		allTerminal := true
		hadError := false

		for _, loopID := range loopIDs {
			var journal []map[string]any
			path := "/v1/loops/" + loopID + "/journal?limit=" + strconv.FormatInt(limit, 10)
			if err := client.doJSON(http.MethodGet, path, nil, &journal); err != nil {
				fmt.Fprintf(stderr, "loop logs follow failed (%s): %v\n", loopID, err)
				hadError = true
				allTerminal = false
				continue
			}
			for _, entry := range journal {
				seq := parseEntrySequence(entry, lastSeq[loopID]+1)
				if seq <= lastSeq[loopID] {
					continue
				}
				lastSeq[loopID] = seq
				printOutput(stdout, output, map[string]any{
					"loop_id": loopID,
					"entry":   entry,
				})
			}
			var loopOut map[string]any
			if err := client.doJSON(http.MethodGet, "/v1/loops/"+loopID, nil, &loopOut); err != nil {
				allTerminal = false
				continue
			}
			if !isLoopTerminal(loopOut) {
				allTerminal = false
			}
		}

		if allTerminal {
			return 0
		}
		if hadError {
			select {
			case <-ctx.Done():
				return 1
			case <-time.After(interval):
				continue
			}
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(interval):
		}
	}
}

func parseEntrySequence(entry map[string]any, fallback int64) int64 {
	raw, ok := entry["sequence"]
	if !ok {
		return fallback
	}
	switch typed := raw.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return fallback
	}
}

func isLoopTerminal(loopOut map[string]any) bool {
	stateBlock, ok := loopOut["state"].(map[string]any)
	if !ok {
		return false
	}
	stateValue, _ := stateBlock["state"].(string)
	switch stateValue {
	case "synced", "flatline", "cancelled":
		return true
	default:
		return false
	}
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
	fmt.Fprintln(w, "  version Print the version information")

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  smithctl loop list")
	fmt.Fprintln(w, "  smithctl loop get loop-abc123")
	fmt.Fprintln(w, "  smithctl loop trace loop-abc123 --limit 100")
	fmt.Fprintln(w, "  smithctl loop get loop-abc123 loop-def456")
	fmt.Fprintln(w, "  smithctl loop create --title \"Fix drift\" --source-type github_issue --source-ref org/repo#1")
	fmt.Fprintln(w, "  smithctl loop create --title \"Env test\" --source-type interactive --source-ref terminal/session-01 --env-image-ref ghcr.io/acme/replica:v2")
	fmt.Fprintln(w, "  smithctl loop create --title \"Dockerfile run\" --source-type prd_task --source-ref docs/prd.md#1 --env-docker-context . --env-dockerfile Dockerfile --env-build-arg GO_VERSION=1.22")
	fmt.Fprintln(w, "  smithctl loop create --title \"Skill run\" --source-type interactive --source-ref terminal/session-02 --skill name=commit,source=local://skills/commit")
	fmt.Fprintln(w, "  smithctl loop create --batch loops.json")
	fmt.Fprintln(w, "  smithctl loop create --from-github issues.json")
	fmt.Fprintln(w, "  smithctl loop create --from-prd docs/prd1.md --source-ref prd:docs/prd1.md")
	fmt.Fprintln(w, "  smithctl loop logs loop-abc123 --follow")
	fmt.Fprintln(w, "  smithctl loop runtime loop-abc123")
	fmt.Fprintln(w, "  smithctl loop cost loop-abc123")
	fmt.Fprintln(w, "  smithctl loop attach loop-abc123")

	fmt.Fprintln(w, "  smithctl loop command loop-abc123 --command \"pause\"")
	fmt.Fprintln(w, "  smithctl loop detach loop-abc123")
	fmt.Fprintln(w, "  smithctl loop cancel loop-abc123 --reason \"operator request\"")
	fmt.Fprintln(w, "  smithctl loop ingest-github --file issues.json")
	fmt.Fprintln(w, "  smithctl prd create \"Auth Flow\" --template feature --out docs/prd-auth.md")
	fmt.Fprintln(w, "  smithctl prd submit --file docs/prd1.md")
}

func printLoopHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: smithctl loop <command>")
	fmt.Fprintln(w, "Commands: list, get, trace, create, logs, runtime, cost, attach, detach, command, cancel, ingest-github")
}

func cmdLoopRuntime(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop runtime", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var filePath string
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop runtime failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop runtime <loop-id> [<loop-id>...] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		var runtime any
		path := "/v1/loops/" + loopID + "/runtime"
		if err := client.doJSON(http.MethodGet, path, nil, &runtime); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"runtime": runtime,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["runtime"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func cmdLoopCost(client *apiClient, output string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loop cost", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var filePath string
	fs.StringVar(&filePath, "file", "", "JSON or newline-delimited loop id file")
	fs.StringVar(&filePath, "f", "", "JSON or newline-delimited loop id file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	loopIDs, err := collectLoopIDs(filePath, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "loop cost failed: %v\n", err)
		return 1
	}
	if len(loopIDs) == 0 {
		fmt.Fprintln(stderr, "usage: smithctl loop cost <loop-id> [<loop-id>...] [--file ids.json]")
		return 2
	}
	results := make([]map[string]any, 0, len(loopIDs))
	failed := false
	for _, loopID := range loopIDs {
		var cost any
		path := "/v1/reporting/cost?loop_id=" + loopID
		if err := client.doJSON(http.MethodGet, path, nil, &cost); err != nil {
			results = append(results, map[string]any{
				"loop_id": loopID,
				"status":  "error",
				"error":   err.Error(),
			})
			failed = true
			continue
		}
		results = append(results, map[string]any{
			"loop_id": loopID,
			"status":  "ok",
			"cost":    cost,
		})
	}
	if len(loopIDs) == 1 && !failed {
		printOutput(stdout, output, results[0]["cost"])
		return 0
	}
	printOutput(stdout, output, map[string]any{"results": results})
	if failed {
		return 1
	}
	return 0
}

func inferCurrentRepo() string {
	out, err := runExternalCommand("git", "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(out) == "" {
		return ""
	}
	repoURL := strings.TrimSpace(out)
	repoURL = strings.TrimSuffix(repoURL, ".git")
	if strings.HasPrefix(repoURL, "git@github.com:") {
		return strings.TrimPrefix(repoURL, "git@github.com:")
	}
	if strings.HasPrefix(repoURL, "https://github.com/") {
		return strings.TrimPrefix(repoURL, "https://github.com/")
	}
	return ""
}

func runExternalCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}
func printPRDHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: smithctl prd <command>")
	fmt.Fprintln(w, "Commands: create, submit")
	fmt.Fprintln(w, "  create [name] [--template default|feature|bugfix] [--out path]")
	fmt.Fprintln(w, "  submit --file <path> [--format markdown|json] [--source-ref ref]")
}
