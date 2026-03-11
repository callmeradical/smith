import { appState } from './stores';
import { get } from 'svelte/store';

const config = (typeof window !== 'undefined' ? (window as any).__SMITH_CONFIG__ : {}) || {};
export const apiBaseUrl = (config.apiBaseUrl || "/api").replace(/\/+$/, "");
export const chatBaseUrl = (config.chatBaseUrl || "/chat").replace(/\/+$/, "");

export async function fetchWithTimeout(url: string, options: any = {}, timeoutMs = 20000, label?: string) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } catch (err: any) {
    if (err && err.name === "AbortError") {
      throw new Error(`Request timed out after ${Math.ceil(timeoutMs / 1000)}s for ${label || url}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

export async function fetchJSON(path: string) {
  const res = await fetchWithTimeout(`${apiBaseUrl}${path}`, {
    headers: { Accept: "application/json" }
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} for ${path}`);
  }
  return res.json();
}

export async function postJSON(path: string, payload: any) {
  return requestJSON(path, "POST", payload);
}

export async function getJSON(path: string) {
  return requestJSON(path, "GET");
}

export async function deleteJSON(path: string) {
  return requestJSON(path, "DELETE");
}

export async function requestJSON(path: string, method: string, payload?: any) {
  const res = await fetchWithTimeout(`${apiBaseUrl}${path}`, {
    method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = body.error || `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body;
}
