import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { normalize, speedKey, cacheKey, splitSentences, buildRequest } from '../src/tts-cache.mjs';

function goldenParams() {
  return {
    text: 'Rest for 30 seconds.',
    voiceId: 'v1',
    modelId: 'fish-audio/s2.1-pro',
    speed: 1.08,
    format: 'mp3_44100_128',
    language: 'en',
  };
}

describe('golden key and pure helpers', () => {
  it('matches the proxy golden key', () => {
    assert.equal(cacheKey(goldenParams()), 'v1-feab4a17bc7070e8');
  });

  it('uses canonical fields only, aliases are ignored', () => {
    assert.equal(
      cacheKey({ ...goldenParams(), voiceID: 'other', modelID: 'other', outputFormat: 'other', lang: 'other' }),
      'v1-feab4a17bc7070e8',
    );
  });

  it('is deterministic for identical params', () => {
    assert.equal(cacheKey(goldenParams()), cacheKey(goldenParams()));
  });

  it('changes when sound fields change', () => {
    const base = cacheKey(goldenParams());
    assert.notEqual(base, cacheKey({ ...goldenParams(), speed: 1.5 }));
    assert.notEqual(base, cacheKey({ ...goldenParams(), modelId: 'other-model' }));
    assert.notEqual(base, cacheKey({ ...goldenParams(), text: 'Different sentence.' }));
  });

  it('collapses noise like Go Normalize', () => {
    assert.equal(normalize('  Rest   for 30 seconds . '), normalize('rest for 30 seconds.'));
  });

  it('formats speed with three decimals, zero means one', () => {
    assert.equal(speedKey(0), '1');
    assert.equal(speedKey(undefined), '1');
    assert.equal(speedKey(1.08), '1.080');
    assert.equal(speedKey(1), '1.000');
  });

  it('splits sentences on end punctuation like the server', () => {
    assert.deepEqual(splitSentences('Hello world. How are you?'), ['Hello world.', 'How are you?']);
    assert.deepEqual(splitSentences('  One.   Two!Three?  four'), ['One.', 'Two!', 'Three?', 'four']);
    assert.deepEqual(splitSentences('no punctuation here'), ['no punctuation here']);
    assert.deepEqual(splitSentences('   '), []);
  });

  it('builds the POST JSON body shape', () => {
    assert.deepEqual(buildRequest('Hi there.', { voiceId: 'v1', speed: 1.08, language: 'en' }), {
      text: 'Hi there.',
      voice_id: 'v1',
      model_id: '',
      speed: 1.08,
      format: '',
      language: 'en',
    });
  });

  it('ignores field aliases when building the POST JSON body', () => {
    const body = buildRequest('Hi there.', { voiceId: 'v1', voiceID: 'other', outputFormat: 'other', lang: 'other' });
    assert.equal(body.voice_id, 'v1');
    assert.equal(body.format, '');
    assert.equal(body.language, '');
  });
});
