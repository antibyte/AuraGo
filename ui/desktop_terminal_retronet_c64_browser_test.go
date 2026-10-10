package ui

import (
	_ "embed"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// The shared Retro-Net fixture plus one C64 board and a CRT background record.
//
//go:embed testdata/terminal-retronet-c64-fixture.js
var terminalRetroNetC64Fixture string

// A C64 call in the real Terminal (Contract C): 40x25 with the C64 font, the temporary commodore64 look, the key
// hint, colour keys, the bell and the restore on every way out; fake sockets and a fake API.
func TestDesktopTerminalRetroNetC64Browser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	srv := newTerminalDesktopServer(t, terminalRetroNetFixture+"\n"+terminalRetroNetC64Fixture, nil)
	browser := newTerminalBrowser(t, "1")
	for _, motion := range []string{"full", "reduce"} {
		t.Run(motion, func(t *testing.T) {
			runTerminalRetroNetC64Browser(t, browser, srv.URL, motion)
		})
	}
}

func runTerminalRetroNetC64Browser(t *testing.T, browser *rod.Browser, base, motion string) {
	page := browser.MustPage().Timeout(240 * time.Second)
	defer page.Close()
	preference := "no-preference"
	if motion == "reduce" {
		preference = "reduce"
	}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: preference}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(1280, 850, 1, false)
	page.MustNavigate(base + "/fixture?motion=" + motion).MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	full := motion == "full"
	c := terminalChecks{t: t, page: page}

	// 1. Directory: the C64 board under its own category, marked [C], explained in the help, announced as not checked.
	c.wait("directory rendered and stale status refreshed", `async()=>(await fixtureText()).includes(t('desktop.terminal_retronet_title'))&&fixtureCalls.some(c=>c.path==='/api/desktop/retronet/status'&&c.method==='POST')`)
	c.run(`async()=>{fixtureTerm().focus();await fixtureInput('3');await fixtureInput('6');}`)
	c.check("the C64 board shows [C] under Commodore 64 and is announced as not checked", `async()=>{const line=await fixtureSelectedLine(),text=await fixtureText(),live=fixtureRoot().querySelector('[data-terminal-announce]').textContent;
		return line.startsWith('>36')&&line.includes('Test C64 Board')&&line.slice(4,7)==='[C]'&&
			text.includes(t('desktop.terminal_retronet_cat_commodore').toLocaleUpperCase())&&text.includes(t('desktop.terminal_retronet_status_c64_help'))&&
			live.includes(t('desktop.terminal_retronet_status_c64'));}`)

	// 2. Dial: 40x25 in the C64 font, the commodore64 look without saving it, the style select locked.
	c.run(`async()=>{await fixtureInput('\r');}`)
	c.wait("a C64 board dials with a fixed 40x25 grid", `()=>{const s=fixtureSocket(),u=new URL(s.url),term=fixtureTerm(),root=fixtureRoot();
		return u.searchParams.get('entry')==='c64-test'&&u.searchParams.get('cols')==='40'&&u.searchParams.get('rows')==='25'&&
			term.cols===40&&term.rows===25&&root.dataset.terminalGeometry==='c64'&&term.options.fontFamily.startsWith('"Aura C64"')&&
			root.dataset.terminalMode==='dialing';}`)
	c.wait("the commodore64 look is on, unsaved, with the style select locked", `()=>{const root=fixtureRoot(),term=fixtureTerm(),select=root.querySelector('select[data-terminal-style]'),
			locked=t('desktop.terminal_retronet_style_locked');
		return root.dataset.terminalStyle==='commodore64'&&select.value==='commodore64'&&select.disabled&&select.title===locked&&
			root.querySelector('.vd-terminal-style-label').title===locked&&localStorage.getItem('aurago.desktop.terminal.style')==='amber'&&
			term.options.theme.background==='#000000'&&term.options.cursorStyle==='block'&&term.options.cursorBlink===true&&
			fixtureCrtBackgrounds.at(-1)==='#706DEB';}`)
	// The fit settles over two or three frames after the font has loaded.
	c.wait("the C64 font fills the glass, centred in the C64 border colour, on a black screen", `async()=>{
		const loaded=(await document.fonts.load('16px "Aura C64"')).some(f=>f.status==='loaded');
		await new Promise(r=>setTimeout(r,300));
		const term=fixtureTerm(),grid=term.element.querySelector('.xterm-screen'),scr=grid.getBoundingClientRect(),
			glass=fixtureRoot().querySelector('.vd-terminal-screen'),g=glass.getBoundingClientRect();
		return loaded&&Number.isInteger(term.options.fontSize*2)&&scr.width<=g.width+0.5&&scr.height<=g.height+0.5&&
			Math.max(scr.width/g.width,scr.height/g.height)>0.75&&
			Math.abs((scr.left-g.left)-(g.right-scr.right))<=2&&Math.abs((scr.top-g.top)-(g.bottom-scr.bottom))<=2&&
			getComputedStyle(glass).backgroundColor==='rgb(112, 109, 235)'&&grid.style.background==='rgb(0, 0, 0)';
	}`)

	// 3. CONNECT, the Telnet notice and the C64 key hint, packed into 40 columns; the skip key stays local.
	c.wait("C64 socket open", `()=>fixtureSocket().readyState===1`)
	c.run(`()=>fixtureControl({type:'connected',protocol:'telnet',kind:'c64',charset:'petscii'})`)
	if full {
		c.run(`async()=>{await fixtureInput('x');}`)
	}
	c.wait("CONNECT, then every part of the key hint whole", `async()=>{await fixtureFlush();const lines=fixtureLines(),text=lines.join('\n'),parts=t('desktop.terminal_retronet_c64_keys').split(' · '),
			connect=lines.findIndex(l=>l.startsWith('CONNECT ')),hint=lines.findIndex(l=>l.startsWith(parts[0]));
		return fixtureRoot().dataset.terminalMode==='retro'&&connect>=0&&hint>connect&&parts.every(p=>text.includes(p))&&!text.includes('…')&&
			fixtureRoot().querySelector('[data-terminal-announce]').textContent.includes(parts[0]);}`)
	c.check("the skip key never reached the board", `()=>fixtureSent(fixtureSocket()).length===0`)

	// 4. The PETSCII engine owns the 40x25 screen from the first board byte: it starts blank with the cursor at row 25,
	// column 1, and every glyph lands where the engine's cursor positioning puts it. Its repaints come as
	// synchronized-output blocks (ESC[?2026h ... ESC[?2026l), which the vendored xterm 6 supports (DEC mode 2026).
	c.run(`()=>fixtureData('\x1b[38;2;112;109;235m')`)
	c.wait("the first board byte clears the local lines and parks the cursor at row 25", `async()=>{await fixtureFlush();const b=fixtureTerm().buffer.active;
		return fixtureLines().every(l=>l==='')&&b.cursorY===24&&b.cursorX===0;}`)
	c.run(`()=>fixtureData('\x1b[?2026h\x1b[1;1H'+String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)+'\x1b[3;5H'+String.fromCharCode(0xee01))`)
	c.wait("an open synchronized block switches xterm's synchronized output on", `async()=>{await fixtureFlush();return fixtureTerm().modes.synchronizedOutputMode===true;}`)
	c.run(`()=>fixtureData('\x1b[?2026l')`)
	c.wait("board glyphs land where the PETSCII engine put them once the block ends", `async()=>{await fixtureFlush();const lines=fixtureLines();
		return fixtureTerm().modes.synchronizedOutputMode===false&&lines[0]===String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)&&
			lines[2]==='    '+String.fromCharCode(0xee01)&&lines[1]===''&&lines.slice(3).every(l=>l==='');}`)

	// 5. Keys: plain keys go out unchanged; colour keys only as the private sequence, nothing else.
	c.run(`async()=>{fixtureTerm().focus();await fixtureInput('A');await fixtureInput('\x7f');}`)
	page.KeyActions().Press(input.ControlLeft, input.ShiftLeft).Type(input.Digit1).MustDo()
	page.KeyActions().Press(input.AltLeft, input.ShiftLeft).Type(input.Digit8).MustDo()
	page.KeyActions().Press(input.ControlLeft, input.ShiftLeft).Type(input.Digit9).MustDo()
	page.KeyActions().Press(input.ControlLeft, input.ShiftLeft).Type(input.Digit0).MustDo()
	c.wait("colour keys send ESC[64;n~ and nothing else", `()=>fixtureSent(fixtureSocket()).join('|')==='bin:A|bin:\x7f|bin:\x1b[64;144~|bin:\x1b[64;155~|bin:\x1b[64;18~|bin:\x1b[64;146~'`)

	// 6. Bell: a short tone under the key-click rules, so only without reduced motion, and at most one per 150 ms
	// however many BELs arrive: a burst right behind the first BEL adds no tone.
	c.run(`()=>{fixtureRoot().querySelector('[data-terminal-audio]').click();window.fixtureBeepMark=fixtureOscillators;fixtureData('\x07');fixtureData('\x07\x07\x07');}`)
	if full {
		c.wait("BEL beeps once", `async()=>{await fixtureFlush();return fixtureOscillators===fixtureBeepMark+1;}`)
		c.check("a BEL burst within 150 ms adds no tone", `async()=>{await fixtureFlush();await new Promise(r=>setTimeout(r,100));return fixtureOscillators===fixtureBeepMark+1;}`)
	} else {
		c.check("reduced motion never beeps", `async()=>{await fixtureFlush();await new Promise(r=>setTimeout(r,150));return fixtureOscillators===fixtureBeepMark;}`)
	}
	c.run(`()=>fixtureRoot().querySelector('[data-terminal-audio]').click()`)

	// 7. The look stays through the result; leaving the result restores the style, cursor, fonts and fit.
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.wait("Ctrl+] hangs up; the result keeps the C64 screen", `async()=>{const text=await fixtureText(),root=fixtureRoot();return text.includes('NO CARRIER')&&root.dataset.terminalMode==='result'&&
		root.dataset.terminalGeometry==='c64'&&root.dataset.terminalStyle==='commodore64'&&root.querySelector('select[data-terminal-style]').disabled&&fixtureLive().length===0;}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)
	c.wait("back in the directory with the user's look", `async()=>{const root=fixtureRoot(),term=fixtureTerm(),select=root.querySelector('select[data-terminal-style]'),amber=TerminalStyles.profile('amber');
		return root.dataset.terminalMode==='directory'&&(await fixtureSelectedLine()).includes('Test C64 Board')&&!root.hasAttribute('data-terminal-geometry')&&
			root.dataset.terminalStyle==='amber'&&select.value==='amber'&&!select.disabled&&!select.title&&!root.querySelector('.vd-terminal-style-label').title&&
			term.options.fontFamily===amber.fontFamily&&term.options.theme.background===amber.theme.background&&term.options.cursorBlink===false&&
			!term.element.style.paddingLeft&&!(term.cols===40&&term.rows===25)&&fixtureCrtBackgrounds.at(-1)===amber.theme.background&&
			localStorage.getItem('aurago.desktop.terminal.style')==='amber';}`)

	// 8. A call the server ends because Retro-Net was switched off: the next key opens the shell in the user's look.
	c.run(`async()=>{await fixtureInput('3');await fixtureInput('6');await fixtureInput('\r');}`)
	c.wait("a second C64 call opens", `()=>fixtureSocket().readyState===1&&new URL(fixtureSocket().url).searchParams.get('entry')==='c64-test'&&fixtureRoot().dataset.terminalStyle==='commodore64'`)
	c.run(`()=>{fixtureControl({type:'connected',protocol:'telnet',kind:'c64',charset:'petscii'});fixtureControl({type:'result',code:'NO CARRIER',reason:'disabled'});fixtureSocket().serverClose(1000);}`)
	c.wait("the disabled result offers the shell", `async()=>(await fixtureFlat()).includes(t('desktop.terminal_retronet_press_key_shell'))&&fixtureRoot().dataset.terminalMode==='result'`)
	c.run(`async()=>{await fixtureDismiss('q');}`)
	c.wait("the shell opens in the user's look", `()=>{const root=fixtureRoot();return root.dataset.terminalMode==='shell'&&root.dataset.terminalStyle==='amber'&&
		!root.hasAttribute('data-terminal-geometry')&&!root.querySelector('select[data-terminal-style]').disabled&&fixtureTerm().options.cursorBlink===false;}`)

	// 9. Closing the window during a C64 call leaves the saved style untouched.
	c.run(`async()=>{await fixtureBoot(true);}`)
	c.wait("a fresh window shows the directory", `async()=>fixtureRoot().dataset.terminalMode==='directory'&&(await fixtureText()).includes(t('desktop.terminal_retronet_title'))&&fixtureLive().length===0`)
	c.run(`async()=>{fixtureTerm().focus();await fixtureInput('3');await fixtureInput('6');await fixtureInput('\r');}`)
	c.wait("a third C64 call opens", `()=>fixtureRoot().dataset.terminalStyle==='commodore64'&&fixtureLive().length===1`)
	c.run(`()=>TerminalApp.dispose()`)
	c.check("dispose closes the call and the C64 look was never saved", `()=>fixtureLive().length===0&&localStorage.getItem('aurago.desktop.terminal.style')==='amber'`)
	c.check("no browser errors and never two live sockets", `()=>fixtureErrors.length===0`)
}
