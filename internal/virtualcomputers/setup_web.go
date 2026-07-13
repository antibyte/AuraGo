package virtualcomputers

import "fmt"

const (
	boringWebSourceSHA256 = "50cddca87651ac11e4b525f13fc60572b63b29e65ba2aa60c6b84d1e610e8832"
	boringWebViteSHA256   = "1cc5f7e2f766f36c07a86a1d5fb8ad0aa22f80a1bf968b7ac87bea5fee0ae6ba"
	managedNodeVersion    = "24.11.1"
)

func managementInstallScript(opts SetupInstallOptions) string {
	installDir := opts.InstallDir
	if installDir == "" {
		installDir = "/opt/boring"
	}
	boringdURL := opts.BoringdURL
	if boringdURL == "" {
		boringdURL = defaultBoringdURL
	}

	return fmt.Sprintf(`
log "installing Boring Computers management web application"
BORING_WEB_REVISION=%s
BORING_WEB_LISTEN=%s
BORING_WEB_BORING_URL=%s
BORING_TOKEN_VALUE=%s
NODE_VERSION=%s
RELEASE_ROOT="${INSTALL_DIR}/releases"
RELEASE_ID="${BORING_WEB_REVISION}-$(date -u +%%Y%%m%%dT%%H%%M%%SZ)-$$"
RELEASE_DIR="${INSTALL_DIR}/releases/${RELEASE_ID}"
STAGING_DIR="${RELEASE_DIR}.staging"
CURRENT_LINK="${INSTALL_DIR}/current"

git -C "${REPO_DIR}" fetch --depth=1 origin ${BORING_WEB_REVISION}
git -C "${REPO_DIR}" checkout --detach ${BORING_WEB_REVISION}

apt-get install -y xz-utils python3
case "${GOARCH}" in
	amd64) NODE_ARCH="x64" ;;
	arm64) NODE_ARCH="arm64" ;;
esac
NODE_ROOT="${INSTALL_DIR}/runtime/node-v${NODE_VERSION}-linux-${NODE_ARCH}"
NODE_BIN="${NODE_ROOT}/bin"
if ! "${NODE_BIN}/node" --version 2>/dev/null | grep -qx "v${NODE_VERSION}"; then
	log "installing managed Node.js ${NODE_VERSION}"
	NODE_ARCHIVE="/tmp/aurago-node-${NODE_VERSION}-${NODE_ARCH}-$$.tar.xz"
	curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz" -o "${NODE_ARCHIVE}"
	rm -rf "${NODE_ROOT}"
	mkdir -p "${INSTALL_DIR}/runtime"
	tar -C "${INSTALL_DIR}/runtime" -xJf "${NODE_ARCHIVE}"
	rm -f "${NODE_ARCHIVE}"
fi
export PATH="${NODE_BIN}:${PATH}"

mkdir -p "${RELEASE_ROOT}"
rm -rf "${STAGING_DIR}"
mkdir -p "${STAGING_DIR}"
rsync -az --delete --exclude .git --exclude node_modules "${REPO_DIR}/" "${STAGING_DIR}/"
cd "${STAGING_DIR}"

printf '%%s  %%s\n' %s apps/web/src/lib/boring.ts | sha256sum -c -
printf '%%s  %%s\n' %s apps/web/vite.config.ts | sha256sum -c -
python3 <<'AURAGO_BORING_WEB_OVERLAY'
from pathlib import Path

boring = Path("apps/web/src/lib/boring.ts")
text = boring.read_text()
replacements = {
    "export const apiBase = PUB || '/boring';": "export const apiBase = PUB || '/boring-computers/boring';",
}
tick = chr(96)
replacements["return " + tick + "${proto}://${location.host}/boring${path}" + tick + ";"] = "return " + tick + "${proto}://${location.host}/boring-computers/boring${path}" + tick + ";"
for old, new in replacements.items():
    if old not in text:
        raise SystemExit(f"unsupported upstream boring.ts: missing {old!r}")
    text = text.replace(old, new)
boring.write_text(text)

vite = Path("apps/web/vite.config.ts")
text = vite.read_text()
old_adapter = "\t\t\t\tadapter: adapter()"
new_adapter = "\t\t\t\tadapter: adapter(),\n\t\t\t\tpaths: { base: '/boring-computers' }"
if old_adapter not in text:
    raise SystemExit("unsupported upstream vite.config.ts: adapter block changed")
text = text.replace(old_adapter, new_adapter, 1)
old_proxy = "\t\tserver: {\n\t\t\tproxy: {\n\t\t\t\t// Browser -> /boring/* -> boringd (token injected here, HTTP + WS).\n\t\t\t\t'/boring': {"
new_proxy = "\t\tserver: {\n\t\t\tproxy: {\n\t\t\t\t// AuraGo authenticated management base -> private boringd.\n\t\t\t\t'/boring-computers/boring': {"
if old_proxy not in text:
    raise SystemExit("unsupported upstream vite.config.ts: proxy block changed")
text = text.replace(old_proxy, new_proxy, 1)
text = text.replace("p.replace(/^\\/boring/, '')", "p.replace(/^\\/boring-computers\\/boring/, '')", 1)
preview = text.replace("\t\tserver: {", "\t\tpreview: {", 1)
preview_block = preview[preview.index("\t\tpreview: {"):preview.index("\n\t\ttest: {")]
text = text.replace("\n\t\ttest: {", "\n" + preview_block + "\n\t\ttest: {", 1)
vite.write_text(text)
AURAGO_BORING_WEB_OVERLAY

install -d -m0755 apps/web/static
printf '%%s\n' "${BORING_WEB_REVISION}" > apps/web/static/aurago-revision
"${NODE_BIN}/npm" ci --include=dev
PUBLIC_BORING_URL= "${NODE_BIN}/npm" run build -w web
mv "${STAGING_DIR}" "${RELEASE_DIR}"

install -d -m0755 /etc/boring
umask 077
cat > /etc/boring/boring-web.env <<EOF
BORING_URL=${BORING_WEB_BORING_URL}
BORING_TOKEN=${BORING_TOKEN_VALUE}
PUBLIC_BORING_URL=
EOF
chmod 0600 /etc/boring/boring-web.env

cat > /etc/systemd/system/boring-web.service <<EOF
[Unit]
Description=Boring Computers management web application
After=network-online.target boringd.service
Requires=boringd.service

[Service]
Type=simple
EnvironmentFile=/etc/boring/boring-web.env
Environment=PATH=${NODE_BIN}:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
WorkingDirectory=${CURRENT_LINK}/apps/web
ExecStart=${NODE_BIN}/npm exec vite -- preview --host 127.0.0.1 --port 18081 --strictPort
Restart=on-failure
RestartSec=2
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${INSTALL_DIR}

[Install]
WantedBy=multi-user.target
EOF

PREVIOUS_RELEASE="$(readlink -f "${CURRENT_LINK}" 2>/dev/null || true)"
ln -sfnT "${RELEASE_DIR}" "${CURRENT_LINK}"
systemctl daemon-reload
systemctl enable boring-web.service
if ! systemctl restart boring-web.service; then
	if [ -n "${PREVIOUS_RELEASE}" ] && [ -d "${PREVIOUS_RELEASE}" ]; then
		ln -sfnT "${PREVIOUS_RELEASE}" "${CURRENT_LINK}"
		systemctl restart boring-web.service || true
	fi
	exit 1
fi
sleep 2
systemctl is-active boring-web.service
curl -fsS --max-time 8 "http://${BORING_WEB_LISTEN}/boring-computers/" >/dev/null
find "${RELEASE_ROOT}" -mindepth 1 -maxdepth 1 -type d ! -path "${RELEASE_DIR}" ! -path "${PREVIOUS_RELEASE}" -mtime +7 -exec rm -rf -- {} +
`, shellQuote(PinnedUpstreamRevision), shellQuote(ManagementListenAddr), shellQuote(boringdURL), shellQuote(envLine(opts.Token)), shellQuote(managedNodeVersion), shellQuote(boringWebSourceSHA256), shellQuote(boringWebViteSHA256))
}
