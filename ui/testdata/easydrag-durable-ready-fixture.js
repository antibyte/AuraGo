// Isolated browser test of a ready service; no real setting or runtime is changed.
edDurableFixture.state.capability = { configured: true, effective: true, recovery_available: true, waiting_count: 0, claimed_count: 0, blocked_count: 0 };
edFixture.catalog.node_types.find(info => info.type === 'logic.durable_wait').availability = { state: 'available' };
