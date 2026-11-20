// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The drum kit: the samples of a standard mod (`Standard.rbm`), as `rbm::standard` loads the
//! embedded one (`rbm/src/standard.rs:139-160`), but from the bytes the host passes.

use std::sync::Arc;

use orb_engine::samples::SampleSet;

/// Why a kit did not load (the codes of `orb_last_error`).
pub const ERR_PARSE: u32 = 1;
pub const ERR_KEYS: u32 = 2;
pub const ERR_SAMPLES: u32 = 3;

/// Parses a mod file and loads its 46 samples.
pub fn load(bytes: &[u8]) -> Result<Arc<SampleSet>, u32> {
    let r = rbm::Rbm::parse(bytes.to_vec()).map_err(|_| ERR_PARSE)?;
    let t = rbm::KeyTable::embedded(&r)
        .filter(|t| r.all_covered(t))
        .ok_or(ERR_KEYS)?;
    let set = r.load_all_samples(&t).map_err(|_| ERR_SAMPLES)?;
    Ok(Arc::new(set))
}
