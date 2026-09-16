import { Client } from '../js/src/tts-cache.mjs';

const endpoints = (process.env.ENDPOINTS ?? 'http://localhost:8080,http://localhost:8081').split(',');
const appToken = process.env.APP_TOKEN ?? '';

const client = new Client({
  endpoints,
  appToken,
  fetchImpl: globalThis.fetch,
  directFetch: async () => {
    throw new Error('direct provider not configured');
  },
});

await client.measureLatency();
const result = await client.synthesize('Hello. World.', { voiceId: 'test-voice' });
console.log(result.source);
console.log(JSON.stringify({ endpoint: result.endpoint, region: result.region, sentenceCount: result.sentenceCount }));
