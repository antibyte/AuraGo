// AURAGO_RUNTIME_API {"version":"voxel-1","api":"startVoxelGame(definition, hooks); hooks: objective, setup(api), step(api,dt), action(api,target), dispose()"}
// World and inventory writes must use the bounded API; observations are read-only.
export { startVoxelGame } from '../vendor/aurago-voxel-1.js';
