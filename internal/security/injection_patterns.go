package security

import (
	"strings"
	"unicode/utf8"

	"github.com/danielthedm/promptsec"
	"github.com/danielthedm/promptsec/guard/heuristic"
)

// The promptsec heuristic set carries override rules for English, German,
// Spanish, French and Croatian only. These rules add AuraGo's own plain-language
// instruction-override and system-prompt-disclosure phrases for Italian,
// Portuguese, Polish, Dutch, Turkish, Russian, Chinese, Japanese and Korean,
// and the chat-template tokens of the Llama 3, Gemma, GPT and Qwen families.
//
// The rules come in two tiers, because manuals, changelogs and docs in these
// languages say "ignore the previous instructions if the device is already
// connected" or "show the system message" all the time:
//
//   - injectionRuleSeverity (0.85, ThreatCritical, withholds external output):
//     an override that reaches for everything ("all instructions", the way
//     promptsec's "ignore all previous") or for the assistant's own ("your
//     instructions"), and a request for the system prompt or the hidden
//     instructions.
//   - injectionWarnSeverity (0.45, ThreatMedium, a warning line only): an
//     override of "the previous/earlier instructions" without either, and a
//     request for the "system message". Nouns that are mostly everyday words
//     ("settings", "system information") match no rule.
//
// The sanitizer rewrites the text before the heuristics run: it folds accented
// Latin letters and swaps Cyrillic lookalikes (a, e, o, p, c, x, y, ...) for
// Latin ones. Rules therefore accept both spellings: Latin rules list the
// accented and the plain letter, and cyrillicRule turns each lookalike letter
// into a class of the Cyrillic and the Latin form.

const (
	injectionRuleSeverity    = 0.85
	injectionWarnSeverity    = 0.45
	chatTemplateRuleSeverity = 0.9
)

// Building blocks of the Chinese rules (the rules repeat them for the strong
// and the warning tier).
const (
	zhIgnore   = `(?:忽略|无视|無視|忽視|不要理会|不要理會|别管|別管)`
	zhForget   = `(?:忘记|忘記|忘掉|忘了|遗忘|遺忘|抛弃|拋棄)`
	zhObey     = `不要(?:再)?(?:遵守|遵循|听从|聽從|服从|服從|执行|執行)`
	zhPrevious = `(?:(?:之前|以前|先前|此前|前面|上面|上述|以上|原来|原有|原先)(?:的)?)`
	zhAll      = `(?:(?:所有|全部|一切)(?:的)?)`
	zhYour     = `(?:你的|您的|你|您)`
	zhNoun     = `(?:(?:系统|系統)?(?:指令|指示|说明|說明|规则|規則|命令|提示词|提示詞))`
	zhNounForg = `(?:指令|指示|规则|規則|命令|提示词|提示詞)`
	zhNounObey = `(?:指令|指示|规则|規則|命令)`
	zhDiscVerb = `(?:显示|顯示|展示|输出|輸出|告诉我|告訴我|透露|泄露|洩露|泄漏|打印|列出|重复|重複|复述|複述|说出|說出|公开|公開)(?:出)?(?:你的|您的|你|您|全部|完整的?|原始的?)?(?:的)?`
)

// Building blocks of the Japanese and Korean rules.
const (
	jaPrevious = `(?:これまで|今まで|以前|前|上記|上述|先ほど)の`
	jaNoun     = `(?:指示|命令|ルール|指令|プロンプト|ガイドライン)`
	jaTail     = `(?:を|は|も)(?:すべて|全て|全部|一切)?(?:無視|忘れ|破棄|取り消|放棄|従わな)`
	jaDiscTail = `(?:を|は|の内容を)(?:そのまま)?(?:すべて|全て|全文)?(?:表示|出力|教え|見せ|開示|暴露|公開|繰り返|言っ|書い|漏らし)`
	koPrevious = `(?:이전|위의?|앞의|기존|지금까지의?|이전의)`
	koNoun     = `(?:지시|지침|명령|규칙|프롬프트)`
	koTail     = `\S{0,3}\s*(?:모두\s*|전부\s*|다\s*)?(?:무시|잊어|잊으|잊고|따르지\s*마|따르지\s*말)`
	koDiscTail = `\S{0,3}\s*(?:모두\s*|그대로\s*|전부\s*)?(?:보여|출력|알려|공개|말해|반복|보내|적어|유출)`
)

