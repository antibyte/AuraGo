// Source files of desktop-sounds.bundle.js in load order. scripts/build-ui-bundles.js concatenates them and
// scripts/desktop-sound-loader.mjs loads the same list into Node, so the bundle and the tests cannot drift apart.
export const desktopSoundCoreParts = [
  'ui/js/desktop/sound/dsp-kernel.js',
  'ui/js/desktop/sound/instruments.js',
  'ui/js/desktop/sound/sound-engine.js'
];
// Theme id -> file, in the runtime's THEME_IDS order.
export const desktopSoundThemeParts = {
  crystal: 'ui/js/desktop/sound/theme-crystal.js',
  wood: 'ui/js/desktop/sound/theme-wood.js',
  analog: 'ui/js/desktop/sound/theme-analog.js',
  workshop: 'ui/js/desktop/sound/theme-workshop.js',
  water: 'ui/js/desktop/sound/theme-water.js',
  amiga: 'ui/js/desktop/sound/theme-amiga.js',
  cyberpunk: 'ui/js/desktop/sound/theme-cyberpunk.js',
  black_gloss: 'ui/js/desktop/sound/theme-black-gloss.js',
  crazy: 'ui/js/desktop/sound/theme-crazy.js',
  lollipop: 'ui/js/desktop/sound/theme-lollipop.js',
  organic: 'ui/js/desktop/sound/theme-organic.js',
  electronic_dreams: 'ui/js/desktop/sound/theme-electronic-dreams.js',
  windows_xp: 'ui/js/desktop/sound/theme-windows-xp.js',
  green_slime: 'ui/js/desktop/sound/theme-green-slime.js',
  voxel_world: 'ui/js/desktop/sound/theme-voxel-world.js',
  orbit: 'ui/js/desktop/sound/theme-orbit.js'
};
export const desktopSoundHostPart = 'ui/js/desktop/sound/render-host.js';
export const desktopSoundParts = [...desktopSoundCoreParts, ...Object.values(desktopSoundThemeParts), desktopSoundHostPart];
