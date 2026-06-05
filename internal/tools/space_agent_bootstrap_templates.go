package tools

import (
	"strconv"
)

func spaceAgentDockerfile() string {
	return `FROM node:22-bookworm-slim
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates openssh-client \
    && rm -rf /var/lib/apt/lists/*
COPY package*.json ./
RUN npm ci --omit=dev || npm install --omit=dev
COPY . .
EXPOSE 3000 3100
CMD ["sh", "-lc", "node aurago_space_bootstrap.mjs && node space supervise --state-dir /app/supervisor HOST=${HOST:-0.0.0.0} PORT=${PORT:-3000}"]
`
}

func spaceAgentInstructionsAPIEndpoint() string {
	return `import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";

export const allowAnonymous = true;

function readPayload(context) {
  return context.body && typeof context.body === "object" && !Buffer.isBuffer(context.body)
    ? context.body
    : {};
}

function normalizeSegment(value, fallback) {
  const raw = String(value || fallback || "").trim().replaceAll("\\", "/");
  if (!raw || raw.includes("/")) {
    return fallback;
  }
  const normalized = path.posix.normalize(raw);
  if (!normalized || normalized === "." || normalized === ".." || normalized.includes("/")) {
    return fallback;
  }
  return normalized;
}

function unauthorized() {
  return {
    status: 401,
    body: {
      status: "error",
      error: "Unauthorized"
    }
  };
}

async function appendOnscreenAgentHistory(userRoot, record) {
  const historyDir = path.join(userRoot, "hist");
  const historyPath = path.join(historyDir, "onscreen-agent.json");
  let history = [];
  try {
    const rawHistory = await fs.readFile(historyPath, "utf8");
    const parsed = JSON.parse(rawHistory);
    history = Array.isArray(parsed) ? parsed : [];
  } catch {
    history = [];
  }
  history.push({
    attachments: [],
    content: String(record.message || record.instruction || "").trim(),
    id: "user-" + Date.now() + "-aurago-" + String(record.message_id || "").trim(),
    kind: "aurago-instruction",
    role: "user"
  });
  await fs.mkdir(historyDir, { recursive: true, mode: 0o700 });
  await fs.writeFile(historyPath, JSON.stringify(history, null, 2) + "\n", {
    mode: 0o600
  });
  return historyPath;
}

export async function post(context) {
  if (String(context.headers?.["x-aurago-instruction"] || "").trim() !== "1") {
    return {
      status: 400,
      body: {
        status: "error",
        error: "X-AuraGo-Instruction header is required"
      }
    };
  }

  const expectedToken = String(process.env.AURAGO_BRIDGE_TOKEN || "").trim();
  const authHeader = String(context.headers?.authorization || context.headers?.Authorization || "").trim();
  if (!expectedToken || authHeader !== "Bearer " + expectedToken) {
    return unauthorized();
  }

  const payload = readPayload(context);
  const instruction = String(payload.instruction || "").trim();
  const information = String(payload.information || "").trim();
  const sessionId = String(payload.session_id || "").trim();
  if (!instruction) {
    return {
      status: 400,
      body: {
        status: "error",
        error: "instruction is required"
      }
    };
  }

  const username = normalizeSegment(process.env.SPACE_AGENT_ADMIN_USER, "admin");
  const projectRoot = String(context.projectRoot || process.cwd());
  const customwareRoot = String(process.env.CUSTOMWARE_PATH || path.join(projectRoot, "customware"));
  const userRoot = path.join(customwareRoot, "L2", username);
  const inboxDir = path.join(userRoot, "aurago_inbox");
  const message = information ? instruction + "\n\nContext from AuraGo:\n" + information : instruction;
  const messageId = crypto.randomUUID();
  const record = {
    type: "aurago_instruction",
    instruction,
    information,
    message,
    session_id: sessionId,
    source: "aurago",
    created_at: new Date().toISOString(),
    delivery: "space_agent_server_api",
    delivery_target: "space_agent_onscreen_prompt",
    auto_execution: false,
    message_id: messageId,
    pickup_hint: "Open ~/aurago_inbox/latest_instruction.json in Space Agent and execute the instruction."
  };

  await fs.mkdir(inboxDir, { recursive: true, mode: 0o700 });
  await fs.writeFile(path.join(inboxDir, "latest_instruction.json"), JSON.stringify(record, null, 2) + "\n", {
    mode: 0o600
  });
  await fs.writeFile(path.join(inboxDir, "latest_instruction.txt"), message + "\n", {
    mode: 0o600
  });
  await fs.appendFile(path.join(inboxDir, "instructions.jsonl"), JSON.stringify(record) + "\n", {
    mode: 0o600
  });
  const onscreenHistoryPath = await appendOnscreenAgentHistory(userRoot, record);

  return {
    accepted: true,
    queued: true,
    delivered: "space_agent_server_api",
    auto_execution: false,
    message_id: messageId,
    message: "AuraGo instruction accepted by Space Agent and written to the managed inbox and onscreen history.",
    inbox_path: "~/aurago_inbox/latest_instruction.json",
    onscreen_history_path: onscreenHistoryPath
  };
}
`
}

