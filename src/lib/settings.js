// How prompts are sent to the harness CLIs (the Settings panel), in
// localStorage `ah_settings` (per browser). Reads and writes never throw.
// Per harness: the models the prompt box offers (none: the harness's default
// model), each with its thinking effort; the effort for the default model;
// fast mode; bypass permissions.

import { HARNESS_IDS } from "./format.js"

const KEY = "ah_settings"
export const SETTINGS_EVENT = "ah:settings"

export function emptyHarness() {
  return { models: [], effort: "", fast: false, bypass: false }
}

/** Fills what a stored (or partial) settings object lacks. */
export function normalizeSettings(raw) {
  const src = raw && typeof raw === "object" ? raw : {}
  const harnesses = {}
  for (const id of HARNESS_IDS) {
    const h = src.harnesses?.[id] || {}
    harnesses[id] = {
      models: (Array.isArray(h.models) ? h.models : [])
        .filter((m) => m && typeof m.id === "string")
        .map((m) => ({ id: m.id, effort: typeof m.effort === "string" ? m.effort : "" })),
      effort: typeof h.effort === "string" ? h.effort : "",
      fast: h.fast === true,
      bypass: h.bypass === true,
    }
  }
  const defaultHarness = HARNESS_IDS.includes(src.defaultHarness) ? src.defaultHarness : "claude"
  return { defaultHarness, harnesses }
}

export function readSettings() {
  try {
    return normalizeSettings(JSON.parse(localStorage.getItem(KEY) || "{}"))
  } catch {
    return normalizeSettings({})
  }
}

/** Saves and tells every mounted reader (useSettings) about it. */
export function writeSettings(settings) {
  try {
    localStorage.setItem(KEY, JSON.stringify(settings))
  } catch {
    // ignore: the settings last until the page reloads
  }
  window.dispatchEvent(new CustomEvent(SETTINGS_EVENT, { detail: settings }))
}

/** The configured models with an id ("" rows are being typed). */
export function configuredModels(settings, harness) {
  return (settings.harnesses[harness]?.models || []).filter((m) => m.id.trim())
}

/**
 * The run request for a prompt: the model's effort (or the harness's effort
 * for its default model), fast mode and bypass from the settings.
 */
export function launchRequest(settings, { harness, model = "", prompt, session = "", cwd = "" }) {
  const h = settings.harnesses[harness] || emptyHarness()
  const row = model ? h.models.find((m) => m.id.trim() === model) : null
  return {
    harness,
    session,
    cwd,
    prompt,
    model,
    effort: row ? row.effort : h.effort,
    fast: h.fast,
    bypass: h.bypass,
  }
}

/** The efforts to offer for a model: what it supports when known, else the harness's. */
export function effortsFor(harnessInfo, modelInfo) {
  if (modelInfo && Array.isArray(modelInfo.efforts)) return modelInfo.efforts
  return harnessInfo?.efforts || []
}
