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

// A C64 call in the real Terminal (Contract C): 40x25 with the C64 font scaled by whole pixels without seams (also
// at DPR 1.25 and 1.5), the temporary commodore64 look, the key hint and its hold, colour keys, the bell and the
// restore on every way out; fake sockets and a fake API.
func TestDesktopTerminalRetroNetC64Browser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	srv := newTerminalDesktopServer(t, terminalRetroNetFixture+"\n"+terminalRetroNetC64Fixture, nil)
	browser := newTerminalBrowser(t, "1")
	for _, motion := range []string{"full", "reduce"} {
		t.Run(motion, func(t *testing.T) {
			runTerminalRetroNetC64Browser(t, browser, srv.URL, motion)
		})
	}
	// Fractional DPRs: at 1.25 xterm's advance comes out one device px short (letter spacing restores it), at 1.5
	// the 960x720 window gets k = 3 (24 px cells) with no correction.
	for _, dpr := range []string{"1.25", "1.5"} {
		t.Run("dpr-"+dpr, func(t *testing.T) {
			scale, _ := strconv.ParseFloat(dpr, 64)
			page := newTerminalBrowser(t, dpr).MustPage().Timeout(120 * time.Second)
			defer page.Close()
			page.MustSetViewport(1280, 850, scale, false)
			page.MustNavigate(srv.URL + "/fixture?motion=reduce").MustWaitLoad()
			page.MustEval(`async()=>{await fixtureReady;}`)
			c := terminalChecks{t: t, page: page}
			c.wait("directory rendered", `async()=>(await fixtureText()).includes(t('desktop.terminal_retronet_title'))`)
			c.run(`async()=>{fixtureTerm().focus();await fixtureInput('3');await fixtureInput('6');await fixtureInput('\r');}`)
			c.wait("the C64 board dials", `()=>fixtureRoot().dataset.terminalGeometry==='c64'&&fixtureSocket().readyState===1`)
			c.wait("the C64 grid takes whole pixels at DPR "+dpr, terminalC64WholePixels(24))
			c.run(`()=>{fixtureExpectHold(true);fixtureControl({type:'connected',protocol:'telnet',kind:'c64',charset:'petscii'});}`)
			c.wait("CONNECT holds the board output", `()=>fixtureRoot().dataset.terminalMode==='retro'&&fixtureHolds.length===1`)
			c.run(`async()=>{await fixtureInput('A');await fixtureFlush();}`)
			c.run(`()=>fixtureData(` + terminalC64SeamScreen + `)`)
			c.check("half blocks show no seam at any cell edge at DPR "+dpr, terminalC64Seams)
			c.check("no browser errors", `()=>fixtureErrors.length===0`)
		})
	}
}

// Two bands of PETSCII half blocks in white on red, drawn as the PETSCII engine draws them (CUP, truecolour SGR,
// glyph): rows 1-3 the lower half block (U+EE62), rows 4-6 the left half block (U+EE61), all 40 columns.
const terminalC64SeamScreen = `(()=>{const E='\x1b[',band=(row,glyph)=>E+row+';1H'+E+'38;2;255;255;255m'+E+'48;2;136;0;0m'+String.fromCharCode(glyph).repeat(40);
	return band(1,0xee62)+band(2,0xee62)+band(3,0xee62)+band(4,0xee61)+band(5,0xee61)+band(6,0xee61)+E+'0m'+E+'25;1H';})()`

