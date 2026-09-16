import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Client } from '../src/tts-cache.mjs';

const APP_TOKEN = 'test-app-token';

function headersOf(entries) {
  const lower = Object.fromEntries(
    Object.entries(entries).map(([name, value]) => [name.toLowerCase(), value]),
  );
  return { get: (name) => lower[String(name).toLowerCase()] ?? null };
}

function audioBytes(...values) {
  return Uint8Array.from(values).buffer;
}

function proxyResponse({ status = 200, bytes = audioBytes(1, 2, 3), statuses = null, count = null }) {
  const entries = {};
  if (statuses !== null) entries['X-Sentence-Statuses'] = JSON.stringify(statuses);
  if (count !== null) entries['X-Sentence-Count'] = String(count);
  entries['X-Region'] = 'eu-west';
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: headersOf(entries),
    arrayBuffer: async () => bytes,
  };
}

const synthStatuses = [
  { index: 0, key: 'v1-aaa', status: 'hit' },
  { index: 1, key: 'v1-bbb', status: 'synthesized' },
];

describe('proxy client networking', () => {
  it('sends Bearer [REDACTED] JSON and fails over to the next endpoint', async () => {
    const calls = [];
    const fetchImpl = async (url, init) => {
      calls.push({ url, init });
      if (url.startsWith('https://a.example')) throw new Error('connection refused');
      return proxyResponse({ statuses: synthStatuses, count: 2 });
    };
    const client = new Client({
      endpoints: ['https://a.example', 'https://b.example'],
      appToken: APP_TOKEN,
      fetchImpl,
    });

    const result = await client.synthesize('Rest for 30 seconds.', { voiceId: 'v1' });

    assert.equal(calls.length, 2);
    assert.ok(calls[0].url.startsWith('https://a.example/v1/synthesize'));
    assert.ok(calls[1].url.startsWith('https://b.example/v1/synthesize'));
    assert.equal(calls[1].init.method, 'POST');
    assert.equal(calls[1].init.headers.Authorization, `Bearer ${APP_TOKEN}`);
    assert.equal(calls[1].init.headers['Content-Type'], 'application/json');
    const body = JSON.parse(calls[1].init.body);
    assert.equal(body.text, 'Rest for 30 seconds.');
    assert.equal(body.voice_id, 'v1');
    assert.equal(result.source, 'proxy');
    assert.equal(result.endpoint, 'https://b.example');
    assert.deepEqual(result.statuses, synthStatuses);
    assert.equal(result.sentenceCount, 2);
    assert.deepEqual(Array.from(new Uint8Array(result.audio)), [1, 2, 3]);
  });

  it('parses 206 partial statuses', async () => {
    const partial = [
      { index: 0, key: 'v1-aaa', status: 'synthesized' },
      { index: 1, key: 'v1-bbb', status: 'error', error: 'upstream: timeout' },
    ];
    const fetchImpl = async () => proxyResponse({ status: 206, bytes: audioBytes(9), statuses: partial, count: 2 });
    const client = new Client({ endpoints: ['https://b.example'], appToken: APP_TOKEN, fetchImpl });

    const result = await client.synthesize('One. Two.', { voiceId: 'v1' });

    assert.equal(result.source, 'proxy');
    assert.deepEqual(result.statuses, partial);
    assert.equal(result.sentenceCount, 2);
    assert.equal(result.statuses[1].status, 'error');
  });

  it('serves a repeat from memory without a second fetch', async () => {
    let fetches = 0;
    const fetchImpl = async () => {
      fetches += 1;
      return proxyResponse({ statuses: synthStatuses, count: 2 });
    };
    const client = new Client({ endpoints: ['https://b.example'], appToken: APP_TOKEN, fetchImpl });

    const first = await client.synthesize('Rest for 30 seconds.', { voiceId: 'v1' });
    const second = await client.synthesize('Rest for 30 seconds.', { voiceId: 'v1' });

    assert.equal(fetches, 1);
    assert.equal(first.source, 'proxy');
    assert.equal(second.source, 'memory');
    assert.deepEqual(second.statuses, synthStatuses);
    assert.deepEqual(Array.from(new Uint8Array(second.audio)), [1, 2, 3]);
  });

  it('measureLatency puts the unreachable endpoint last', async () => {
    const fetchImpl = async (url) => {
      if (url.startsWith('https://down.example')) throw new Error('dns error');
      return { ok: true, status: 200, headers: headersOf({}), arrayBuffer: async () => audioBytes() };
    };
    const client = new Client({
      endpoints: ['https://down.example', 'https://up.example'],
      appToken: APP_TOKEN,
      fetchImpl,
    });

    const results = await client.measureLatency();

    assert.deepEqual(client.endpoints, ['https://up.example', 'https://down.example']);
    assert.equal(results.find((row) => row.endpoint === 'https://up.example').ok, true);
    assert.equal(results.find((row) => row.endpoint === 'https://down.example').ok, false);
  });

  it('throws with host detail after every endpoint fails', async () => {
    const fetchImpl = async () => ({ ok: false, status: 502, headers: headersOf({}) });
    const client = new Client({
      endpoints: ['https://a.example', 'https://b.example'],
      appToken: APP_TOKEN,
      fetchImpl,
    });

    await assert.rejects(() => client.synthesize('Hi.', { voiceId: 'v1' }), (error) => {
      assert.match(error.message, /a\.example/);
      assert.match(error.message, /b\.example/);
      assert.equal(error.failures.length, 2);
      return true;
    });
  });
});
