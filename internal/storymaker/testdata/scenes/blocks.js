export default function scene(api) {
  api.duration(8);
  const head = api.headline('Die *Antwort* in Zahlen', { at: 'top', entry: 'fade', start: 0 });
  const count = api.chart({ type: 'counter', labels: ['Treffer'], values: [90], unit: '%' }, { at: 'left', entry: 'fade', start: 0.4 });
  const icon = api.icon('icon.server', { at: 'right', style: 'card', label: 'Server', entry: 'draw', start: 0.8 });
  head.fadeOut({ start: 4, dur: 0.5 });
  count.fadeOut({ start: 4, dur: 0.5 });
  icon.fadeOut({ start: 4, dur: 0.5 });
  const flow = api.diagram({
    layout: 'flow',
    flow: true,
    nodes: [
      { id: 'req', label: 'Anfrage', icon: 'icon.send' },
      { id: 'srv', label: 'Server', icon: 'icon.server' },
      { id: 'res', label: 'Antwort', icon: 'icon.mail' }
    ]
  }, { at: 'left', entry: 'draw', start: 4.5, dur: 1.5 });
  flow.highlight('srv', { start: 6, dur: 1.2 });
  api.list(['Anfrage lesen', 'Antwort bauen', 'Senden'], { at: 'right', entry: 'fade', start: 5 });
}
