package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// chatCodeBlockFixture renders assistant messages through the real chat
// Markdown pipeline (markdown-it, sanitizer, appendMessage) without the chat
// bootstrap, so code, Mermaid and chart fences can be measured in each theme.
const chatCodeBlockFixture = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/shared-variables.css">
<link rel="stylesheet" href="/css/hljs-github-dark.min.css"><link rel="stylesheet" href="/css/chat.bundle.css">
<style>#chat-box{height:100vh;overflow-y:auto;padding:24px;box-sizing:border-box}</style></head>
<body><main id="chat-box"><div id="chat-content"></div></main>
<script>
document.documentElement.setAttribute('data-theme', new URLSearchParams(location.search).get('theme') || 'dark');
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));
window.t=key=>key;window.chatBox=document.getElementById('chat-box');window.chatContent=document.getElementById('chat-content');
window.seenSSEImages=new Set();window.__copied=[];
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:text=>{__copied.push(text);return Promise.resolve()}}});
</script>
<script src="/js/vendor/markdown-it.min.js"></script><script src="/js/vendor/highlight.min.js"></script>
<script src="/js/shared/render-markdown.js"></script><script src="/js/shared/chat-core.js"></script>
<script src="/js/chat/ui-icons.js"></script><script src="/js/chat/modules/code-blocks.js"></script>
<script src="/js/chat/chat-messages.js"></script>
<script>window.CodeBlocks.init();</script></body></html>`

func TestChatCodeBlocksRenderCompactBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(chatCodeBlockFixture))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer server.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	controlURL := launch.MustLaunch()
	defer func() { launch.Kill(); launch.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(controlURL).MustConnect()
	defer browser.Close()

	for _, theme := range []string{"dark", "black-matrix", "light", "8bit", "cyberwar", "dark-sun", "galaxy", "lollipop", "ocean", "papyrus", "sandstorm", "threedee"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage("about:blank")
			defer page.Close()
			page.MustSetViewport(1280, 900, 1, false)
			page.MustNavigate(server.URL + "/?theme=" + theme).MustWaitLoad()
			result := page.MustEval("async () => {\n" +
				"appendMessage('assistant', 'Run this:\\n\\n```bash\\necho one\\necho two\\n```\\n\\nDone.');\n" +
				"appendMessage('assistant', '```mermaid\\ngraph TD; A-->B\\n```\\n\\n```chart\\n{\"type\":\"bar\",\"data\":{}}\\n```');\n" +
				`const bubble=chatContent.querySelector('.bubble'), wrappers=bubble.querySelectorAll('.code-block-wrapper');
				const wrapper=wrappers[0], pre=wrapper && wrapper.querySelector('.code-body pre'), code=pre && pre.querySelector('code');
				const numbers=wrapper && wrapper.querySelector('.line-numbers'), header=wrapper && wrapper.querySelector('.code-header');
				const button=header && header.querySelector('button.copy-btn');
				if(button)button.click();
				await new Promise(r=>setTimeout(r,20));
				const lineHeight=pre?parseFloat(getComputedStyle(pre).lineHeight)||parseFloat(getComputedStyle(code).lineHeight):0;
				const box=el=>el?Math.round(el.getBoundingClientRect().height):0;
				const lineCenters=el=>{if(!el)return [];const range=document.createRange();range.selectNodeContents(el);
					return [...new Set([...range.getClientRects()].filter(r=>r.height>0).map(r=>Math.round((r.top+r.bottom)/2)))].sort((a,b)=>a-b)};
				const second=chatContent.querySelectorAll('.bubble')[1];
				return {
					wrappers:wrappers.length, nested:!!(wrapper&&wrapper.closest('pre')), nestedPre:bubble.querySelectorAll('pre pre').length,
					outerPre:bubble.querySelectorAll(':scope > pre').length,
					code:code?code.textContent:'', highlighted:!!(code&&code.querySelector('[class^="hljs-"]')),
					numbers:numbers?numbers.innerText:'', lang:header?(header.querySelector('.code-lang')||{}).textContent:'',
					wrapperHeight:box(wrapper), headerHeight:box(header), preHeight:box(pre), numbersHeight:box(numbers), lineHeight,
					numberLines:lineCenters(numbers), codeLines:lineCenters(code),
					button:!!button, copied:__copied.slice(),
					mermaidInPre:!!(second.querySelector('.mermaid-raw')||document.body).closest('pre'), mermaid:second.querySelectorAll('.mermaid-raw').length,
					chartInPre:!!(second.querySelector('.chart-raw')||document.body).closest('pre'), chart:second.querySelectorAll('.chart-raw').length,
					errors:__errors
				};
			}`).Map()
			t.Logf("%s: %v", theme, result)
			if result["wrappers"].Int() != 1 || result["nested"].Bool() || result["nestedPre"].Int() != 0 || result["outerPre"].Int() != 0 {
				t.Errorf("fenced block must render as one code-block-wrapper outside any outer <pre>: %v", result)
			}
			if got := result["code"].Str(); got != "echo one\necho two" {
				t.Errorf("code text = %q, want the two fenced lines without a trailing blank line", got)
			}
			if got := result["numbers"].Str(); got != "1\n2" {
				t.Errorf("line numbers = %q, want one stacked number per code line", got)
			}
			if result["lang"].Str() != "bash" || !result["highlighted"].Bool() {
				t.Errorf("language label or hljs highlighting missing: %v", result)
			}
			lineHeight := result["lineHeight"].Num()
			if lineHeight <= 0 || result["preHeight"].Num() > 2*lineHeight+40 || result["numbersHeight"].Num() > result["preHeight"].Num()+2 {
				t.Errorf("code body is taller than its two lines: %v", result)
			}
			if result["wrapperHeight"].Num() > result["headerHeight"].Num()+result["preHeight"].Num()+4 {
				t.Errorf("code block carries empty bands beyond header and code: %v", result)
			}
			numberLines, codeLines := result["numberLines"].Arr(), result["codeLines"].Arr()
			if len(numberLines) != 2 || len(codeLines) != 2 {
				t.Errorf("want two rendered lines in the number column and the code: %v / %v", numberLines, codeLines)
			} else {
				for i := range numberLines {
					if diff := numberLines[i].Num() - codeLines[i].Num(); diff > 2 || diff < -2 {
						t.Errorf("line number %d sits %.0fpx off its code line: %v / %v", i+1, diff, numberLines, codeLines)
					}
				}
			}
			if copied := result["copied"].Arr(); !result["button"].Bool() || len(copied) != 1 || copied[0].Str() != "echo one\necho two" {
				t.Errorf("copy button must exist and copy the exact code: %v", result)
			}
			if result["mermaid"].Int() != 1 || result["mermaidInPre"].Bool() || result["chart"].Int() != 1 || result["chartInPre"].Bool() {
				t.Errorf("Mermaid and chart fences must keep their raw blocks outside an outer <pre>: %v", result)
			}
			if len(result["errors"].Arr()) > 0 {
				t.Errorf("browser errors: %v", result["errors"])
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "chat-code-block-"+theme+".png"), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
