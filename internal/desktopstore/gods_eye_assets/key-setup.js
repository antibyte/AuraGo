// AuraGo owns provider settings. No credentials are accepted by this surface.
const labels = {
    cs: 'Klíče API: Obchod se softwarem → Nastavit',
    da: 'API-nøgler: Softwarebutik → Konfigurer',
    de: 'API-Schlüssel: Software Store → Einrichten',
    el: 'Κλειδιά API: Κατάστημα λογισμικού → Ρύθμιση',
    en: 'API keys: Software Store → Configure',
    es: 'Claves API: Tienda de software → Configurar',
    fr: 'Clés API : Boutique de logiciels → Configurer',
    hi: 'API कुंजियाँ: सॉफ़्टवेयर स्टोर → कॉन्फ़िगर करें',
    it: 'Chiavi API: Negozio software → Configura',
    ja: 'APIキー：ソフトウェアストア → 設定',
    nl: 'API-sleutels: Softwarewinkel → Instellen',
    no: 'API-nøkler: Programvarebutikk → Konfigurer',
    pl: 'Klucze API: Sklep z oprogramowaniem → Konfiguruj',
    pt: 'Chaves API: Loja de software → Configurar',
    sv: 'API-nycklar: Programvarubutik → Konfigurera',
    zh: 'API 密钥：软件商店 → 配置'
};

export async function initKeySetup({ documentRef = globalThis.document } = {}) {
    if (documentRef.getElementById('aurago-provider-notice')) return;
    const language = (new URLSearchParams(globalThis.location.search).get('aurago_lang') || documentRef.documentElement.lang || 'en').split('-')[0];
    const notice = documentRef.createElement('div');
    notice.id = 'aurago-provider-notice';
    notice.textContent = labels[language] || labels.en;
    notice.style.cssText = 'position:fixed;bottom:8px;right:8px;z-index:9999;padding:8px 12px;background:#10231eee;color:#d6e8df;border:1px solid #446757;border-radius:6px;font:12px sans-serif;max-width:75vw;pointer-events:none';
    documentRef.body.appendChild(notice);
}
