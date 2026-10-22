# Cloud speech output

Select a provider in **Configuration → Speech Output**, save its API key with
**Save key**, then save the provider, voice and model settings. Credentials are
kept in the encrypted Vault and are never written to `config.yaml` or exported
to agent Python tools. The local sanoTTS installation default remains available.

| Provider ID | Default model | Default voice | API documentation |
| --- | --- | --- | --- |
| `xai` | Selected by the service | `eve` | [xAI TTS](https://docs.x.ai/developers/model-capabilities/audio/text-to-speech) |
| `openai` | `gpt-4o-mini-tts` | `coral` | [OpenAI TTS](https://developers.openai.com/api/docs/guides/text-to-speech) |
| `stepfun` | `step-tts-mini` | `jingdiannvsheng` | [StepFun example](https://huggingface.co/spaces/stepfun-ai/Step-Audio/blob/main/yuewen_api.py) |
| `resemble` | Selected by the account voice | Required account voice UUID | [Resemble synchronous synthesis](https://docs.resemble.ai/api-reference/text-to-speech/synthesize) |

Use the custom option for account-specific voices or model IDs. Choose a voice
supported by the selected model. All four adapters request MP3 and share the
existing chat, agent tools, Bluetooth, Chromecast, Personal Radio, AgoDesk and
classic SIP output paths. Credentials are specific to speech output; LLM provider
keys are not reused implicitly.

OpenAI documents shutdown of its Speech API models on **January 6, 2027**.
Its recommended replacement uses the separate [Realtime API](https://developers.openai.com/api/docs/guides/realtime).
The OpenAI entry here uses the Speech API until that shutdown.

Resemble describes its hosted voice APIs as a [historical archive](https://docs.resemble.ai/voice-archive),
which does not guarantee continued commercial availability. This adapter can only
work with access that still accepts synthesis requests. For a local alternative,
consider its open Chatterbox models through an appropriate Speech Lab runtime.
