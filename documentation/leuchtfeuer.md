# Leuchtfeuer

Leuchtfeuer is a built-in virtual-desktop game. A harbour watch walks the beach, the pier and one moored sloop, picks up glass, wick and oil, and lights the lighthouse. A full round takes about ten minutes. It needs no account, no model provider and no server save.

Open it from the start menu under Games. The window is 1100 by 720. Move with WASD or the arrow keys. Click the ground to walk there. Press E, or the touch button, to pick up a part or to light the tower. The three parts can be collected in any order. The pause button holds the round, starts it again, and turns sound on. Sound stays off until that switch is used.

Stepping into the water or into a patrolling guard returns the watch to the beach. Parts already collected stay in the bag. The lit tower ends the round. Start again from the win card or the pause menu.

The harbour uses the installed Game Maker pack `aurago-pirates-3d` at lod0, with the coast sky, ocean and sounds from `aurago-effects` and `aurago-sounds`. Those files stay in the Game Maker library and are read through the existing asset route. Leuchtfeuer does not copy the models into the desktop UI. If the pack is unavailable, the window shows a retry card.

Progress for the open tab is kept in the browser session only (`sessionStorage`, key `aurago.leuchtfeuer.v1`). Closing the tab clears it. There is no high-score list.

The sentences are translated in all sixteen desktop languages. The name Leuchtfeuer stays the same in every language.
