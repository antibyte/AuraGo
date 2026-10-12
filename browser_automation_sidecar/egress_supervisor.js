function noteEgressProxyExit(input) {
  const limit = input.limit;
  const windowMs = input.windowMs;
  let restarts = input.restarts;
  let windowStart = input.windowStart;
  if (input.shuttingDown || !input.accepted || input.gaveUp) {
    return { action: 'stop', restarts, windowStart, gaveUp: input.gaveUp };
  }
  if (input.now - windowStart > windowMs) {
    restarts = 0;
    windowStart = input.now;
  }
  if (restarts >= limit) {
    return { action: 'give-up', restarts, windowStart, gaveUp: true };
  }
  return { action: 'restart', restarts: restarts + 1, windowStart, gaveUp: false };
}

module.exports = { noteEgressProxyExit };
