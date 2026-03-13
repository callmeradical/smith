import { apiBase, operatorToken, requestTimeoutMs } from "./state.js";

export async function fetchWithTimeout(url, options, timeoutMs, label) {
  console.log('API: Fetching', url);
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...(options || {}), signal: controller.signal });
  } catch (err) {
    if (err && err.name === "AbortError") {
      throw new Error(
        "Request timed out after " +
          String(Math.ceil(timeoutMs / 1000)) +
          "s for " +
          String(label || url),
      );
    }
    throw err;
  } finally {
    window.clearTimeout(timer);
  }
}

export async function fetchJSON(path) {
  const headers = { Accept: "application/json" };
  if (operatorToken) headers.Authorization = "Bearer " + operatorToken;
  
  const fullPath = apiBase ? apiBase + path : path;
  
  const res = await fetchWithTimeout(
    fullPath,
    { headers },
    requestTimeoutMs,
    path,
  );
  if (!res.ok) {
    throw new Error("HTTP " + res.status + " for " + path);
  }
  return res.json();
}

export async function postJSON(path, payload) {
  return requestJSON(path, "POST", payload);
}

export async function getJSON(path) {
  return requestJSON(path, "GET");
}

export async function deleteJSON(path, payload) {
  return requestJSON(path, "DELETE", payload);
}

export async function requestJSON(path, method, payload) {
  const headers = {
    Accept: "application/json",
    "Content-Type": "application/json",
  };
  if (operatorToken) headers.Authorization = "Bearer " + operatorToken;
  
  const fullPath = apiBase ? apiBase + path : path;

  const res = await fetchWithTimeout(
    fullPath,
    {
      method,
      headers,
      body: payload === undefined ? undefined : JSON.stringify(payload),
    },
    requestTimeoutMs,
    path,
  );
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = body.error || "HTTP " + res.status;
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return body;
}
