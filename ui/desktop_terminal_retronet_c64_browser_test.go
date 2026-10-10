package ui

import (
	_ "embed"
	"strconv"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// The shared Retro-Net fixture plus one C64 board, a CRT background record and the hint-hold, key and font helpers.
//
//go:embed testdata/terminal-retronet-c64-fixture.js
var terminalRetroNetC64Fixture string

// A C64 call in the real Terminal (Contract C): 40x25 with the C64 font, the temporary commodore64 look, the key
// hint and its hold, colour keys, the bell and the restore on every way out; fake sockets and a fake API.
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

// The C64 colour keys (Contract C) in the order the test presses them and the PETSCII code each one sends.
var terminalC64ColourKeys = []struct {
	mods  []input.Key
	digit input.Key
	code  int
}{
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit1, 0x90},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit2, 0x05},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit3, 0x1C},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit4, 0x9F},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit5, 0x9C},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit6, 0x1E},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit7, 0x1F},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit8, 0x9E},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit1, 0x81},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit2, 0x95},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit3, 0x96},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit4, 0x97},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit5, 0x98},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit6, 0x99},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit7, 0x9A},
	{[]input.Key{input.AltLeft, input.ShiftLeft}, input.Digit8, 0x9B},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit9, 0x12},
	{[]input.Key{input.ControlLeft, input.ShiftLeft}, input.Digit0, 0x92},
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
	press := func(mods []input.Key, key input.Key) {
		page.KeyActions().Press(mods...).Type(key).MustDo()
	}
	ctrlShift := []input.Key{input.ControlLeft, input.ShiftLeft}
	altShift := []input.Key{input.AltLeft, input.ShiftLeft}
	// connect sends board output (`data`, shown as `marker`) before CONNECT, skips an animated dial with a local key
	// and waits until CONNECT shows the Telnet notice and the key hint while that output waits for the hold.
	connect := func(name, data, marker string) {
		c.run(`()=>{fixtureWatchConnect();fixtureData('` + data + `');fixtureControl({type:'connected',protocol:'telnet',kind:'c64',charset:'petscii'});}`)
		if full {
			c.check(name+": still dialling", `()=>fixtureRoot().dataset.terminalMode==='dialing'`)
			c.run(`async()=>{await fixtureInput('x');}`)
		}
		c.wait(name+": CONNECT holds the Telnet notice and the key hint, the board output waits", `async()=>fixtureRoot().dataset.terminalMode==='retro'&&await fixtureHintShown('`+marker+`')`)
		c.check(name+": still held a second after CONNECT", `async()=>{await fixtureAfterConnect(1000);return fixtureHintShown('`+marker+`');}`)
	}
	// dial opens the C64 board from the directory, where it is the last entry and preselected.
	dial := func(name string) {
		c.wait(name+": the directory is back", `async()=>fixtureRoot().dataset.terminalMode==='directory'&&(await fixtureSelectedLine()).includes('Test C64 Board')`)
		c.run(`async()=>{fixtureTerm().focus();await fixtureInput('3');await fixtureInput('6');await fixtureInput('\r');}`)
		c.wait(name+": the C64 call opens", `()=>fixtureSocket().readyState===1&&new URL(fixtureSocket().url).searchParams.get('entry')==='c64-test'&&fixtureRoot().dataset.terminalStyle==='commodore64'&&fixtureRoot().dataset.terminalMode==='dialing'`)
	}
	// freshScreen: the PETSCII engine's screen after the hold, blank but for `marker` at row 25.
	freshScreen := `(marker)=>{const lines=fixtureLines();return lines[24]===marker&&lines.slice(0,24).every(l=>l==='');}`

	// 1. Directory: the C64 board under its own category, marked [C], explained in the help, announced as not checked.
	c.wait("directory rendered and stale status refreshed", `async()=>(await fixtureText()).includes(t('desktop.terminal_retronet_title'))&&fixtureCalls.some(c=>c.path==='/api/desktop/retronet/status'&&c.method==='POST')`)
	c.run(`async()=>{fixtureTerm().focus();await fixtureInput('3');await fixtureInput('6');}`)
	c.check("the C64 board shows [C] under Commodore 64 and is announced as not checked", `async()=>{const line=await fixtureSelectedLine(),text=await fixtureText(),live=fixtureRoot().querySelector('[data-terminal-announce]').textContent;
		return line.startsWith('>36')&&line.includes('Test C64 Board')&&line.slice(4,7)==='[C]'&&
			text.includes(t('desktop.terminal_retronet_cat_commodore').toLocaleUpperCase())&&text.includes(t('desktop.terminal_retronet_status_c64_help'))&&
			live.includes(t('desktop.terminal_retronet_status_c64'));}`)

	// 2. Dial: 40x25 in the C64 font, the commodore64 look without saving it, the style select locked and described.
	c.run(`async()=>{await fixtureInput('\r');}`)
	c.wait("a C64 board dials with a fixed 40x25 grid", `()=>{const s=fixtureSocket(),u=new URL(s.url),term=fixtureTerm(),root=fixtureRoot();
		return u.searchParams.get('entry')==='c64-test'&&u.searchParams.get('cols')==='40'&&u.searchParams.get('rows')==='25'&&
			term.cols===40&&term.rows===25&&root.dataset.terminalGeometry==='c64'&&term.options.fontFamily.startsWith('"Aura C64"')&&
			root.dataset.terminalMode==='dialing';}`)
	c.wait("the commodore64 look is on, unsaved, with the style select locked", `()=>{const root=fixtureRoot(),term=fixtureTerm(),select=root.querySelector('select[data-terminal-style]'),
			locked=t('desktop.terminal_retronet_style_locked'),note=document.getElementById(select.getAttribute('aria-describedby')||'');
		return root.dataset.terminalStyle==='commodore64'&&select.value==='commodore64'&&select.disabled&&select.title===locked&&
			root.querySelector('.vd-terminal-style-label').title===locked&&localStorage.getItem('aurago.desktop.terminal.style')==='amber'&&
			!!note&&note.textContent===locked&&note.classList.contains('vd-sr-only')&&root.contains(note)&&
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

	// 3. A colour key while dialling stays local. Board output buffered before CONNECT waits while CONNECT, the Telnet
	// notice and the key hint (packed into 40 columns) stay readable; a key ends that hold at once and still goes out.
	c.wait("C64 socket open", `()=>fixtureSocket().readyState===1`)
	c.run(`()=>fixtureTerm().focus()`)
	press(ctrlShift, input.Digit1)
	c.check("the colour key reached the terminal while dialling", `()=>fixtureKeys.at(-1)==='CS+Digit1'&&fixtureRoot().dataset.terminalMode==='dialing'`)
	connect("first call", `\x1b[38;2;112;109;235mBOARD ONE`, "BOARD ONE")
	c.check("CONNECT, then every part of the key hint whole", `async()=>{await fixtureFlush();const lines=fixtureLines(),text=lines.join('\n'),parts=t('desktop.terminal_retronet_c64_keys').split(' · '),
			connect=lines.findIndex(l=>l.startsWith('CONNECT ')),notice=lines.indexOf(fixtureWrapped('desktop.terminal_retronet_telnet_notice')[0]),hint=lines.findIndex(l=>l.startsWith(parts[0]));
		return connect>=0&&notice>connect&&hint>notice&&parts.every(p=>text.includes(p))&&!text.includes('…');}`)
	c.check("CONNECT announces the Telnet notice, the C64 keys and the locked style, Ctrl+] only once", `()=>{const live=fixtureRoot().querySelector('[data-terminal-announce]').textContent;
		return live.startsWith(t('desktop.terminal_retronet_telnet_notice'))&&live.includes(t('desktop.terminal_retronet_c64_keys'))&&
			live.includes(t('desktop.terminal_retronet_style_locked'))&&!live.includes(t('desktop.terminal_retronet_hangup_hint'))&&live.split('Strg+]').length===2;}`)
	c.check("neither the colour key nor the skip key reached the board", `()=>fixtureSent(fixtureSocket()).length===0`)
	c.check("a key ends the hold at once: it reaches the board and the board output lands on the fresh screen", `async()=>{const early=performance.now()-fixtureConnectAt<1900;
		await fixtureInput('A');await fixtureFlush();const b=fixtureTerm().buffer.active;
		return early&&fixtureSent(fixtureSocket()).join('|')==='bin:A'&&(`+freshScreen+`)('BOARD ONE')&&b.cursorY===24&&b.cursorX===9;}`)

	// 4. The PETSCII engine owns the 40x25 screen from the first board byte: it started blank with the cursor at row 25,
	// column 1 (above), and every glyph lands where the engine's cursor positioning puts it. Its repaints come as
	// synchronized-output blocks (ESC[?2026h ... ESC[?2026l), which the vendored xterm 6 supports (DEC mode 2026).
	c.run(`()=>fixtureData('\x1b[?2026h\x1b[1;1H'+String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)+'\x1b[3;5H'+String.fromCharCode(0xee01))`)
	c.wait("an open synchronized block switches xterm's synchronized output on", `async()=>{await fixtureFlush();return fixtureTerm().modes.synchronizedOutputMode===true;}`)
	c.run(`()=>fixtureData('\x1b[?2026l')`)
	c.wait("board glyphs land where the PETSCII engine put them once the block ends", `async()=>{await fixtureFlush();const lines=fixtureLines();
		return fixtureTerm().modes.synchronizedOutputMode===false&&lines[0]===String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)&&
			lines[2]==='    '+String.fromCharCode(0xee01)&&lines[1]===''&&lines.slice(3,24).every(l=>l==='')&&lines[24]==='BOARD ONE';}`)

	// 5. Keys: plain keys go out unchanged; all 18 colour keys only as the private sequence, nothing else.
	c.run(`async()=>{window.fixtureSentMark=fixtureSent(fixtureSocket()).length;fixtureTerm().focus();await fixtureInput('A');await fixtureInput('\x7f');}`)
	for _, key := range terminalC64ColourKeys {
		press(key.mods, key.digit)
	}
	codes := ""
	for i, key := range terminalC64ColourKeys {
		if i > 0 {
			codes += ","
		}
		codes += strconv.Itoa(key.code)
	}
	c.wait("colour keys send ESC[64;n~ and nothing else", `()=>fixtureSent(fixtureSocket()).slice(fixtureSentMark).join('|')===['bin:A','bin:\x7f',...[`+codes+`].map(n=>'bin:\x1b[64;'+n+'~')].join('|')`)

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

	// 7. The look stays through the result; leaving the result restores the style, cursor, fonts and fit, and never
	// puts the C64 font back on the way out.
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.wait("Ctrl+] hangs up; the result keeps the C64 screen", `async()=>{const text=await fixtureText(),root=fixtureRoot();return text.includes('NO CARRIER')&&root.dataset.terminalMode==='result'&&
		root.dataset.terminalGeometry==='c64'&&root.dataset.terminalStyle==='commodore64'&&root.querySelector('select[data-terminal-style]').disabled&&fixtureLive().length===0;}`)
	c.check("the result lines wrap between words inside 39 columns", `async()=>{await fixtureFlush();const lines=fixtureLines();
		return fixtureWrapped('desktop.terminal_retronet_press_key').length>1&&fixtureWrapped('desktop.terminal_retronet_press_key').every(l=>lines.includes(l))&&
			fixtureWrapped('desktop.terminal_retronet_result_local_hangup').every(l=>lines.includes(l));}`)
	c.run(`async()=>{fixtureWatchFonts();await fixtureDismiss('q');}`)
	c.wait("back in the directory with the user's look", `async()=>{const root=fixtureRoot(),term=fixtureTerm(),select=root.querySelector('select[data-terminal-style]'),amber=TerminalStyles.profile('amber');
		return root.dataset.terminalMode==='directory'&&(await fixtureSelectedLine()).includes('Test C64 Board')&&!root.hasAttribute('data-terminal-geometry')&&
			root.dataset.terminalStyle==='amber'&&select.value==='amber'&&!select.disabled&&!select.title&&!root.querySelector('.vd-terminal-style-label').title&&
			!select.hasAttribute('aria-describedby')&&[...root.querySelectorAll('.vd-sr-only')].every(n=>n.textContent!==t('desktop.terminal_retronet_style_locked'))&&
			term.options.fontFamily===amber.fontFamily&&term.options.theme.background===amber.theme.background&&term.options.cursorBlink===false&&
			!term.element.style.paddingLeft&&!(term.cols===40&&term.rows===25)&&fixtureCrtBackgrounds.at(-1)===amber.theme.background&&
			localStorage.getItem('aurago.desktop.terminal.style')==='amber';}`)
	c.check("leaving the C64 look switches straight to the user's font", `()=>fixtureFonts.length>0&&!fixtureFonts.some(f=>f.includes('Aura C64'))`)

	// 8. Without a key the held board output follows the hold. Then the server ends the call because Retro-Net was
	// switched off: the next key opens the shell in the user's look.
	dial("second call")
	connect("second call", "BOARD TWO", "BOARD TWO")
	c.wait("without a key the board output follows the hold on the fresh screen", `async()=>{await fixtureFlush();return (`+freshScreen+`)('BOARD TWO')&&performance.now()-fixtureConnectAt>=1950;}`)
	c.check("nothing went to the board", `()=>fixtureSent(fixtureSocket()).length===0`)
	c.run(`()=>{fixtureControl({type:'result',code:'NO CARRIER',reason:'disabled'});fixtureSocket().serverClose(1000);}`)
	c.wait("the disabled result offers the shell", `async()=>{await fixtureFlush();const lines=fixtureLines();
		return fixtureWrapped('desktop.terminal_retronet_press_key_shell').every(l=>lines.includes(l))&&fixtureRoot().dataset.terminalMode==='result';}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)
	c.wait("the shell opens in the user's look", `()=>{const root=fixtureRoot();return root.dataset.terminalMode==='shell'&&root.dataset.terminalStyle==='amber'&&
		!root.hasAttribute('data-terminal-geometry')&&!root.querySelector('select[data-terminal-style]').disabled&&fixtureTerm().options.cursorBlink===false;}`)
	// Colour keys and the bell belong to C64 calls: in the shell, with sound on, they send no C64 sequence and never beep.
	c.wait("shell socket open", `()=>fixtureSocket().readyState===1&&new URL(fixtureSocket().url).pathname==='/api/code-studio/terminal'`)
	c.run(`()=>{fixtureRoot().querySelector('[data-terminal-audio]').click();window.fixtureBeepMark=fixtureOscillators;fixtureTerm().focus();}`)
	press(ctrlShift, input.Digit1)
	press(altShift, input.Digit8)
	c.run(`()=>fixtureData('\x07')`)
	c.check("the shell sends no C64 sequence and its BEL never beeps", `async()=>{await fixtureFlush();await new Promise(r=>setTimeout(r,200));
		return fixtureKeys.filter(k=>k.includes('Digit')).slice(-2).join()==='CS+Digit1,AS+Digit8'&&localStorage.getItem('aurago.desktop.terminal.audioMuted')==='0'&&
			!fixtureSent(fixtureSocket()).some(f=>f.includes('\x1b[64;'))&&fixtureOscillators===fixtureBeepMark;}`)
	c.run(`()=>fixtureRoot().querySelector('[data-terminal-audio]').click()`)

	// 9. Every other way out of the hold. A board that hangs up during the hold: its output is shown before the result.
	c.run(`async()=>{await fixtureBoot(true);}`)
	c.wait("a fresh window shows the directory", `async()=>fixtureRoot().dataset.terminalMode==='directory'&&(await fixtureText()).includes(t('desktop.terminal_retronet_title'))&&fixtureLive().length===0`)
	dial("third call")
	connect("third call", "BOARD THREE", "BOARD THREE")
	c.run(`()=>{fixtureControl({type:'result',code:'NO CARRIER',reason:'remote_closed'});fixtureSocket().serverClose(1000);}`)
	c.wait("the held output is shown before the result", `async()=>{await fixtureFlush();const lines=fixtureLines(),board=lines.indexOf('BOARD THREE'),code=lines.indexOf('NO CARRIER');
		return fixtureRoot().dataset.terminalMode==='result'&&board>=0&&code>board&&!lines.some(l=>l.startsWith(t('desktop.terminal_retronet_c64_keys').split(' · ')[0]));}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// A local hang-up during the hold drops the held output and the hold never writes over the result.
	dial("fourth call")
	connect("fourth call", "BOARD FOUR", "BOARD FOUR")
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.check("Ctrl+] hangs up during the hold and is never sent", `async()=>{await fixtureFlush();return fixtureRoot().dataset.terminalMode==='result'&&fixtureLines().includes('NO CARRIER')&&fixtureSent(fixtureSocket()).length===0;}`)
	c.check("after the hold time the result is untouched and the held output dropped", `async()=>{await fixtureAfterConnect(2400);await fixtureFlush();const lines=fixtureLines();
		return lines.includes('NO CARRIER')&&!lines.some(l=>l.includes('BOARD FOUR'))&&fixtureRoot().dataset.terminalMode==='result';}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// A colour key ends the hold like any key.
	dial("fifth call")
	connect("fifth call", "BOARD FIVE", "BOARD FIVE")
	c.run(`()=>fixtureTerm().focus()`)
	press(ctrlShift, input.Digit1)
	c.check("a colour key ends the hold at once and is sent", `async()=>{const early=performance.now()-fixtureConnectAt<1900;await fixtureFlush();
		return early&&fixtureSent(fixtureSocket()).join('|')==='bin:\x1b[64;144~'&&(`+freshScreen+`)('BOARD FIVE');}`)
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.wait("the fifth call hangs up", `()=>fixtureRoot().dataset.terminalMode==='result'&&fixtureLive().length===0`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// More held output than the throttle may queue ends the hold early; closing the window during the call leaves
	// the saved style untouched.
	dial("sixth call")
	connect("sixth call", "BOARD SIX", "BOARD SIX")
	c.check("a full queue ends the hold at once", `async()=>{const early=performance.now()-fixtureConnectAt<1900;
		fixtureData('\x1b[0m'.repeat(TerminalModem.MAX_QUEUE_BYTES/4+1));await fixtureFlush();
		return early&&(`+freshScreen+`)('BOARD SIX');}`)
	c.run(`()=>TerminalApp.dispose()`)
	c.check("dispose closes the call and the C64 look was never saved", `()=>fixtureLive().length===0&&localStorage.getItem('aurago.desktop.terminal.style')==='amber'`)
	c.check("no browser errors and never two live sockets", `()=>fixtureErrors.length===0`)
}
