// Upstream stylesheets fetch fonts from Google and perplexity.ai; AuraGo serves every font itself.
const googleFonts = "@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;600;700&family=JetBrains+Mono:wght@400;600&display=swap');";
const perplexityFont = `@font-face {
  font-family: 'FKGroteskNeue';
  src: url('https://www.perplexity.ai/fonts/FKGroteskNeue.woff2')
    format('woff2');
}
`;

export function localizeFonts(name, text, patch) {
    if (name === 'observatory/css/observatory.css' || name === 'pose-fusion/css/style.css') {
        text = patch(text, googleFonts, "@import url('/fonts/fonts.css');\n@import url('/fonts/jetbrains-mono.css');");
    }
    if (name === 'style.css') {
        // FK Grotesk Neue is proprietary; the stack continues with the bundled Geist.
        text = "@import url('/fonts/fonts.css');\n" + patch(text, perplexityFont, '');
    }
    if (/@import\s+(?:url\()?\s*['"]?(?:https?:)?\/\/|url\(\s*['"]?(?:https?:)?\/\//i.test(text)) {
        throw Error('RuView stylesheet loads a remote resource: ' + name);
    }
    return text;
}