// Reads terminalC64SeamScreen back from xterm's WebGL canvas (preserveDrawingBuffer; test-only access to the device
// cell) in device px. The grid starts at the canvas top, or one row lower when the canvas is a px taller (WebGL
// anchors at the bottom); found from the red top half of the first cell, which a moved glyph cannot change.
// G is 255 on white and 0 on red; R below 120 is neither (a dark or colour-fringed pixel). Lower half band: each pixel
// row of the cell is all red above the middle and all white below, and no column is dimmer (a seam column). Left
// half band: all pixel rows of the cell alike (a glyph moved down shows as a different row); columns fully lit up
// to the middle and red after it, the edge columns at the cell edges and the middle only antialiased (lit side at
// least 75 %, unlit side at most 60 %: the browser's greyscale antialiasing softens vertical glyph edges). Problems
// are added to fixtureErrors so a failure prints them.
const terminalC64Seams = `async()=>{
	await fixtureFlush();
	await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,100))));
	const term=fixtureTerm(),dev=term._core._renderService.dimensions.device.cell,w=dev.width,h=dev.height,W=40*w,problems=[];
	const gl=[...term.element.querySelectorAll('.xterm-screen canvas')].find(c=>!c.classList.contains('xterm-link-layer'));
	const copy=document.createElement('canvas');copy.width=gl.width;copy.height=gl.height;
	const ctx=copy.getContext('2d');ctx.drawImage(gl,0,0);
	const data=ctx.getImageData(0,0,gl.width,gl.height).data,at=(x,y)=>(y*gl.width+x)*4;
	let y0=0;while(y0<gl.height&&!(data[at(2,y0)]>120&&data[at(2,y0)+1]<20))y0++;
	if(y0!==0&&y0!==gl.height-25*h)problems.push('grid top at '+y0+' of '+gl.height);
	const G=(x,y)=>data[at(x,y0+y)+1],range=(a,b)=>Array.from({length:b-a},(_,i)=>a+i),cols=range(0,W);
	const mean=(xs,ys)=>{let s=0;for(const y of ys)for(const x of xs)s+=G(x,y);return s/(xs.length*ys.length);};
	let dark=0;for(let y=0;y<6*h;y++)for(let x=0;x<W;x++)if(data[at(x,y0+y)]<120)dark++;
	if(dark)problems.push(dark+' dark px');
	const lower=range(0,3*h),upperRows=lower.filter(y=>y%h<h/2),lowerRows=lower.filter(y=>y%h>=h/2);
	for(let r=0;r<h;r++){const g=mean(cols,lower.filter(y=>y%h===r));if(r<h/2?g>8:g<247)problems.push('lower half block row '+r+': '+g.toFixed(1));}
	for(let x=1;x<W-1;x++){const top=mean([x],upperRows),bottom=mean([x],lowerRows);
		if(top>8||bottom<235)problems.push('lower half block column '+x+': '+top.toFixed(1)+'/'+bottom.toFixed(1));}
	const left=range(3*h,6*h),rows=range(0,h).map(r=>mean(cols,left.filter(y=>y%h===r)));
	if(Math.max(...rows)-Math.min(...rows)>6)problems.push('left half block rows '+rows.map(g=>g.toFixed(0)).join(','));
	for(let c=0;c<w;c++){const g=mean(cols.filter(x=>x%w===c),left);
		const ok=c===0||c===w/2-1?g>=191:c===w/2||c===w-1?g<=153:c<w/2?g>=247:g<=8;
		if(!ok)problems.push('left half block column '+c+': '+g.toFixed(1));}
	if(problems.length)fixtureErrors.push('C64 seams at DPR '+devicePixelRatio+', '+w+'x'+h+' cells: '+problems.slice(0,8).join('; '));
	return problems.length===0;
}`

