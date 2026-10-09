package localwiki

// htmlPage wraps body HTML like mwoffliner's mobile renderer does.
func htmlPage(title, body string) string {
	return `<!DOCTYPE html><html><head><title>` + title + `</title></head><body><div id="mw-content-text">` +
		`<h1 class="section-heading"><span class="mw-headline">` + title + `</span></h1>` + body + `</div></body></html>`
}
