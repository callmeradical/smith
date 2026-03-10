export const eyeIconMarkup =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6z"></path><circle cx="12" cy="12" r="2.7"></circle></svg>';
export const eyeOffIconMarkup =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 3l18 18"></path><path d="M10.7 6.3A10.8 10.8 0 0 1 12 6c6.5 0 10 6 10 6a19 19 0 0 1-4.2 4.6"></path><path d="M6.1 6.9C3.7 8.6 2 12 2 12s3.5 6 10 6c1.6 0 3-.3 4.3-.8"></path><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"></path></svg>';

export const providerCatalog = [
  {
    id: "anthropic",
    label: "Anthropic",
    subtitle: "Claude and other models from Anthropic.",
  },
  {
    id: "amazon_bedrock",
    label: "Amazon Bedrock",
    subtitle: "Run models through Amazon Bedrock.",
  },
  {
    id: "azure_openai",
    label: "Azure OpenAI",
    subtitle: "Models through Azure OpenAI Service.",
  },
  {
    id: "claude_code_cli",
    label: "Claude Code CLI",
    subtitle: "Execute Claude models via Claude Code CLI.",
  },
  {
    id: "codex",
    label: "OpenAI Codex CLI",
    subtitle: "Execute OpenAI models via Codex CLI.",
  },
  {
    id: "cursor_agent",
    label: "Cursor Agent",
    subtitle: "Execute AI models via cursor-agent CLI.",
  },
  {
    id: "deepseek",
    label: "DeepSeek",
    subtitle: "Custom DeepSeek provider.",
  },
  {
    id: "databricks",
    label: "Databricks",
    subtitle: "Models on Databricks AI Gateway.",
  },
  {
    id: "gcp_vertex_ai",
    label: "GCP Vertex AI",
    subtitle: "Access models through Vertex AI.",
  },
  {
    id: "gemini_cli",
    label: "Gemini CLI",
    subtitle: "Execute Gemini models via CLI.",
  },
  {
    id: "github_copilot",
    label: "GitHub Copilot",
    subtitle: "Run and configure GitHub Copilot.",
  },
  {
    id: "google_gemini",
    label: "Google Gemini",
    subtitle: "Gemini models from Google AI.",
  },
];

export const projectStorageKey = "smith.console.projects.v1";
export const podViewTerminalStates = new Set([
  "idle",
  "attaching",
  "attached",
  "executing",
  "detaching",
  "error",
]);
