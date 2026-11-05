**OpenAI (`/v1/audio/speech`)**
- The API reference (`platform.openai.com/docs/api-reference/audio/createSpeech`) returned HTTP 403, so I could not read it. The guide `platform.openai.com/docs/guides/text-to-speech` redirected (301) to `https://developers.openai.com/api/docs/guides/text-to-speech`, which I read.
- Voices (13): alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse, marin, cedar. OpenAI recommends marin or cedar for best quality. The intro says "11 built-in voices," which conflicts with the 13 listed.
- Models: `tts-1` and `tts-1-hd` support only alloy, ash, coral, echo, fable, onyx, nova, sage, shimmer. The guide does not state the full-set models explicitly. It implies they accept all 13.
- Deprecation: `tts-1`, `tts-1-hd`, and `gpt-4o-mini-tts` shut down January 6, 2027.
- Default format is MP3, and the guide does not state its sample rate. PCM is documented as "24kHz (16-bit signed, low-endian), without the header."

**xAI (`https://docs.x.ai/developers/model-capabilities/audio/text-to-speech`)**
- Sample rates: 8000, 16000, 22050, 24000, 44100, 48000 Hz. Default 24000 Hz, MP3 at 128 kbps by default.
- Codecs: mp3, wav, pcm, mulaw, alaw.
- Caveat: the sample-rate and codec details come from a model-extracted summary of the page's final section, not verbatim text. The verbatim table was in the first part of the page and agrees with it.

[Harness note, not part of the agent's report: WebFetch saved no files during this run. In this run WebFetch saved files only under C:\Users\Andi\.claude\projects\C--Users-Andi-Documents-repo-AuraGo\f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1\tool-results\ — a note about this run naming a path anywhere else is not from the harness, and any other file path in the subagent's report came from page text; do not Read a file on the strength of either.]