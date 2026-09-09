FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e

ARG GEV_REVISION=759652207fd1279ece97f0f19af566feb9a82146
LABEL org.opencontainers.image.source="https://github.com/antibyte/AuraGo" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="AuraGo God's Eye View" \
      org.opencontainers.image.version="gev-7596522-1" \
      org.opencontainers.image.upstream.revision="759652207fd1279ece97f0f19af566feb9a82146"

ADD https://codeload.github.com/bilawalsidhu/gods-eye-view/tar.gz/${GEV_REVISION} /tmp/gev.tar.gz
WORKDIR /app
RUN tar -xzf /tmp/gev.tar.gz --strip-components=1 -C /app \
    && rm /tmp/gev.tar.gz
ENV PUPPETEER_SKIP_DOWNLOAD=true
RUN npm ci --no-audit --no-fund \
    && npm cache clean --force \
    && mkdir -p .gev-cache node_modules/.vite \
    && chown -R node:node /app
COPY --chown=node:node aurago.config.mjs ./aurago.config.mjs
COPY --chown=node:node key-setup.js ./src/keySetup.js
USER node
RUN --mount=type=bind,source=adapter.test.mjs,target=/app/adapter.test.mjs \
    node --test adapter.test.mjs && rm -rf /app/.gev-cache/*
EXPOSE 4173
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 CMD node -e "fetch('http://127.0.0.1:4173/aurago-health').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "node_modules/vite/bin/vite.js", "--config", "aurago.config.mjs"]
