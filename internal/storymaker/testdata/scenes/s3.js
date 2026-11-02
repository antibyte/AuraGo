export default function scene(api) {
  api.duration(5);
  const anna = api.add('char.office-woman-1', { at: 'left', pose: 'stand' });
  const label = api.text('3 von 4 Paketen geblockt', 'title');
  anna.gesture('point', { start: 1 });
  api.tween(label, { opacity: 1 }, { start: 0.5, dur: 1, ease: 'ease-out' });
  anna.emote('happy', { start: 3 });
}