// After a C64 dial the grid is scaled by whole device pixels (Contract C): xterm's cell (test-only access to its
// render dimensions) is `cell` device px square (the largest exact k for the fixture's 960x720 window: 16 at DPR 1,
// 24 at 1.25 and at 1.5, where k = 4 gets a 33 px font box), equals the font size times the DPR and, in CSS px times
// the DPR, a multiple of 8, with line height 1; the grid is centred in the C64 border colour on a black screen and
// drawn from xterm's alpha glyph atlas (no LCD fringe over the neighbouring cell).
func terminalC64WholePixels(cell int) string {
	return `async()=>{
	const loaded=(await document.fonts.load('16px "Aura C64"')).some(f=>f.status==='loaded');
	await new Promise(r=>setTimeout(r,300));
	const term=fixtureTerm(),dims=term._core._renderService.dimensions,dpr=devicePixelRatio,css=dims.css.cell,dev=dims.device.cell,
		grid=term.element.querySelector('.xterm-screen'),scr=grid.getBoundingClientRect(),
		glass=fixtureRoot().querySelector('.vd-terminal-screen'),g=glass.getBoundingClientRect();
	return loaded&&dev.width===` + strconv.Itoa(cell) + `&&dev.height===dev.width&&dev.width%8===0&&Math.abs(term.options.fontSize*dpr-dev.width)<1e-6&&
		Math.abs(css.width*dpr-dev.width)<0.05&&Math.abs(css.height*dpr-dev.height)<0.05&&Math.round(css.width*dpr)%8===0&&
		term.options.lineHeight===1&&term.options.allowTransparency===true&&scr.width<=g.width+0.5&&scr.height<=g.height+0.5&&
		Math.abs((scr.left-g.left)-(g.right-scr.right))<=2&&Math.abs((scr.top-g.top)-(g.bottom-scr.bottom))<=2&&
		getComputedStyle(glass).backgroundColor==='rgb(112, 109, 235)'&&grid.style.background==='rgb(0, 0, 0)';
}`
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
	// connect sends board output (`data`, shown as `marker`) before CONNECT and skips an animated dial with a local
	// key. A manual hold is kept by the fixture until terminal.js ends it or fixtureReleaseHold(), so connect checks
	// without a deadline that CONNECT shows the Telnet notice and the key hint while that output waits. Otherwise
	// the real 2 s timer runs and connect only waits for CONNECT.
	connect := func(name, data, marker string, manual bool) {
		c.run(`()=>{fixtureExpectHold(` + strconv.FormatBool(manual) + `);fixtureData('` + data + `');fixtureControl({type:'connected',protocol:'telnet',kind:'c64',charset:'petscii'});}`)
		if full {
			c.check(name+": still dialling", `()=>fixtureRoot().dataset.terminalMode==='dialing'`)
			c.run(`async()=>{await fixtureInput('x');}`)
		}
		if !manual {
			c.wait(name+": CONNECT arms the real hold timer", `()=>fixtureRoot().dataset.terminalMode==='retro'&&fixtureHolds.length===1&&!fixtureHold().manual`)
			return
		}
		c.wait(name+": CONNECT holds the Telnet notice and the key hint, the board output waits", `async()=>fixtureRoot().dataset.terminalMode==='retro'&&
			fixtureHolds.length===1&&fixtureHold().manual&&fixtureHold().state==='pending'&&await fixtureHintShown('`+marker+`')`)
		c.check(name+": still held a moment later", `async()=>{await new Promise(r=>setTimeout(r,300));
			return fixtureHold().state==='pending'&&fixtureHintShown('`+marker+`');}`)
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
	c.wait("the C64 grid takes whole pixels, centred in the C64 border colour, on a black screen", terminalC64WholePixels(16))

	// 3. A colour key while dialling stays local. Board output buffered before CONNECT waits while CONNECT, the Telnet
	// notice and the key hint (packed into 40 columns) stay readable; a key ends that hold at once and still goes out.
	c.wait("C64 socket open", `()=>fixtureSocket().readyState===1`)
	c.run(`()=>fixtureTerm().focus()`)
	press(ctrlShift, input.Digit1)
	c.check("the colour key reached the terminal while dialling", `()=>fixtureKeys.at(-1)==='CS+Digit1'&&fixtureRoot().dataset.terminalMode==='dialing'`)
	connect("first call", `\x1b[38;2;112;109;235mBOARD ONE`, "BOARD ONE", true)
	c.check("CONNECT, then every part of the key hint whole", `async()=>{await fixtureFlush();const lines=fixtureLines(),text=lines.join('\n'),parts=t('desktop.terminal_retronet_c64_keys').split(' · '),
			connect=lines.findIndex(l=>l.startsWith('CONNECT ')),notice=lines.indexOf(fixtureWrapped('desktop.terminal_retronet_telnet_notice')[0]),hint=lines.findIndex(l=>l.startsWith(parts[0]));
		return connect>=0&&notice>connect&&hint>notice&&parts.every(p=>text.includes(p))&&!text.includes('…');}`)
	c.check("CONNECT announces the Telnet notice, the C64 keys and the locked style, Ctrl+] only once", `()=>{const live=fixtureRoot().querySelector('[data-terminal-announce]').textContent;
		return live.startsWith(t('desktop.terminal_retronet_telnet_notice'))&&live.includes(t('desktop.terminal_retronet_c64_keys'))&&
			live.includes(t('desktop.terminal_retronet_style_locked'))&&!live.includes(t('desktop.terminal_retronet_hangup_hint'))&&live.split('Strg+]').length===2;}`)
	c.check("neither the colour key nor the skip key reached the board", `()=>fixtureSent(fixtureSocket()).length===0`)
	c.check("a key ends the hold at once: it reaches the board and the board output lands on the fresh screen", `async()=>{const held=fixtureHold().state==='pending';
		await fixtureInput('A');await fixtureFlush();const b=fixtureTerm().buffer.active;
		return held&&fixtureHold().state==='cleared'&&fixtureSent(fixtureSocket()).join('|')==='bin:A'&&(`+freshScreen+`)('BOARD ONE')&&b.cursorY===24&&b.cursorX===9;}`)

	// 4. The PETSCII engine owns the 40x25 screen from the first board byte: it started blank with the cursor at row 25,
	// column 1 (above), and every glyph lands where the engine's cursor positioning puts it. Its repaints come as
	// synchronized-output blocks (ESC[?2026h ... ESC[?2026l), which the vendored xterm 6 supports (DEC mode 2026).
	c.run(`()=>fixtureData('\x1b[?2026h\x1b[1;1H'+String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)+'\x1b[3;5H'+String.fromCharCode(0xee01))`)
	c.wait("an open synchronized block switches xterm's synchronized output on", `async()=>{await fixtureFlush();return fixtureTerm().modes.synchronizedOutputMode===true;}`)
	c.run(`()=>fixtureData('\x1b[?2026l')`)
	c.wait("board glyphs land where the PETSCII engine put them once the block ends", `async()=>{await fixtureFlush();const lines=fixtureLines();
		return fixtureTerm().modes.synchronizedOutputMode===false&&lines[0]===String.fromCharCode(0xef08,0xef05,0xef0c,0xef0c,0xef0f)&&
			lines[2]==='    '+String.fromCharCode(0xee01)&&lines[1]===''&&lines.slice(3,24).every(l=>l==='')&&lines[24]==='BOARD ONE';}`)
	// Block graphics as the board draws them, read back from the canvas: no seam at any cell edge.
	c.run(`()=>fixtureData(` + terminalC64SeamScreen + `)`)
	c.check("half blocks show no seam at any cell edge", terminalC64Seams)

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
			term.options.allowTransparency===false&&term.options.letterSpacing===0&&
			!term.element.style.paddingLeft&&!(term.cols===40&&term.rows===25)&&fixtureCrtBackgrounds.at(-1)===amber.theme.background&&
			localStorage.getItem('aurago.desktop.terminal.style')==='amber';}`)
	c.check("leaving the C64 look switches straight to the user's font", `()=>fixtureFonts.length>0&&!fixtureFonts.some(f=>f.includes('Aura C64'))`)

	// 8. Without a key the real 2 s timer ends the hold by itself and the held board output follows (c.wait polls up
	// to 8 s, so a slow machine only delays it). Then the server ends the call because Retro-Net was switched off:
	// the next key opens the shell in the user's look.
	dial("second call")
	connect("second call", "BOARD TWO", "BOARD TWO", false)
	c.wait("without a key the real timer ends the hold and the board output follows on the fresh screen", `async()=>{await fixtureFlush();const hold=fixtureHold();
		return hold.state==='fired'&&hold.endedAt-hold.armedAt>=1950&&(`+freshScreen+`)('BOARD TWO');}`)
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
	connect("third call", "BOARD THREE", "BOARD THREE", true)
	c.run(`()=>{fixtureControl({type:'result',code:'NO CARRIER',reason:'remote_closed'});fixtureSocket().serverClose(1000);}`)
	c.wait("the hang-up ends the hold and the held output is shown before the result", `async()=>{await fixtureFlush();const lines=fixtureLines(),board=lines.indexOf('BOARD THREE'),code=lines.indexOf('NO CARRIER');
		return fixtureHold().state==='cleared'&&fixtureRoot().dataset.terminalMode==='result'&&board>=0&&code>board&&!lines.some(l=>l.startsWith(t('desktop.terminal_retronet_c64_keys').split(' · ')[0]));}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// A local hang-up during the hold drops the held output and the hold never writes over the result.
	dial("fourth call")
	connect("fourth call", "BOARD FOUR", "BOARD FOUR", true)
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.check("Ctrl+] hangs up during the hold, ends it and is never sent", `async()=>{await fixtureFlush();
		return fixtureHold().state==='cleared'&&fixtureRoot().dataset.terminalMode==='result'&&fixtureLines().includes('NO CARRIER')&&fixtureSent(fixtureSocket()).length===0;}`)
	c.check("a hold timer going off late leaves the result untouched and the held output dropped", `async()=>{const pending=fixtureReleaseHold(),late=fixtureReleaseHold(true);
		await fixtureFlush();const lines=fixtureLines();
		return !pending&&late&&lines.includes('NO CARRIER')&&!lines.some(l=>l.includes('BOARD FOUR'))&&fixtureRoot().dataset.terminalMode==='result';}`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// A colour key ends the hold like any key.
	dial("fifth call")
	connect("fifth call", "BOARD FIVE", "BOARD FIVE", true)
	c.run(`()=>fixtureTerm().focus()`)
	press(ctrlShift, input.Digit1)
	c.check("a colour key ends the hold at once and is sent", `async()=>{await fixtureFlush();
		return fixtureHold().state==='cleared'&&fixtureSent(fixtureSocket()).join('|')==='bin:\x1b[64;144~'&&(`+freshScreen+`)('BOARD FIVE');}`)
	c.run(`async()=>{await fixtureInput('\x1d');}`)
	c.wait("the fifth call hangs up", `()=>fixtureRoot().dataset.terminalMode==='result'&&fixtureLive().length===0`)
	c.run(`async()=>{await fixtureDismiss('q');}`)

	// More held output than the throttle may queue ends the hold early; closing the window during the call leaves
	// the saved style untouched.
	dial("sixth call")
	connect("sixth call", "BOARD SIX", "BOARD SIX", true)
	c.check("a full queue ends the hold at once", `async()=>{const held=fixtureHold().state==='pending';
		fixtureData('\x1b[0m'.repeat(TerminalModem.MAX_QUEUE_BYTES/4+1));await fixtureFlush();
		return held&&fixtureHold().state==='cleared'&&(`+freshScreen+`)('BOARD SIX');}`)
	c.run(`()=>TerminalApp.dispose()`)
	c.check("dispose closes the call and the C64 look was never saved", `()=>fixtureLive().length===0&&localStorage.getItem('aurago.desktop.terminal.style')==='amber'`)
	c.check("no browser errors and never two live sockets", `()=>fixtureErrors.length===0`)
}
