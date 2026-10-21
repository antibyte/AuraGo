(function () {
    'use strict';

    // The pool of rigid-body cubes behind the fragments of the voxel overlay. Cubes are created in batches of 32 up to
    // maxFragments, reused once recycled, and the oldest one is taken over when every slot is busy. voxel_world.js owns the
    // physics world, the bookkeeping of active fragments and the stepping; this part only creates, finds and retires slots.
    const BATCH = 32;

    // options: fragments (the shared slot array) and colliderToFragment (collider handle -> slot), graphics (AuraVoxelGraphics
    // instance), materials, maxFragments, physics() -> { ready, world, RAPIER } read when needed, onRecycled() after a slot was retired.
    function create(options) {
        const { fragments, colliderToFragment, graphics, materials, maxFragments, physics, onRecycled } = options;

        function ensure(minimum) {
            const { ready, world, RAPIER } = physics();
            if (!ready || !graphics.rendererReady) return;
            const desired = Math.min(maxFragments, Math.max(minimum || 0, Math.min(BATCH, fragments.length + BATCH)));
            while (fragments.length < desired) {
                const body = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setEnabled(false).setCanSleep(true)
                    .setLinearDamping(.18).setAngularDamping(.22).setCcdEnabled(true));
                const collider = world.createCollider(RAPIER.ColliderDesc.cuboid(.055, .055, .055)
                    .setDensity(1.7).setFriction(.78).setRestitution(.19)
                    .setActiveEvents(RAPIER.ActiveEvents.CONTACT_FORCE_EVENTS)
                    .setContactForceEventThreshold(.018), body);
                body.setEnabled(false);
                const slot = { body, collider, active: false, age: 0, settle: 0, size: 10, alpha: 1, lastImpactAge: -1, incoming: { x: 0, y: 0, z: 0 }, material: 'stone' };
                fragments.push(slot);
                colliderToFragment.set(collider.handle, slot);
                graphics.setFragmentSlot(fragments.length - 1, materials.stone[0]);
            }
            graphics.commitFragments(fragments.length);
        }

        function recycle(slot, index) {
            if (!slot || !slot.active) return;
            slot.active = false;
            slot.age = 0;
            slot.settle = 0;
            slot.alpha = 0;
            slot.body.setEnabled(false);
            graphics.setFragmentOpacity(index, 0);
            onRecycled();
        }

        // A free slot as { slot, index }, a freshly grown one, or the oldest active one taken over; null while there is no physics.
        function acquire() {
            if (!physics().ready) return null;
            for (let i = 0; i < fragments.length; i++) {
                if (!fragments[i].active) return { slot: fragments[i], index: i };
            }
            if (fragments.length < maxFragments) {
                const oldLength = fragments.length;
                ensure(Math.min(maxFragments, oldLength + BATCH));
                return fragments.length > oldLength ? { slot: fragments[oldLength], index: oldLength } : null;
            }
            let oldest = 0;
            for (let i = 1; i < fragments.length; i++) if (fragments[i].age > fragments[oldest].age) oldest = i;
            recycle(fragments[oldest], oldest);
            return { slot: fragments[oldest], index: oldest };
        }

        return { ensure, recycle, acquire };
    }

    window.AuraVoxelFragments = Object.freeze({ create });
})();
