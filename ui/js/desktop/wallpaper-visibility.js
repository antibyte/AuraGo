// Shared occlusion policy for animated desktop wallpapers.
export function desktopCovered() {
  const saver = document.getElementById("vd-screensaver");
  if (saver && saver.dataset.state !== "stopping" && saver.getClientRects().length) return true;
  const area = window.innerWidth * window.innerHeight;
  for (const win of document.querySelectorAll(".vd-window.maximized")) {
    if (win.classList.contains("vd-space-hidden") || win.classList.contains("minimized")) continue;
    const rect = win.getBoundingClientRect();
    if (rect.width * rect.height < area * 0.82) continue;
    const style = getComputedStyle(win);
    if (style.display === "none" || style.visibility === "hidden" || parseFloat(style.opacity) < 0.99) continue;
    return true;
  }
  return false;
}
