package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	api "smith/pkg/api/v1"
)

// Client is a Smith API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s returned %d: %s", e.Method, e.Path, e.StatusCode, strings.TrimSpace(string(e.Body)))
}

func (e *APIError) JSONBody() bool {
	return len(e.Body) > 0 && json.Valid(e.Body)
}

// NewClient creates a new Smith API client
func NewClient(baseURL string, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

// SetHTTPClient sets a custom http client
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

// CreateLoop creates a new loop
func (c *Client) CreateLoop(ctx context.Context, req api.LoopCreateRequest) (*api.LoopCreateResult, error) {
	var res api.LoopCreateResult
	err := c.do(ctx, http.MethodPost, "/v1/loops", req, &res)
	return &res, err
}

// CreateLoops creates multiple loops in a batch
func (c *Client) CreateLoops(ctx context.Context, req api.LoopBatchRequest) (*api.LoopCreateResult, error) {
	var res api.LoopCreateResult
	err := c.do(ctx, http.MethodPost, "/v1/loops", req, &res)
	return &res, err
}

// GetLoop returns loop details
func (c *Client) GetLoop(ctx context.Context, loopID string) (*api.LoopResponse, error) {
	var res api.LoopResponse
	err := c.do(ctx, http.MethodGet, "/v1/loops/"+loopID, nil, &res)
	return &res, err
}

// ListLoops returns all loops
func (c *Client) ListLoops(ctx context.Context) ([]api.LoopWithRevision, error) {
	var res []api.LoopWithRevision
	err := c.do(ctx, http.MethodGet, "/v1/loops", nil, &res)
	return res, err
}

// TraceLoop returns the full trace of a loop
func (c *Client) TraceLoop(ctx context.Context, loopID string) (*api.LoopTraceResponse, error) {
	var res api.LoopTraceResponse
	err := c.do(ctx, http.MethodGet, "/v1/loops/"+loopID+"/trace", nil, &res)
	return &res, err
}

// GetJournal returns journal entries for a loop
func (c *Client) GetJournal(ctx context.Context, loopID string, limit int64) ([]api.JournalEntry, error) {
	var res []api.JournalEntry
	path := fmt.Sprintf("/v1/loops/%s/journal?limit=%d", loopID, limit)
	err := c.do(ctx, http.MethodGet, path, nil, &res)
	return res, err
}

// GetRuntime returns the runtime target for a loop
func (c *Client) GetRuntime(ctx context.Context, loopID string) (*api.LoopRuntimeResponse, error) {
	var res api.LoopRuntimeResponse
	err := c.do(ctx, http.MethodGet, "/v1/loops/"+loopID+"/runtime", nil, &res)
	return &res, err
}

// AttachTerminal attaches an actor to a loop's terminal
func (c *Client) AttachTerminal(ctx context.Context, loopID string, req api.TerminalAttachRequest) (map[string]any, error) {
	var res map[string]any
	err := c.do(ctx, http.MethodPost, "/v1/loops/"+loopID+"/control/attach", req, &res)
	return res, err
}

// DetachTerminal detaches an actor from a loop's terminal
func (c *Client) DetachTerminal(ctx context.Context, loopID string, req api.TerminalDetachRequest) (map[string]any, error) {
	var res map[string]any
	err := c.do(ctx, http.MethodPost, "/v1/loops/"+loopID+"/control/detach", req, &res)
	return res, err
}

// SendCommand sends a command to an attached terminal
func (c *Client) SendCommand(ctx context.Context, loopID string, req api.TerminalCommandRequest) (map[string]any, error) {
	var res map[string]any
	err := c.do(ctx, http.MethodPost, "/v1/loops/"+loopID+"/control/command", req, &res)
	return res, err
}

// SubmitPRD submits a PRD for processing
func (c *Client) SubmitPRD(ctx context.Context, req api.PRDIngressRequest) (*api.IngressSummary, error) {
	var res api.IngressSummary
	err := c.do(ctx, http.MethodPost, "/v1/ingress/prd", req, &res)
	return &res, err
}

// IngestGitHubIssues ingests GitHub issues
func (c *Client) IngestGitHubIssues(ctx context.Context, req api.GitHubIngressRequest) (*api.IngressSummary, error) {
	var res api.IngressSummary
	err := c.do(ctx, http.MethodPost, "/v1/ingress/github/issues", req, &res)
	return &res, err
}

// OverrideLoop overrides loop state
func (c *Client) OverrideLoop(ctx context.Context, req api.OverrideRequest) (*api.OverrideResponse, error) {
	var res api.OverrideResponse
	err := c.do(ctx, http.MethodPost, "/v1/control/override", req, &res)
	return &res, err
}

// Do executes a generic JSON request
func (c *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &APIError{
			Method:     method,
			Path:       path,
			StatusCode: resp.StatusCode,
			Body:       raw,
		}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	return c.Do(ctx, method, path, body, out)
}

func StructuredErrorBody(err error) ([]byte, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.JSONBody() {
		return nil, false
	}
	return bytes.TrimSpace(apiErr.Body), true
}
