//! Animation helpers for particles and effects.
//! Currently minimal — can be expanded with a full particle system later.

#[allow(dead_code)]
pub fn rainbow_tick(tick: u64) -> u16 {
    (tick % 360) as u16
}
