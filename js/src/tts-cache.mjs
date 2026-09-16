import { createHash } from 'node:crypto';

export const KEY_VERSION = 'v1';

export const AUDIO_SOURCES = Object.freeze(['memory', 'disk', 'proxy', 'direct', 'system']);

export function normalize(text) {
  return String(text ?? '')
    .trim()
    .replace(/\s+/g, ' ')
    .replaceAll(' .', '.')
    .replaceAll(' ,', ',')
    .toLowerCase();
}

export function speedKey(speed) {
  const value = Number(speed);
  if (!Number.isFinite(value) || value === 0) return '1';
  return value.toFixed(3);
}

function firstDefined(...values) {
  return values.find((value) => value !== undefined && value !== null) ?? '';
}

export function cacheKey(params = {}) {
  const parts = [
    KEY_VERSION,
    normalize(firstDefined(params.text)),
    String(firstDefined(params.voiceId, params.voiceID)).trim(),
    String(firstDefined(params.modelId, params.modelID)).trim(),
    speedKey(firstDefined(params.speed, 0)),
    String(firstDefined(params.format, params.outputFormat)).trim(),
    String(firstDefined(params.language, params.lang)).trim(),
  ];
  const hex = createHash('sha256').update(parts.join('|'), 'utf8').digest('hex');
  return `${KEY_VERSION}-${hex.slice(0, 16)}`;
}

export function splitSentences(text) {
  const source = String(text ?? '');
  const end = /[.!?]+\s*/g;
  const out = [];
  let start = 0;
  let match;
  while ((match = end.exec(source)) !== null) {
    const part = source.slice(start, match.index + match[0].length).trim();
    if (part !== '') out.push(part);
    start = match.index + match[0].length;
  }
  const rest = source.slice(start).trim();
  if (rest !== '') out.push(rest);
  if (out.length === 0 && source.trim() !== '') out.push(source.trim());
  return out;
}

export function buildRequest(text, options = {}) {
  return {
    text: String(text ?? ''),
    voice_id: String(firstDefined(options.voiceId, options.voiceID)),
    model_id: String(firstDefined(options.modelId, options.modelID)),
    speed: Number(firstDefined(options.speed, 1)),
    format: String(firstDefined(options.format, options.outputFormat)),
    language: String(firstDefined(options.language, options.lang)),
  };
}

function stripTrailingSlash(value) {
  return String(value ?? '').replace(/\/+$/, '');
}

function parseStatuses(headers) {
  const raw = headers?.get?.('x-sentence-statuses');
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function parseSentenceCount(headers) {
  const count = Number.parseInt(headers?.get?.('x-sentence-count') ?? '', 10);
  return Number.isInteger(count) ? count : 0;
}

export class Client {
  constructor({ endpoints = [], appToken = '', fetchImpl = globalThis.fetch } = {}) {
    this.endpoints = [...endpoints].map(stripTrailingSlash).filter((endpoint) => endpoint !== '');
    this.appToken = appToken;
    this.fetchImpl = fetchImpl;
    this.latencies = new Map();
    this.memory = new Map();
  }

  latencyOf(endpoint) {
    return this.latencies.get(stripTrailingSlash(endpoint));
  }

  async measureLatency() {
    const results = [];
    for (const endpoint of this.endpoints) {
      const started = Date.now();
      try {
        const res = await this.fetchImpl(`${endpoint}/healthz`, { method: 'GET' });
        if (!res.ok) throw new Error(`healthz status ${res.status}`);
        const latencyMs = Date.now() - started;
        this.latencies.set(endpoint, latencyMs);
        results.push({ endpoint, latencyMs, ok: true });
      } catch (error) {
        this.latencies.set(endpoint, Number.POSITIVE_INFINITY);
        results.push({
          endpoint,
          latencyMs: Number.POSITIVE_INFINITY,
          ok: false,
          error: error?.message ?? String(error),
        });
      }
    }
    results.sort((a, b) => {
      const rank = (row) => (row.ok ? row.latencyMs : Number.POSITIVE_INFINITY);
      return rank(a) - rank(b);
    });
    this.endpoints = results.map((row) => row.endpoint);
    return results;
  }

  async synthesize(text, options = {}) {
    const key = cacheKey({ text, ...options });
    const hit = this.memory.get(key);
    if (hit) {
      return {
        audio: hit.audio.slice(0),
        source: 'memory',
        statuses: hit.statuses,
        sentenceCount: hit.sentenceCount,
        key,
        endpoint: null,
        region: '',
      };
    }
    if (this.endpoints.length === 0) throw new Error('no synthesize endpoints configured');
    const failures = [];
    for (const endpoint of this.endpoints) {
      const started = Date.now();
      try {
        const res = await this.fetchImpl(`${endpoint}/v1/synthesize`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${this.appToken}`,
          },
          body: JSON.stringify(buildRequest(text, options)),
        });
        const latencyMs = Date.now() - started;
        if (res.status === 200 || res.status === 206) {
          const audio = await res.arrayBuffer();
          const statuses = parseStatuses(res.headers);
          const sentenceCount = parseSentenceCount(res.headers);
          this.memory.set(key, { audio: audio.slice(0), statuses, sentenceCount });
          return {
            audio,
            source: 'proxy',
            statuses,
            sentenceCount,
            key,
            endpoint,
            region: res.headers?.get?.('x-region') ?? '',
            latencyMs,
          };
        }
        failures.push({ endpoint, code: res.status, latencyMs });
      } catch (error) {
        failures.push({
          endpoint,
          code: 0,
          latencyMs: Date.now() - started,
          error: error?.message ?? String(error),
        });
      }
    }
    const detail = failures.map((entry) => `${entry.endpoint} code=${entry.code}`).join('; ');
    const error = new Error(`all synthesize endpoints failed: ${detail}`);
    error.failures = failures;
    throw error;
  }
}
