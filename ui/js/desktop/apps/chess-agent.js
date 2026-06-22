(function () {
    'use strict';

    const AGENT_MOVE_URL = '/api/desktop/chess/agent-move';

    function createChessAgentClient(options = {}) {
        const api = options.api || defaultAPI;

        async function chooseMove(payload) {
            const response = await api(AGENT_MOVE_URL, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    fen: payload.fen,
                    pgn: payload.pgn || '',
                    legal_moves: payload.legal_moves || [],
                    side_to_move: payload.side_to_move || '',
                    move_number: payload.move_number || 0,
                    player_color: payload.player_color || ''
                })
            });
            const move = String(response && response.move ? response.move : '').toLowerCase();
            if (!move) throw new Error('Agent did not return a move.');
            return { move, comment: String(response.comment || '') };
        }

        return { chooseMove };
    }

    async function defaultAPI(url, options) {
        const resp = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) {
            throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        }
        return data;
    }

    window.createChessAgentClient = createChessAgentClient;
})();