// cyrillicLookalikes are the Cyrillic letters the sanitizer rewrites to Latin.
var cyrillicLookalikes = map[rune]rune{
	'а': 'a', 'в': 'b', 'с': 'c', 'е': 'e', 'н': 'h', 'к': 'k', 'м': 'm',
	'о': 'o', 'р': 'p', 'т': 't', 'х': 'x', 'у': 'y', 'з': '3', 'і': 'i',
}

// cyrillicRule compiles a Cyrillic phrase into a pattern that matches the
// phrase as written and as the sanitizer leaves it. A space stands for \s+, '~'
// for any word ending; ASCII group syntax such as (?:a|b)? passes through.
func cyrillicRule(phrase string) string {
	var b strings.Builder
	b.WriteString("(?i)")
	for i := 0; i < len(phrase); {
		r, size := utf8.DecodeRuneInString(phrase[i:])
		i += size
		switch {
		case r == ' ':
			b.WriteString(`\s+`)
		case r == '~':
			b.WriteString(`\p{L}*`)
		case r == 'е':
			b.WriteString("[eеё]")
		case cyrillicLookalikes[r] != 0:
			b.WriteString("[" + string(cyrillicLookalikes[r]) + string(r) + "]")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

type injectionRule struct {
	pattern     string
	threatType  promptsec.ThreatType
	severity    float64
	description string
}

// auraGoInjectionPatterns returns the Guardian's own heuristic rules.
func auraGoInjectionPatterns() []heuristic.PatternEntry {
	const (
		override = promptsec.ThreatInstructionOverride
		leak     = promptsec.ThreatSystemPromptLeak
		strong   = injectionRuleSeverity
		warn     = injectionWarnSeverity
	)
	rules := []injectionRule{
		// ── Italian ──────────────────────────────────────────────────
		{`(?i)\b(?:ignora|ignorare|ignorate|dimentica|dimenticate|scorda|trascura)\s+(?:completamente\s+)?tutt[ei]\s+(?:(?:le|gli|i)\s+)?(?:(?:tue|vostre|precedenti|vecchie)\s+)*(?:istruzion[ie]|regol[ae]|direttiv[ae]|indicazion[ie]|comand[oi])`,
			override, strong, "Italian instruction override ('ignora tutte le istruzioni')"},
		{`(?i)\b(?:ignora|ignorare|ignorate|dimentica|dimenticate|trascura|non\s+seguire|non\s+rispettare)\s+(?:(?:le|gli|i)\s+)?(?:tue|vostre)\s+(?:(?:precedenti|iniziali|originali|vecchie)\s+)?(?:istruzion[ie]|regol[ae]|direttiv[ae]|indicazion[ie])`,
			override, strong, "Italian instruction override ('ignora le tue istruzioni')"},
		{`(?i)\b(?:ignora|ignorare|ignorate|dimentica|dimenticate|trascura|non\s+seguire|non\s+rispettare)\s+(?:(?:le|gli|i)\s+)?(?:tue\s+|vostre\s+)?(?:(?:istruzion[ie]|regol[ae]|direttiv[ae]|indicazion[ie])\s+(?:precedent[ei]|sopra|di\s+sistema|iniziali|originali|di\s+prima)|(?:precedent[ei]|iniziali|originali|vecchie)\s+(?:istruzion[ie]|regol[ae]|direttiv[ae]|indicazion[ie]))`,
			override, warn, "Italian override of previous instructions ('ignora le istruzioni precedenti')"},
		{`(?i)\b(?:mostra|mostrami|rivela|rivelami|svela|stampa|ripeti|dimmi|visualizza|scrivi)[^.\n]{0,40}?\b(?:prompt\s+di\s+sistema|istruzioni\s+di\s+sistema|istruzioni\s+iniziali|prompt\s+iniziale|system\s+prompt)\b`,
			leak, strong, "Italian system prompt disclosure request"},
		{`(?i)\b(?:mostra|mostrami|rivela|rivelami|svela|stampa|ripeti|dimmi|visualizza|scrivi)[^.\n]{0,40}?\bmessaggio\s+di\s+sistema\b`,
			leak, warn, "Italian system message request"},

		// ── Portuguese ───────────────────────────────────────────────
		{`(?i)\b(?:ignor[ae]|ignorem|esque[cç]a|esque[cç]am|desconsidere|desconsidera|descarte)\s+(?:completamente\s+)?tod[oa]s\s+(?:(?:as|os)\s+)?(?:(?:suas|seus|vossas)\s+)?(?:(?:anteriores|pr[eé]vias)\s+)?(?:instru[cç][oõó]es|regras|diretrizes|diretivas|ordens|comandos|orienta[cç][oõó]es)`,
			override, strong, "Portuguese instruction override ('ignore todas as instruções')"},
		{`(?i)\b(?:ignor[ae]|ignorem|esque[cç]a|desconsidere|desconsidera|descarte)\s+(?:(?:as|os)\s+)?(?:suas|seus|vossas)\s+(?:(?:anteriores|pr[eé]vias|iniciais|originais)\s+)?(?:instru[cç][oõó]es|regras|diretrizes|ordens|comandos)`,
			override, strong, "Portuguese instruction override ('ignore suas instruções')"},
		{`(?i)\b(?:ignor[ae]|ignorem|esque[cç]a|desconsidere|desconsidera|descarte)\s+(?:(?:as|os)\s+)?(?:(?:suas|seus)\s+)?(?:(?:instru[cç][oõó]es|regras|diretrizes|ordens|comandos)\s+(?:anteriores|pr[eé]vias|acima|iniciais|originais|do\s+sistema)|(?:anteriores|pr[eé]vias|iniciais|originais)\s+(?:instru[cç][oõó]es|regras|diretrizes|ordens|comandos))`,
			override, warn, "Portuguese override of previous instructions ('ignore as instruções anteriores')"},
		{`(?i)\b(?:mostre|mostra|revele|revela|exiba|imprima|repita|diga|liste)[^.\n]{0,40}?\b(?:prompt\s+(?:do|de)\s+sistema|instru[cç][oõó]es\s+(?:do|de)\s+sistema|instru[cç][oõó]es\s+iniciais|prompt\s+inicial|system\s+prompt)\b`,
			leak, strong, "Portuguese system prompt disclosure request"},
		{`(?i)\b(?:mostre|mostra|revele|revela|exiba|imprima|repita|diga|liste)[^.\n]{0,40}?\bmensagem\s+(?:do|de)\s+sistema\b`,
			leak, warn, "Portuguese system message request"},

		// ── Polish ───────────────────────────────────────────────────
		{`(?i)\b(?:zignoruj|ignoruj|pomi[nń]|zapomnij(?:\s+o)?|odrzu[cć])\s+(?:wszystk\p{L}*|wszelk\p{L}*)\s+(?:(?:poprzedni\p{L}*|wcze[sś]niejsz\p{L}*|powy[zż]sz\p{L}*|swoj\p{L}*|twoj\p{L}*)\s+)?(?:instrukcj\p{L}*|polece\p{L}*|zasad\p{L}*|wytyczn\p{L}*|komend\p{L}*|dyrektyw\p{L}*)`,
			override, strong, "Polish instruction override ('zignoruj wszystkie instrukcje')"},
		{`(?i)\b(?:zignoruj|ignoruj|pomi[nń]|zapomnij(?:\s+o)?|odrzu[cć])\s+(?:(?:poprzedni\p{L}*|wcze[sś]niejsz\p{L}*|powy[zż]sz\p{L}*|dotychczasow\p{L}*|pierwotn\p{L}*)\s+)?(?:swoj\p{L}*|twoj\p{L}*)\s+(?:(?:poprzedni\p{L}*|wcze[sś]niejsz\p{L}*|powy[zż]sz\p{L}*|dotychczasow\p{L}*|pierwotn\p{L}*)\s+)?(?:instrukcj\p{L}*|polece\p{L}*|zasad\p{L}*|wytyczn\p{L}*|komend\p{L}*|dyrektyw\p{L}*)`,
			override, strong, "Polish instruction override ('zignoruj swoje instrukcje')"},
		{`(?i)\b(?:zignoruj|ignoruj|pomi[nń]|zapomnij(?:\s+o)?|odrzu[cć])\s+(?:(?:swoj\p{L}*|twoj\p{L}*)\s+)?(?:poprzedni\p{L}*|wcze[sś]niejsz\p{L}*|powy[zż]sz\p{L}*|dotychczasow\p{L}*|pierwotn\p{L}*|swoj\p{L}*|twoj\p{L}*)\s+(?:instrukcj\p{L}*|polece\p{L}*|zasad\p{L}*|wytyczn\p{L}*|komend\p{L}*|dyrektyw\p{L}*)`,
			override, warn, "Polish override of previous instructions ('zignoruj poprzednie instrukcje')"},
		{`(?i)\b(?:poka[zżź]\p{L}*|ujawnij|wy[sś]wietl|wypisz|powt[oó]rz|podaj|wydrukuj)[^.\n]{0,40}?\b(?:prompt\p{L}*\s+systemow\p{L}*|systemow\p{L}*\s+prompt\p{L}*|instrukcj\p{L}*\s+systemow\p{L}*|systemow\p{L}*\s+instrukcj\p{L}*|pierwotn\p{L}*\s+instrukcj\p{L}*|system\s+prompt)`,
			leak, strong, "Polish system prompt disclosure request"},
		{`(?i)\b(?:poka[zżź]\p{L}*|ujawnij|wy[sś]wietl|wypisz|powt[oó]rz|podaj|wydrukuj)[^.\n]{0,40}?\bpolece\p{L}*\s+systemow\p{L}*`,
			leak, warn, "Polish system command request"},

		// ── Dutch ────────────────────────────────────────────────────
		{`(?i)\b(?:negeer|negeren|vergeet|vergeten)\s+(?:alle|al\s+je|al\s+uw)\s+(?:(?:je|jouw|uw|de|het|vorige|eerdere|voorgaande|bovenstaande)\s+)*(?:instructies|instructie|opdrachten|regels|richtlijnen|aanwijzingen|commando's|commandos)`,
			override, strong, "Dutch instruction override ('negeer alle instructies')"},
		{`(?i)\b(?:negeer|negeren|vergeet|vergeten)\s+(?:(?:vorige|eerdere|voorgaande|bovenstaande|oorspronkelijke)\s+)?(?:je|jouw|uw)\s+(?:(?:vorige|eerdere|voorgaande|bovenstaande|oorspronkelijke)\s+)?(?:instructies|opdrachten|regels|richtlijnen|aanwijzingen)`,
			override, strong, "Dutch instruction override ('negeer je instructies')"},
		{`(?i)\b(?:negeer|negeren|vergeet|vergeten)\s+(?:(?:de|je|jouw|uw)\s+)?(?:(?:vorige|eerdere|voorgaande|bovenstaande|oorspronkelijke)\s+|(?:je|jouw|uw)\s+)(?:instructies|opdrachten|regels|richtlijnen|aanwijzingen)`,
			override, warn, "Dutch override of previous instructions ('negeer de vorige instructies')"},
		{`(?i)\b(?:toon|laat\s+zien|onthul|geef|herhaal|print|schrijf|vertel)[^.\n]{0,40}?\b(?:systeem\s?prompt|systeeminstructies|begininstructies|beginprompt|verborgen\s+instructies|system\s+prompt)\b`,
			leak, strong, "Dutch system prompt disclosure request"},
		{`(?i)\b(?:toon|laat\s+zien|onthul|geef|herhaal|print|schrijf|vertel)[^.\n]{0,40}?\bsysteembericht\b`,
			leak, warn, "Dutch system message request"},

		// ── Turkish ──────────────────────────────────────────────────
		{`(?i)(?:(?:[oö]nceki|[uü]stteki|yukar[iıİ]daki|ilk)\s+)?(?:t[uü]m|b[uü]t[uü]n)\s+(?:(?:[oö]nceki|[uü]stteki|yukar[iıİ]daki|ilk)\s+)?(?:talimat|y[oö]nerge|kural|komut|y[oö]nlendirme|direktif)\p{L}*\s+(?:yok\s+say|g[oö]rmezden\s+gel|unut|dikkate\s+alma|ge[cç]ersiz\s+say|bir\s+kenara\s+b[iıİ]rak)`,
			override, strong, "Turkish instruction override ('tüm talimatları yok say')"},
		{`(?i)(?:[oö]nceki|[uü]stteki|yukar[iıİ]daki|ilk)\s+(?:talimat|y[oö]nerge|kural|komut|y[oö]nlendirme|direktif)\p{L}*\s+(?:yok\s+say|g[oö]rmezden\s+gel|unut|dikkate\s+alma|ge[cç]ersiz\s+say|bir\s+kenara\s+b[iıİ]rak)`,
			override, warn, "Turkish override of previous instructions ('önceki talimatları yok say')"},
		{`(?i)(?:sistem\s+(?:istem|prompt|talimat|y[oö]nerge)\p{L}*|ba[sş]lang[iıİ][cç]\s+(?:talimat|prompt|istem)\p{L}*|gizli\s+talimat\p{L}*)[^.\n]{0,30}?\b(?:g[oö]ster|payla[sş]|a[cç][iıİ]kla|yazd[iıİ]r|tekrarla|s[oö]yle)`,
			leak, strong, "Turkish system prompt disclosure request"},
		{`(?i)sistem\s+mesaj\p{L}*[^.\n]{0,30}?\b(?:g[oö]ster|payla[sş]|a[cç][iıİ]kla|yazd[iıİ]r|tekrarla|s[oö]yle)`,
			leak, warn, "Turkish system message request"},

		// ── Russian (Cyrillic and sanitizer-folded spelling) ─────────
		{cyrillicRule(`(?:проигнорируй|игнорируй|игнорировать)(?:те)? (?:(?:все|всю|любые|всё|свои|ваши|твои) )+(?:(?:предыдущ~|прежн~|прошл~|вышеуказанн~|вышесказанн~|изначальн~) )?(?:инструкци~|указани~|команд~|правил~|директив~)`),
			override, strong, "Russian instruction override ('игнорируй все предыдущие инструкции')"},
		{cyrillicRule(`(?:проигнорируй|игнорируй|игнорировать)(?:те)? (?:(?:все|всю|любые|всё) )?(?:(?:свои|ваши|твои) )?(?:предыдущ~|прежн~|прошл~|вышеуказанн~|вышесказанн~|изначальн~) (?:инструкци~|указани~|команд~|правил~|директив~)`),
			override, warn, "Russian override of previous instructions ('игнорируй предыдущие инструкции')"},
		{cyrillicRule(`(?:забудь|забудьте|забыть) (?:(?:все|всё|свои|ваши|любые|твои) )+(?:(?:прежн~|предыдущ~|прошл~|изначальн~) )?(?:инструкци~|указани~|правил~|команд~|директив~)`),
			override, strong, "Russian instruction override ('забудь все инструкции')"},
		{cyrillicRule(`(?:забудь|забудьте|забыть)(?: (?:все|всё|свои|ваши|любые|твои))? (?:прежн~|предыдущ~|прошл~|изначальн~) (?:инструкци~|указани~|правил~|команд~|директив~)`),
			override, warn, "Russian override of previous instructions ('забудь предыдущие инструкции')"},
		{cyrillicRule(`не (?:обращай|обращайте|учитывай|учитывайте) (?:внимани(?:я|е) на )?(?:(?:все|любые|свои|ваши) )+(?:предыдущ~|прежн~|прошл~) (?:инструкци~|указани~|правил~|команд~)`),
			override, strong, "Russian instruction override ('не обращай внимания на все предыдущие инструкции')"},
		{cyrillicRule(`не (?:обращай|обращайте|учитывай|учитывайте) (?:внимани(?:я|е) на )?(?:(?:все|любые) )?(?:предыдущ~|прежн~|прошл~) (?:инструкци~|указани~|правил~|команд~)`),
			override, warn, "Russian override of previous instructions ('не обращай внимания на предыдущие инструкции')"},
		{cyrillicRule(`(?:покажи|покажите|выведи|выведите|раскрой|раскройте|напечатай|напечатайте|повтори|повторите|расскажи|расскажите|скажи|скажите|назови|дай)(?: мне)?(?: (?:свой|твой|ваш|свои|твои|ваши|весь|полный|исходный|начальный))* (?:системн~ (?:промпт|промт|инструкци~|подсказк~|указани~)|исходн~ (?:промпт|промт|инструкци~)|начальн~ (?:промпт|промт|инструкци~|указани~)|скрыт~ инструкци~)`),
			leak, strong, "Russian system prompt disclosure request"},
		{cyrillicRule(`(?:покажи|покажите|выведи|выведите|раскрой|раскройте|напечатай|напечатайте|повтори|повторите|расскажи|расскажите|скажи|скажите|назови|дай)(?: мне)?(?: (?:свой|твой|ваш|свои|твои|ваши|весь|полный|исходный|начальный))* системн~ сообщени~`),
			leak, warn, "Russian system message request"},

		// ── Chinese ──────────────────────────────────────────────────
		{zhIgnore + `(?:你)?(?:的)?` + zhPrevious + `?` + zhAll + zhPrevious + `?` + zhNoun,
			override, strong, "Chinese instruction override ('忽略之前的所有指令')"},
		{zhIgnore + zhYour + zhPrevious + `?` + zhNoun,
			override, strong, "Chinese instruction override ('忽略你的指令')"},
		{zhIgnore + `(?:你)?` + zhPrevious + zhNoun,
			override, warn, "Chinese override of previous instructions ('忽略之前的说明')"},
		{zhForget + `(?:你)?(?:的)?` + zhPrevious + `?` + zhAll + zhNounForg,
			override, strong, "Chinese instruction override ('忘记之前的所有指令')"},
		{zhForget + zhYour + zhPrevious + `?` + zhNounForg,
			override, strong, "Chinese instruction override ('忘记你的指令')"},
		{zhForget + `(?:你)?` + zhPrevious + zhNounForg,
			override, warn, "Chinese override of previous instructions ('忘记之前的指令')"},
		{zhObey + `(?:你)?(?:的)?` + zhPrevious + `?` + zhAll + zhNounObey,
			override, strong, "Chinese instruction override ('不要遵守之前的所有指令')"},
		{zhObey + zhYour + zhPrevious + `?` + zhNounObey,
			override, strong, "Chinese instruction override ('不要遵守你的指令')"},
		{zhObey + `(?:你)?` + zhPrevious + zhNounObey,
			override, warn, "Chinese override of previous instructions ('不要遵守之前的指令')"},
		{zhDiscVerb + `(?:系统提示词|系統提示詞|系统指令|系統指令|初始指令|初始提示词|初始提示詞|原始指令|隐藏指令|隱藏指令|开发者指令|開發者指令)`,
			leak, strong, "Chinese system prompt disclosure request"},
		{zhDiscVerb + `(?:系统提示|系統提示|系统消息|系統消息)`,
			leak, warn, "Chinese system message request"},

		// ── Japanese ─────────────────────────────────────────────────
		{`(?:` + jaPrevious + `)?(?:すべて|全て|全部)の?` + jaNoun + jaTail,
			override, strong, "Japanese instruction override ('すべての指示を無視')"},
		{`あなたの(?:` + jaPrevious + `)?` + jaNoun + jaTail,
			override, strong, "Japanese instruction override ('あなたの指示を無視')"},
		{`(?:指示|命令|ルール|指令)(?:を|は)(?:すべて|全て|全部|一切)(?:無視|忘れ|破棄)`,
			override, strong, "Japanese instruction override ('指示をすべて無視')"},
		{jaPrevious + jaNoun + jaTail,
			override, warn, "Japanese override of previous instructions ('以前の指示を無視')"},
		{`(?:システムプロンプト|システム指示|初期プロンプト|初期指示|隠された指示|内部指示)` + jaDiscTail,
			leak, strong, "Japanese system prompt disclosure request"},
		{`システムメッセージ` + jaDiscTail,
			leak, warn, "Japanese system message request"},

		// ── Korean ───────────────────────────────────────────────────
		{`(?:` + koPrevious + `\s*)?(?:모든|너의|당신의|네)\s*(?:` + koPrevious + `\s*)?(?:모든\s*)?` + koNoun + koTail,
			override, strong, "Korean instruction override ('이전의 모든 지시를 무시')"},
		{koPrevious + `\s*` + koNoun + koTail,
			override, warn, "Korean override of previous instructions ('이전 지침을 무시')"},
		{`(?:시스템\s*프롬프트|시스템\s*지시(?:사항)?|초기\s*(?:지침|프롬프트|지시)|숨겨진\s*(?:지침|지시|프롬프트))` + koDiscTail,
			leak, strong, "Korean system prompt disclosure request"},
		{`시스템\s*메시지` + koDiscTail,
			leak, warn, "Korean system message request"},

		// ── Chat-template tokens ─────────────────────────────────────
		// The promptsec set covers ChatML, <|system|> style roles, <system>
		// and [INST]; these are the Llama 3, Gemma, GPT and Hermes/Qwen
		// control tokens.
		{`(?i)<\|(?:start_header_id|end_header_id|eot_id|eom_id|begin_of_text)\|>`,
			promptsec.ThreatDelimiterInjection, chatTemplateRuleSeverity, "Llama 3 chat-template token injection"},
		{`(?i)</?(?:start|end)_of_turn>`,
			promptsec.ThreatDelimiterInjection, chatTemplateRuleSeverity, "Gemma chat-template token injection"},
		{`(?i)<\|endoftext\|>`,
			promptsec.ThreatDelimiterInjection, chatTemplateRuleSeverity, "end-of-text token injection"},
		{`(?i)</?tool_call>`,
			promptsec.ThreatDelimiterInjection, chatTemplateRuleSeverity, "tool-call tag injection"},
	}

	entries := make([]heuristic.PatternEntry, 0, len(rules))
	for _, r := range rules {
		entries = append(entries, heuristic.PatternEntry{
			Pattern:     r.pattern,
			ThreatType:  r.threatType,
			Severity:    r.severity,
			Description: r.description,
		})
	}
	return entries
}
