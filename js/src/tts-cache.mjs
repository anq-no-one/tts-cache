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

function textField(value) {
  return String(value ?? '').trim();
}

export function cacheKey({ text = '', voiceId = '', modelId = '', speed, format = '', language = '' } = {}) {
  const parts = [
    KEY_VERSION,
    normalize(text),
    textField(voiceId),
    textField(modelId),
    speedKey(speed),
    textField(format),
    textField(language),
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

export function buildRequest(text, { voiceId = '', modelId = '', speed = 1, format = '', language = '' } = {}) {
  return {
    text: String(text ?? ''),
    voice_id: String(voiceId ?? ''),
    model_id: String(modelId ?? ''),
    speed: Number(speed ?? 1),
    format: String(format ?? ''),
    language: String(language ?? ''),
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

function isRetryableStatus(status) {
  return status === 429 || status >= 500;
}

function toAudioBuffer(value) {
  if (value instanceof ArrayBuffer) return value;
  if (ArrayBuffer.isView(value)) return value.buffer.slice(value.byteOffset, value.byteOffset + value.byteLength);
  throw new TypeError('directFetch must resolve to an ArrayBuffer or a typed array');
}

export class Client {
  constructor({ endpoints = [], appToken = '', fetchImpl = globalThis.fetch, directFetch = null } = {}) {
    this.endpoints = [...endpoints].map(stripTrailingSlash).filter((endpoint) => endpoint !== '');
    this.appToken = appToken;
    this.fetchImpl = fetchImpl;
    this.directFetch = directFetch;
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
    const request = buildRequest(text, options);
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
          body: JSON.stringify(request),
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
        const failure = { endpoint, code: res.status, latencyMs };
        if (!isRetryableStatus(res.status)) {
          return {
            audio: new ArrayBuffer(0),
            source: 'proxy',
            statuses: parseStatuses(res.headers),
            sentenceCount: parseSentenceCount(res.headers),
            key,
            endpoint,
            region: res.headers?.get?.('x-region') ?? '',
            latencyMs,
            failures: [failure],
          };
        }
        failures.push(failure);
      } catch (error) {
        failures.push({
          endpoint,
          code: 0,
          latencyMs: Date.now() - started,
          error: error?.message ?? String(error),
        });
      }
    }
    const directAudio = this.directFetch
      ? await Promise.resolve()
        .then(() => this.directFetch(String(text ?? '')))
        .then(toAudioBuffer)
        .catch(() => null)
      : null;
    if (directAudio !== null) {
      this.memory.set(key, { audio: directAudio.slice(0), statuses: [], sentenceCount: 0 });
      return {
        audio: directAudio,
        source: 'direct',
        statuses: [],
        sentenceCount: 0,
        key,
        endpoint: null,
        region: '',
        failures,
      };
    }
    return {
      audio: new ArrayBuffer(0),
      source: 'system',
      statuses: [],
      sentenceCount: 0,
      key,
      endpoint: null,
      region: '',
      failures,
    };
  }
}