func spaceAgentBootstrapScript() string {
	return `import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

import { loadSupervisorAuthEnv } from "./commands/lib/supervisor/auth_keys.js";
import { createUser, setUserPassword } from "./server/lib/auth/user_manage.js";

const username = String(process.env.SPACE_AGENT_ADMIN_USER || "").trim();
const password = String(process.env.SPACE_AGENT_ADMIN_PASSWORD || "");
const projectRoot = process.cwd();
const stateDir = "/app/supervisor";
const managedStatePath = path.join(stateDir, "auth", "aurago_managed_user.json");

function normalizeEntityId(value) {
  const raw = String(value || "").trim().replaceAll("\\", "/");
  if (!raw || raw.includes("/")) {
    throw new Error("Managed Space Agent username must be a single path segment.");
  }
  const normalized = path.posix.normalize(raw);
  if (!normalized || normalized === "." || normalized === ".." || normalized.includes("/")) {
    throw new Error("Managed Space Agent username must be a single path segment.");
  }
  return normalized;
}

function digestPassword(value) {
  return createHash("sha256").update(String(value || ""), "utf8").digest("hex");
}

function readManagedState() {
  try {
    const parsed = JSON.parse(fs.readFileSync(managedStatePath, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function writeManagedState(usernameValue, passwordDigest) {
  fs.mkdirSync(path.dirname(managedStatePath), { recursive: true, mode: 0o700 });
  fs.writeFileSync(
    managedStatePath,
    JSON.stringify({
      password_sha256: passwordDigest,
      updated_at: new Date().toISOString(),
      username: usernameValue
    }, null, 2) + "\n",
    { mode: 0o600 }
  );
}

function clearInvalidatedUserCrypto(usernameValue) {
  const customwarePath = process.env.CUSTOMWARE_PATH || "/app/customware";
  fs.rmSync(path.join(customwarePath, "L2", usernameValue, "meta", "user_crypto.json"), {
    force: true
  });
}

function seedFile(filePath, content) {
  if (!fs.existsSync(filePath)) {
    fs.writeFileSync(filePath, content, { mode: 0o600 });
  }
}

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true, mode: 0o750 });
  fs.writeFileSync(filePath, content, { mode: 0o600 });
}

const bridgeHelperESMTemplate = ` + strconv.Quote(spaceAgentBridgeHelperESM("__AURAGO_BRIDGE_URL__", "__AURAGO_BRIDGE_TOKEN__")) + `;
const bridgeHelperCJSTemplate = ` + strconv.Quote(spaceAgentBridgeHelperCJS("__AURAGO_BRIDGE_URL__", "__AURAGO_BRIDGE_TOKEN__")) + `;

function jsStringLiteralContent(value) {
  return JSON.stringify(String(value || "")).slice(1, -1);
}

function bridgeHelperContent(template) {
  return template
    .replaceAll("__AURAGO_BRIDGE_URL__", jsStringLiteralContent(process.env.AURAGO_BRIDGE_URL || ""))
    .replaceAll("__AURAGO_BRIDGE_TOKEN__", jsStringLiteralContent(process.env.AURAGO_BRIDGE_TOKEN || ""));
}

function bridgeURLUsesLoopback(value) {
  try {
    const url = new URL(String(value || ""));
    const host = url.hostname.toLowerCase().replace(/^\[|\]$/g, "");
    return host === "localhost" || host === "::1" || host === "127.0.0.1" || host.startsWith("127.");
  } catch {
    return false;
  }
}

function bridgeConfigJSON() {
  const rawBridgeURL = process.env.AURAGO_BRIDGE_URL || "";
  const browserBridgeURL = bridgeURLUsesLoopback(rawBridgeURL) ? "" : rawBridgeURL;
  return JSON.stringify({
    bridge_url: browserBridgeURL,
    bridge_token: process.env.AURAGO_BRIDGE_TOKEN || "",
    browser_bridge_url_strategy: "Import aurago_bridge.js; it derives https://aurago.../api/space-agent/bridge/messages from https://aurago-space-agent... at runtime.",
    note: "Browser contexts should import aurago_bridge.js instead of reading process.env directly."
  }, null, 2) + "\n";
}

function seedWorkspaceFiles(rootPath) {
  for (const dir of [
    rootPath,
    path.join(rootPath, "meta"),
    path.join(rootPath, "spaces"),
    path.join(rootPath, "conf"),
    path.join(rootPath, "hist"),
    path.join(rootPath, "docs"),
    path.join(rootPath, "dashboard"),
    path.join(rootPath, "onscreen-agent"),
    path.join(rootPath, ".config"),
    path.join(rootPath, ".local", "share")
  ]) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o750 });
  }
  writeFile(path.join(rootPath, "AGENTS.md"), ` + strconv.Quote(spaceAgentAuraGoAgentsMarkdown()) + `);
  writeFile(path.join(rootPath, "conf", "aurago.system.include.md"), ` + strconv.Quote(spaceAgentAuraGoSystemInclude()) + `);
  writeFile(path.join(rootPath, "docs", "aurago-bridge.md"), ` + strconv.Quote(spaceAgentAuraGoBridgeReadme()) + `);
  writeFile(path.join(rootPath, "mod", "aurago", "inbox_poller", "ext", "js", "_core", "framework", "initializer.js", "initialize", "end", "aurago-inbox-poller.js"), ` + strconv.Quote(spaceAgentInboxPollerJS()) + `);
  writeFile(path.join(rootPath, "aurago_bridge.js"), bridgeHelperContent(bridgeHelperESMTemplate));
  writeFile(path.join(rootPath, "aurago_bridge.cjs"), bridgeHelperContent(bridgeHelperCJSTemplate));
  writeFile(path.join(rootPath, "aurago_bridge_config.json"), bridgeConfigJSON());
  seedFile(path.join(rootPath, "meta", "login_hooks.json"), "[]\n");
  seedFile(path.join(rootPath, "conf", "dashboard.yaml"), "{}\n");
  seedFile(path.join(rootPath, "conf", "onscreen-agent.yaml"), "{}\n");
  seedFile(path.join(rootPath, "hist", "onscreen-agent.json"), "[]\n");
  seedFile(path.join(rootPath, "dashboard", "prefs.json"), "{}\n");
  seedFile(path.join(rootPath, "dashboard", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, "onscreen-agent", "config.json"), "{}\n");
  seedFile(path.join(rootPath, "onscreen-agent", "history.json"), "[]\n");
  seedFile(path.join(rootPath, "meta", "onscreen-agent-config.json"), "{}\n");
  seedFile(path.join(rootPath, "meta", "onscreen-agent-history.json"), "[]\n");
  seedFile(path.join(rootPath, "meta", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "dashboard-prefs.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "onscreen-agent-config.json"), "{}\n");
  seedFile(path.join(rootPath, ".config", "onscreen-agent-history.json"), "[]\n");
}

if (username && password) {
  process.env.CUSTOMWARE_PATH = process.env.CUSTOMWARE_PATH || "/app/customware";
  const normalizedUsername = normalizeEntityId(username);
  const passwordDigest = digestPassword(password);
  writeFile(path.join(process.env.CUSTOMWARE_PATH, "aurago_bridge.js"), bridgeHelperContent(bridgeHelperESMTemplate));
  writeFile(path.join(process.env.CUSTOMWARE_PATH, "aurago_bridge.cjs"), bridgeHelperContent(bridgeHelperCJSTemplate));
  writeFile(path.join(process.env.CUSTOMWARE_PATH, "aurago_bridge_config.json"), bridgeConfigJSON());
  writeFile(path.join(process.env.CUSTOMWARE_PATH, "aurago_bridge.md"), ` + strconv.Quote(spaceAgentBridgeHelperReadme()) + `);
  seedWorkspaceFiles(path.join(process.env.CUSTOMWARE_PATH, "L2", normalizedUsername));
  const auth = await loadSupervisorAuthEnv({ env: process.env, stateDir });
  Object.assign(process.env, auth.env);

  try {
    createUser(projectRoot, username, password, { fullName: username });
    writeManagedState(normalizedUsername, passwordDigest);
    console.log("[aurago-bootstrap] Created managed Space Agent user " + username + ".");
  } catch (error) {
    if (!String(error?.message || "").startsWith("User already exists:")) {
      throw error;
    }
    const managedState = readManagedState();
    if (
      managedState.username === normalizedUsername &&
      managedState.password_sha256 === passwordDigest
    ) {
      console.log("[aurago-bootstrap] Managed Space Agent user " + username + " already current.");
    } else {
      setUserPassword(projectRoot, username, password);
      clearInvalidatedUserCrypto(normalizedUsername);
      writeManagedState(normalizedUsername, passwordDigest);
      console.log("[aurago-bootstrap] Updated managed Space Agent user " + username + ".");
    }
  }
}
`
}
