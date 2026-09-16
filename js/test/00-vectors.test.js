import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { normalize } from '../src/tts-cache.mjs';

const vectors = JSON.parse(readFileSync(new URL('../../vectors/vectors.json', import.meta.url), 'utf8'));

describe('shared vectors parity', () => {
  for (const [index, vector] of vectors.entries()) {
    it(`vector ${index}: ${JSON.stringify(vector.input)}`, () => {
      assert.equal(normalize(vector.input), vector.normalized);
    });
  }

  it('vectors file is non-empty', () => {
    assert.ok(vectors.length > 0);
  });
});
