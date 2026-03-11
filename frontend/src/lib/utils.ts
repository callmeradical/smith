export function escapeHtml(v: any) {
  const map: any = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  };
  return String(v).replace(/[&<>"']/g, (m) => map[m]);
}

export function slugifySegment(v: string) {
  return String(v).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

export function normalizeBranchName(v: string) {
  return String(v).replace(/[^a-zA-Z0-9._/-]/g, "-");
}
