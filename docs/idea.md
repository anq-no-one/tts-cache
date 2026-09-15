# Idea

TTS APIs (ElevenLabs, Fish Audio) bill per character. Apps with
repetitive spoken phrases, like fitness coach cues, pay to synthesize
the same sentences over and over.

`tts-cache` is a self-hosted caching proxy. The app sends synthesis
requests to it instead of the provider. The proxy splits text into
sentences, serves repeats from disk, synthesizes each new sentence
once, and concatenates the audio back in order.

Goals:

- Cut TTS spend on repeated phrases to near zero.
- Keep the voice quality identical to direct synthesis.
- Stay deployable anywhere with one container and one volume.
- Work for any client over plain HTTP. Native SDKs are optional.
