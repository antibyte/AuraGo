// The source files of the two Storymaker player bundles. scripts/build-ui-bundles.js concatenates them, and
// scripts/test-storymaker-player.mjs loads the same lists, so the bundles and the tests cannot drift apart.
//
//   host.bundle.js    document realm: the host page (app protocol, render driver, workers, deadlines)
//   player.bundle.js  worker realm: compile, render, rig, code-scene API, role handlers
export const storymakerHostParts = [
  'ui/js/storymaker/schedule.js',
  'ui/js/storymaker/host-core.js',
  'ui/js/storymaker/host-app.js',
  'ui/js/storymaker/host-pool.js',
  'ui/js/storymaker/host-render.js',
  'ui/js/storymaker/host.js'
];

export const storymakerPlayerParts = [
  'ui/js/storymaker/textures.js',
  'ui/js/storymaker/ease.js',
  'ui/js/storymaker/seed.js',
  'ui/js/storymaker/ranges.js',
  'ui/js/storymaker/timeline.js',
  'ui/js/storymaker/schedule.js',
  'ui/js/storymaker/stage.js',
  'ui/js/storymaker/parts.js',
  'ui/js/storymaker/rig.js',
  'ui/js/storymaker/glyphs.js',
  'ui/js/storymaker/text.js',
  'ui/js/storymaker/camera.js',
  'ui/js/storymaker/scene.js',
  'ui/js/storymaker/narration.js',
  'ui/js/storymaker/cuemap.js',
  'ui/js/storymaker/captions.js',
  'ui/js/storymaker/lipsync.js',
  'ui/js/storymaker/plan.js',
  'ui/js/storymaker/motion.js',
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
  'ui/js/storymaker/checks.js',
  'ui/js/storymaker/compile.js',
  'ui/js/storymaker/transitions.js',
  'ui/js/storymaker/render.js',
  'ui/js/storymaker/layout.js',
  'ui/js/storymaker/raster-rig.js',
  'ui/js/storymaker/raster.js',
  'ui/js/storymaker/codescene.js',
  'ui/js/storymaker/determinism.js',
  'ui/js/storymaker/worker-player.js',
  'ui/js/storymaker/worker-scene.js',
  'ui/js/storymaker/worker-main.js'
];
