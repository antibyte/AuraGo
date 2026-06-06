# AgoDesk Coding Agent: Mood Metadata And Plan Updates

AuraGo can send the agent's current mood as chat metadata and the active chat plan as live protocol updates. Implement this in AgoDesk by extending the existing WebSocket chat handling; do not add a second HTTP polling path for plans.

## Contract

AgoDesk should advertise these client capabilities in `session.start.payload.client_capabilities` when the local client supports them:

```json
[
  "chat.full_response",
  "chat.agent_metadata",
  "chat.plan_updates"
]
```

When `chat.agent_metadata` is negotiated, AuraGo may include `payload.metadata.agent_mood` on `chat.response` and on future `chat.response.chunk` frames:

```json
{
  "type": "chat.response",
  "payload": {
    "session_id": "agodesk:device-123",
    "request_id": "req-1",
    "text": "Done.",
    "role": "assistant",
    "metadata": {
      "source": "agodesk_chat",
      "agent_mood": {
        "mood": "focused",
        "primary_mood": "focused",
        "secondary_mood": "steady",
        "description": "I feel calm and ready to help.",
        "valence": 0.2,
        "arousal": 0.3,
        "confidence": 0.8,
        "recommended_response_style": "calm_and_precise",
        "source": "emotion_history",
        "timestamp": "2026-06-06 12:34:56"
      }
    }
  }
}
```

When `chat.plan_updates` is negotiated, AuraGo may send `chat.plan_update` frames during a chat turn:

```json
{
  "type": "chat.plan_update",
  "payload": {
    "session_id": "agodesk:device-123",
    "request_id": "req-1",
    "plan": {
      "id": "plan_123",
      "title": "Fix homepage preview",
      "status": "active",
      "tasks": [],
      "task_counts": {
        "total": 3,
        "pending": 1,
        "in_progress": 1,
        "completed": 1
      },
      "progress_pct": 33,
      "current_task": "Verify the preview URL"
    }
  }
}
```

`payload.plan` uses the same JSON shape as AuraGo's web chat plan panel. If `payload.plan` is `null`, clear the local plan panel for that session.

The final `chat.response.payload.metadata.plan` may contain the latest plan snapshot for clients that want to reconcile their display after the turn completes.

## Required AgoDesk Changes

1. Add `chat.agent_metadata` and `chat.plan_updates` to the capabilities sent in `session.start`.
2. Store the returned `session.accepted.advertised_capabilities` and enable the UI paths only when the corresponding capability is present.
3. Extend the local `chat.response` and `chat.response.chunk` payload models so `metadata.agent_mood` is optional and ignored when absent.
4. Pass `agent_mood` to the voice model before speech synthesis for the matching assistant response.
5. Map `mood` or `primary_mood` to the primary voice style. Use `valence`, `arousal`, and `confidence` only when the configured voice backend supports them.
6. Use `recommended_response_style` as a short style hint. Never display or speak internal metadata directly unless the user opens a debug view.
7. Add a handler for `chat.plan_update`. Store the latest plan per `session_id`, keyed with `request_id` when present.
8. Render plan title, task list, task status, `task_counts`, `progress_pct`, and `current_task` in the same place where AgoDesk shows chat planning state.
9. If `metadata.plan` arrives on the final `chat.response`, reconcile it with the currently displayed plan snapshot.
10. Treat unknown metadata fields and unknown plan fields as forward-compatible. They must not crash the client.

## Suggested Data Shape

```ts
type AgentMoodMetadata = {
  mood?: string;
  primary_mood?: string;
  secondary_mood?: string;
  description?: string;
  valence?: number;
  arousal?: number;
  confidence?: number;
  recommended_response_style?: string;
  source?: string;
  timestamp?: string;
};

type ChatResponseMetadata = {
  source?: string;
  server_push?: boolean;
  agent_mood?: AgentMoodMetadata;
  plan?: AgoDeskPlan | null;
  [key: string]: unknown;
};

type ChatPlanUpdatePayload = {
  session_id: string;
  request_id?: string;
  plan: AgoDeskPlan | null;
};
```

`AgoDeskPlan` should be permissive. Require only the fields needed for rendering and keep unknown fields for debugging if the local state store supports that.

## Voice Model Guidance

- Prefer a small deterministic mapping from AuraGo moods to local voice styles.
- `focused`, `analytical`, and `cautious` should sound precise and calm.
- `curious`, `creative`, and `playful` may sound warmer and more animated.
- `concerned` and `frustrated` should reduce playfulness and increase clarity.
- Clamp numeric mood parameters before sending them to the voice provider.
- If the provider rejects style parameters, retry the same text without mood parameters instead of dropping the spoken response.

## Acceptance Criteria

- AgoDesk advertises both new capabilities after session start.
- Older AuraGo servers without these capabilities still work.
- `agent_mood` reaches the voice-model request for assistant responses.
- `chat.plan_update` changes the visible plan panel without waiting for the final response.
- `plan: null` clears the visible plan panel.
- Final `metadata.plan` reconciles the displayed plan snapshot.
- Unknown metadata or plan fields are ignored safely.
