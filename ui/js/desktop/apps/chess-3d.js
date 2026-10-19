/* AuraGo chess - 3D board.
 *
 * Renders the live position with the original AuraGo chess piece kit from
 * /3d/chess/v1 on top of the vendored Three.js runtime. The module is a
 * companion of chess.js: it never owns game state. It mirrors the snapshot it
 * is given and reports player intent (select, move) back to the app.
 *
 * Contract with chess.js:
 *   app          the chess app state (read-only): game, flipped, lastMove,
 *                playerColor, boardSkin, reviewActive
 *   canMove()    true while the local player may move
 *   legalMoves(square)  verbose chess.js move list for a square
 *   onMove(from, to)    apply a player move, returns true when accepted
 *   t(key)       translation helper for visible strings
 */
(function () {
  'use strict';

  const PIECE_FILES = { p: 'pawn', n: 'knight', b: 'bishop', r: 'rook', q: 'queen', k: 'king' };
  const FILES = 'abcdefgh';
  const SQUARE = 0.055;
  const BOARD_SIZE = SQUARE * 8;
  const FRAME_WIDTH = 0.019;
  const BASE_THICKNESS = 0.014;
  const SQUARE_LIFT = 0.0004;
  const MARKER_LIFT = 0.0011;
  const ASSET_BASE = '/3d/chess/v1/';
  const MOVE_MS = 300;
  const FLIP_MS = 420;
  const PIECE_LIFT = 0.0016;

  const SKINS = {
    green: { light: 0xe0ddcc, dark: 0x4c946a, frame: 0x5a3f28, frameHi: 0x7a5636, floor: 0x111a15 },
    blue: { light: 0xd8ecfb, dark: 0x86afcf, frame: 0x3a4657, frameHi: 0x55687e, floor: 0x11161d },
    'chess-club': { light: 0xe6d3b1, dark: 0xaf6b3f, frame: 0x6b4a2f, frameHi: 0x8a6540, floor: 0x191410 },
    default: { light: 0xecdab9, dark: 0xc5a076, frame: 0x5a3f28, frameHi: 0x7a5636, floor: 0x14181f }
  };

  const PIECE_MATERIALS = {
    w: { color: 0xf4efe3, roughness: 0.36, metalness: 0.02 },
    b: { color: 0x23242b, roughness: 0.31, metalness: 0.12 }
  };

  function squareName(file, rank) {
    return FILES[file] + (rank + 1);
  }

  function squareCoords(name) {
    return { file: FILES.indexOf(name.charAt(0)), rank: Number(name.charAt(1)) - 1 };
  }

  function parsePosition(fen) {
    const map = new Map();
    const rows = String(fen || '').split(' ')[0].split('/');
    rows.forEach((row, rowIndex) => {
      let file = 0;
      for (const ch of row) {
        if (ch >= '1' && ch <= '8') {
          file += Number(ch);
          continue;
        }
        if (file > 7) break;
        // A FEN starts at rank 8, the board model counts from rank 1.
        map.set(squareName(file, 7 - rowIndex), ch);
        file += 1;
      }
    });
    return map;
  }

  function easeInOut(t) {
    return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
  }

  function prefersReducedMotion() {
    try {
      return window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    } catch (error) {
      return false;
    }
  }

  function createChess3D(config) {
    const options = config || {};
    const container = options.container;
    const app = options.app || {};
    const api = options.api || {};
    if (!container || !window.THREE || !window.THREE.GLTFLoader) return null;

    const THREE = window.THREE;
    const t = typeof options.t === 'function' ? options.t : (key) => key;
    const reduced = prefersReducedMotion();

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0d1017);
    scene.fog = new THREE.Fog(0x0d1017, 1.6, 3.4);

    const camera = new THREE.PerspectiveCamera(32, 1, 0.05, 12);
    camera.position.set(0, 0.62, -0.72);

    let renderer = null;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'low-power' });
    } catch (error) {
      return null;
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    if ('outputColorSpace' in renderer) renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.className = 'vd-chess-3d-canvas';
    renderer.domElement.setAttribute('aria-label', t('desktop.chess_3d_canvas'));
    renderer.domElement.setAttribute('title', t('desktop.chess_3d_hint'));
    renderer.domElement.setAttribute('role', 'img');
    container.appendChild(renderer.domElement);

    const hemi = new THREE.HemisphereLight(0xdce7f6, 0x1a1a18, 0.75);
    scene.add(hemi);
    const key = new THREE.DirectionalLight(0xfff2df, 1.45);
    key.position.set(0.32, 0.55, 0.36);
    key.castShadow = true;
    key.shadow.mapSize.set(1024, 1024);
    key.shadow.camera.near = 0.05;
    key.shadow.camera.far = 2;
    key.shadow.camera.left = -0.4;
    key.shadow.camera.right = 0.4;
    key.shadow.camera.top = 0.4;
    key.shadow.camera.bottom = -0.4;
    key.shadow.bias = -0.0008;
    key.shadow.normalBias = 0.004;
    scene.add(key);
    const rim = new THREE.DirectionalLight(0xa8c4ff, 0.42);
    rim.position.set(-0.42, 0.26, -0.34);
    scene.add(rim);

    const boardGroup = new THREE.Group();
    scene.add(boardGroup);
    const pieceGroup = new THREE.Group();
    boardGroup.add(pieceGroup);
    const markerGroup = new THREE.Group();
    boardGroup.add(markerGroup);

    const squareGeometry = new THREE.PlaneGeometry(SQUARE, SQUARE);
    squareGeometry.rotateX(-Math.PI / 2);
    const markerGeometry = new THREE.PlaneGeometry(SQUARE * 0.94, SQUARE * 0.94);
    markerGeometry.rotateX(-Math.PI / 2);
    const dotGeometry = new THREE.CircleGeometry(SQUARE * 0.13, 24);
    dotGeometry.rotateX(-Math.PI / 2);
    const ringGeometry = new THREE.RingGeometry(SQUARE * 0.26, SQUARE * 0.36, 28);
    ringGeometry.rotateX(-Math.PI / 2);

    const squares = new Map();
    const baseMaterial = new THREE.MeshStandardMaterial({ color: 0x5a3f28, roughness: 0.7, metalness: 0.02 });
    const base = new THREE.Mesh(
      new THREE.BoxGeometry(BOARD_SIZE + FRAME_WIDTH * 2, BASE_THICKNESS, BOARD_SIZE + FRAME_WIDTH * 2),
      baseMaterial
    );
    base.position.y = -BASE_THICKNESS / 2;
    base.receiveShadow = true;
    boardGroup.add(base);

    const frameMaterial = new THREE.MeshStandardMaterial({ color: 0x5a3f28, roughness: 0.55, metalness: 0.04 });
    const frame = new THREE.Group();
    const frameLong = new THREE.BoxGeometry(BOARD_SIZE + FRAME_WIDTH * 2, BASE_THICKNESS * 0.9, FRAME_WIDTH);
    const frameShort = new THREE.BoxGeometry(FRAME_WIDTH, BASE_THICKNESS * 0.9, BOARD_SIZE);
    const frameParts = [
      [frameLong, 0, (BOARD_SIZE + FRAME_WIDTH) / 2],
      [frameLong, 0, -(BOARD_SIZE + FRAME_WIDTH) / 2],
      [frameShort, (BOARD_SIZE + FRAME_WIDTH) / 2, 0],
      [frameShort, -(BOARD_SIZE + FRAME_WIDTH) / 2, 0]
    ];
    frameParts.forEach(([geometry, x, z]) => {
      const part = new THREE.Mesh(geometry, frameMaterial);
      part.position.set(x, BASE_THICKNESS * 0.42, z);
      part.castShadow = true;
      part.receiveShadow = true;
      frame.add(part);
    });
    boardGroup.add(frame);

    const floorMaterial = new THREE.MeshStandardMaterial({ color: 0x11151c, roughness: 0.95, metalness: 0 });
    const floor = new THREE.Mesh(new THREE.PlaneGeometry(4, 4), floorMaterial);
    floor.rotation.x = -Math.PI / 2;
    floor.position.y = -BASE_THICKNESS - 0.0005;
    floor.receiveShadow = true;
    scene.add(floor);

    for (let rank = 0; rank < 8; rank += 1) {
      for (let file = 0; file < 8; file += 1) {
        const name = squareName(file, rank);
        const material = new THREE.MeshStandardMaterial({ color: 0xffffff, roughness: 0.78, metalness: 0.01 });
        const mesh = new THREE.Mesh(squareGeometry, material);
        mesh.position.set((3.5 - file) * SQUARE, SQUARE_LIFT, (rank - 3.5) * SQUARE);
        mesh.receiveShadow = true;
        mesh.userData.square = name;
        boardGroup.add(mesh);
        squares.set(name, mesh);
      }
    }

    const controls = window.THREE.OrbitControls
      ? new window.THREE.OrbitControls(camera, renderer.domElement)
      : null;
    if (controls) {
      controls.enableDamping = true;
      controls.dampingFactor = 0.09;
      controls.enablePan = false;
      controls.minDistance = 0.34;
      controls.maxDistance = 1.8;
      controls.minPolarAngle = 0.22;
      controls.maxPolarAngle = 1.32;
      controls.target.set(0, 0.02, 0);
      controls.update();
    }

    const state = {
      prototypes: new Map(),
      pieces: new Map(),
      markers: [],
      selected: null,
      targets: [],
      fen: '',
      skin: null,
      orientation: 0,
      flipTween: null,
      tweens: [],
      loaded: false,
      built: false,
      failed: false,
      disposed: false,
      frame: 0,
      ready: false
    };

    function squarePosition(name) {
      const coords = squareCoords(name);
      return new THREE.Vector3(
        (3.5 - coords.file) * SQUARE,
        PIECE_LIFT,
        (coords.rank - 3.5) * SQUARE
      );
    }

    function applySkin(skin) {
      const palette = SKINS[skin] || SKINS.green;
      squares.forEach((mesh) => {
        const coords = squareCoords(mesh.userData.square);
        const dark = (coords.file + coords.rank) % 2 === 0;
        mesh.material.color.setHex(dark ? palette.dark : palette.light);
      });
      baseMaterial.color.setHex(palette.frame);
      frameMaterial.color.setHex(palette.frame);
      floorMaterial.color.setHex(palette.floor);
      scene.background.setHex(palette.floor);
      if (scene.fog) scene.fog.color.setHex(palette.floor);
    }

    function loadPieces() {
      const loader = new THREE.GLTFLoader();
      const names = Object.keys(PIECE_FILES).map((code) => PIECE_FILES[code]);
      let pending = names.length;
      let failed = false;
      names.forEach((name) => {
        loader.load(
          ASSET_BASE + name + '.glb',
          (gltf) => {
            state.prototypes.set(name, gltf.scene);
            pending -= 1;
            if (pending === 0) {
              state.loaded = !failed;
              state.failed = failed;
              state.built = false;
              if (state.ready) sync();
            }
          },
          undefined,
          () => {
            failed = true;
            pending -= 1;
            if (pending === 0) {
              state.loaded = false;
              state.failed = true;
              state.built = false;
              if (state.ready) sync();
            }
          }
        );
      });
    }

    function pieceMaterial(code) {
      const preset = PIECE_MATERIALS[code === 'w' ? 'w' : 'b'];
      return new THREE.MeshStandardMaterial({
        color: preset.color,
        roughness: preset.roughness,
        metalness: preset.metalness
      });
    }

    function createPiece(code, name) {
      const file = PIECE_FILES[code.toLowerCase()];
      const prototype = state.prototypes.get(file);
      if (!prototype) return null;
      const mesh = prototype.clone(true);
      const material = pieceMaterial(code === code.toUpperCase() ? 'w' : 'b');
      mesh.traverse((node) => {
        if (node.isMesh) {
          node.material = material;
          node.castShadow = true;
          node.receiveShadow = true;
        }
      });
      mesh.position.copy(squarePosition(name));
      pieceGroup.add(mesh);
      return { mesh: mesh, code: code, material: material };
    }

    function removePiece(entry) {
      if (!entry) return;
      pieceGroup.remove(entry.mesh);
      entry.material.dispose();
    }

    function tween(duration, update) {
      const tweenEntry = { start: performance.now(), duration: duration, update: update };
      state.tweens.push(tweenEntry);
      return tweenEntry;
    }

    function animateTo(entry, name) {
      const target = squarePosition(name);
      if (reduced) {
        entry.mesh.position.copy(target);
        return;
      }
      const from = entry.mesh.position.clone();
      tween(MOVE_MS, (progress) => {
        const eased = easeInOut(progress);
        entry.mesh.position.lerpVectors(from, target, eased);
        entry.mesh.position.y += Math.sin(progress * Math.PI) * 0.014;
      });
    }

    function spawn(entry) {
      if (reduced) return;
      const base = entry.mesh.position.clone();
      entry.mesh.scale.setScalar(0.001);
      tween(MOVE_MS, (progress) => {
        const eased = easeInOut(progress);
        entry.mesh.scale.setScalar(0.001 + eased * 0.999);
        entry.mesh.position.copy(base);
        entry.mesh.position.y += (1 - eased) * 0.03;
      });
    }

    function syncPieces(position, lastMove) {
      const previous = state.pieces;
      const next = new Map();
      let moved = null;
      previous.forEach((entry, name) => {
        if (position.get(name) === entry.code) next.set(name, entry);
      });
      if (lastMove && lastMove.from && lastMove.to) {
        const moving = previous.get(lastMove.from);
        if (moving && position.get(lastMove.to) && position.get(lastMove.to) !== moving.code) {
          const captured = previous.get(lastMove.to);
          if (captured && captured !== moving) removePiece(captured);
          animateTo(moving, lastMove.to);
          next.set(lastMove.to, moving);
          moved = moving;
        }
      }
      position.forEach((code, name) => {
        if (next.has(name)) return;
        const entry = createPiece(code, name);
        if (!entry) return;
        spawn(entry);
        next.set(name, entry);
      });
      previous.forEach((entry, name) => {
        if (entry === moved) return;
        if (!next.has(name)) removePiece(entry);
      });
      state.pieces = next;
    }

    function clearMarkers() {
      while (state.markers.length) {
        const marker = state.markers.pop();
        markerGroup.remove(marker.mesh);
        marker.mesh.material.dispose();
      }
    }

    function addMarker(geometry, name, color, opacity) {
      const material = new THREE.MeshBasicMaterial({
        color: color,
        transparent: true,
        opacity: opacity,
        depthWrite: false
      });
      const mesh = new THREE.Mesh(geometry, material);
      const position = squarePosition(name);
      mesh.position.set(position.x, MARKER_LIFT + state.markers.length * 0.00004, position.z);
      markerGroup.add(mesh);
      state.markers.push({ mesh: mesh });
    }

    function renderMarkers(snapshot) {
      clearMarkers();
      if (snapshot.lastMove) {
        addMarker(markerGeometry, snapshot.lastMove.from, 0xf0b429, 0.34);
        addMarker(markerGeometry, snapshot.lastMove.to, 0xf0b429, 0.34);
      }
      if (snapshot.checkSquare) {
        addMarker(markerGeometry, snapshot.checkSquare, 0xe04b4b, 0.42);
      }
      if (state.selected) {
        addMarker(markerGeometry, state.selected, 0x3f8cd8, 0.32);
      }
      state.targets.forEach((target) => {
        if (target.capture) {
          addMarker(ringGeometry, target.to, 0x1d2a22, 0.55);
        } else {
          addMarker(dotGeometry, target.to, 0x1d2a22, 0.5);
        }
      });
    }

    function snapshot() {
      const game = app.game;
      if (!game) return null;
      const fen = typeof game.fen === 'function' ? game.fen() : '';
      const turn = typeof game.turn === 'function' ? game.turn() : 'w';
      const inCheck = typeof game.inCheck === 'function'
        ? !!game.inCheck()
        : (typeof game.in_check === 'function' ? !!game.in_check() : false);
      const player = app.playerColor === 'black' ? 'b' : 'w';
      return {
        fen: fen,
        turn: turn,
        flipped: !!app.flipped,
        orientation: app.flipped ? (player === 'w' ? 'b' : 'w') : player,
        skin: app.boardSkin || 'green',
        lastMove: app.lastMove || null,
        playerColor: player,
        review: !!app.reviewActive,
        checkSquare: inCheck ? kingSquare(fen, turn) : null
      };
    }

    function kingSquare(fen, color) {
      const rows = String(fen || '').split(' ')[0].split('/');
      const boardRows = color === 'b' ? rows : rows.slice().reverse();
      for (let row = 0; row < 8; row += 1) {
        let file = 0;
        for (const ch of boardRows[row]) {
          if (ch >= '1' && ch <= '8') {
            file += Number(ch);
            continue;
          }
          const mine = color === 'b' ? ch === 'k' : ch === 'K';
          if (mine) return squareName(file, color === 'b' ? 7 - row : row);
          file += 1;
        }
      }
      return null;
    }

    function sync() {
      if (state.disposed) return;
      const current = snapshot();
      if (!current) return;
      if (current.skin !== state.skin) {
        state.skin = current.skin;
        applySkin(current.skin);
      }
      const orientation = current.orientation === 'b' ? Math.PI : 0;
      if (Math.abs(orientation - state.orientation) > 0.001) {
        state.orientation = orientation;
        if (reduced) {
          boardGroup.rotation.y = orientation;
        } else {
          const from = boardGroup.rotation.y;
          state.flipTween = tween(FLIP_MS, (progress) => {
            boardGroup.rotation.y = from + (orientation - from) * easeInOut(progress);
          });
        }
      }
      if (current.fen === state.fen && state.built) {
        renderMarkers(current);
        return;
      }
      const changed = current.fen !== state.fen;
      state.fen = current.fen;
      if (state.loaded) {
        syncPieces(parsePosition(current.fen), changed ? current.lastMove : null);
        state.built = true;
      }
      renderMarkers(current);
    }

    function squareFromPointer(event) {
      const rect = renderer.domElement.getBoundingClientRect();
      if (!rect.width || !rect.height) return null;
      const ndc = new THREE.Vector2(
        ((event.clientX - rect.left) / rect.width) * 2 - 1,
        -((event.clientY - rect.top) / rect.height) * 2 + 1
      );
      const raycaster = new THREE.Raycaster();
      raycaster.setFromCamera(ndc, camera);
      const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
      const hit = new THREE.Vector3();
      if (!raycaster.ray.intersectPlane(plane, hit)) return null;
      const local = boardGroup.worldToLocal(hit.clone());
      const file = Math.round(3.5 - local.x / SQUARE);
      const rank = Math.round(local.z / SQUARE + 3.5);
      if (file < 0 || file > 7 || rank < 0 || rank > 7) return null;
      return squareName(file, rank);
    }

    function selectSquare(name) {
      const current = snapshot();
      if (!current) return;
      const piece = state.pieces.get(name);
      const isOwn = piece && (piece.code === piece.code.toUpperCase() ? 'w' : 'b') === current.playerColor;
      const canMove = typeof api.canMove === 'function' ? api.canMove() : false;
      if (isOwn && canMove && current.turn === current.playerColor && !current.review) {
        state.selected = name;
        state.targets = typeof api.legalMoves === 'function' ? api.legalMoves(name) : [];
        renderMarkers(current);
        return;
      }
      state.selected = null;
      state.targets = [];
      renderMarkers(current);
    }

    function clickSquare(name) {
      const current = snapshot();
      if (!current) return;
      const target = state.targets.find((entry) => entry.to === name);
      if (state.selected && target) {
        const from = state.selected;
        state.selected = null;
        state.targets = [];
        if (typeof api.onMove === 'function') api.onMove(from, name);
        renderMarkers(current);
        return;
      }
      selectSquare(name);
    }

    let pointerDown = null;
    function onPointerDown(event) {
      if (event.button !== 0) return;
      pointerDown = { x: event.clientX, y: event.clientY, time: performance.now() };
    }

    function onPointerUp(event) {
      if (event.button !== 0 || !pointerDown) return;
      const moved = Math.hypot(event.clientX - pointerDown.x, event.clientY - pointerDown.y);
      const elapsed = performance.now() - pointerDown.time;
      pointerDown = null;
      if (moved > 7 || elapsed > 700) return;
      const name = squareFromPointer(event);
      if (name) clickSquare(name);
    }

    renderer.domElement.addEventListener('pointerdown', onPointerDown);
    renderer.domElement.addEventListener('pointerup', onPointerUp);
    renderer.domElement.addEventListener('pointercancel', () => {
      pointerDown = null;
    });
    renderer.domElement.addEventListener('webglcontextlost', (event) => {
      event.preventDefault();
      state.failed = true;
    });

    function fitCamera() {
      const target = controls ? controls.target : new THREE.Vector3(0, 0.02, 0);
      const half = Math.tan((camera.fov * Math.PI) / 360);
      const radius = (BOARD_SIZE / 2 + FRAME_WIDTH) * 1.26;
      const distance = Math.max(radius / half, radius / (half * Math.max(camera.aspect, 0.25)));
      const direction = camera.position.clone().sub(target);
      if (direction.lengthSq() < 1e-8) direction.set(0, 0.62, -0.72);
      direction.normalize();
      camera.position.copy(target).addScaledVector(direction, distance);
      if (controls) {
        controls.minDistance = distance * 0.6;
        controls.maxDistance = distance * 2.1;
        controls.update();
      }
    }

    function resize() {
      const width = container.clientWidth || 1;
      const height = container.clientHeight || 1;
      renderer.setSize(width, height);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
      fitCamera();
    }

    if (typeof ResizeObserver !== 'undefined') {
      state.observer = new ResizeObserver(resize);
      state.observer.observe(container);
    }
    window.addEventListener('resize', resize);

    function render(now) {
      if (state.disposed) return;
      state.frame = window.requestAnimationFrame(render);
      if (document.hidden) return;
      const active = state.tweens.filter((entry) => now - entry.start < entry.duration);
      state.tweens = active;
      active.forEach((entry) => {
        const progress = Math.min(1, (now - entry.start) / entry.duration);
        entry.update(progress);
      });
      if (controls) controls.update();
      renderer.render(scene, camera);
    }

    resize();
    loadPieces();
    state.ready = true;
    sync();
    state.frame = window.requestAnimationFrame(render);

    return {
      sync: sync,
      dispose: function () {
        if (state.disposed) return;
        state.disposed = true;
        window.cancelAnimationFrame(state.frame);
        window.removeEventListener('resize', resize);
        if (state.observer) state.observer.disconnect();
        renderer.domElement.removeEventListener('pointerdown', onPointerDown);
        renderer.domElement.removeEventListener('pointerup', onPointerUp);
        clearMarkers();
        state.pieces.forEach((entry) => removePiece(entry));
        state.pieces.clear();
        if (controls && controls.dispose) controls.dispose();
        renderer.dispose();
        if (renderer.domElement.parentNode === container) container.removeChild(renderer.domElement);
      }
    };
  }

  window.createChess3D = createChess3D;

  const views = new WeakMap();

  function setViewMode(state, mode) {
    const refs = state.refs || {};
    state.viewMode = mode;
    if (refs.viewToggle) {
      refs.viewToggle.querySelectorAll('[data-chess-view]').forEach((button) => {
        const active = button.getAttribute('data-chess-view') === mode;
        button.classList.toggle('is-active', active);
        button.setAttribute('aria-pressed', active ? 'true' : 'false');
      });
    }
    if (refs.board) refs.board.hidden = mode === '3d';
    if (refs.board3d) refs.board3d.hidden = mode !== '3d';
    if (mode === '3d') {
      if (!views.has(state)) {
        const view = createChess3D({
          container: refs.board3d,
          t: state.ctx ? state.ctx.t : null,
          app: state,
          api: state.view3dApi || {}
        });
        if (view) {
          views.set(state, view);
          state.view3d = view;
          return;
        }
      }
      if (refs.board3d) {
        refs.board3d.textContent = '';
        const note = document.createElement('p');
        note.className = 'vd-chess-3d-note';
        note.textContent = (state.ctx && state.ctx.t ? state.ctx.t('desktop.chess_3d_unavailable') : '3D unavailable');
        refs.board3d.appendChild(note);
      }
      return;
    }
    const view = views.get(state);
    if (view) {
      view.dispose();
      views.delete(state);
      state.view3d = null;
    }
  }

  window.ChessView3D = {
    bind: function (state, api) {
      state.view3dApi = api || {};
      const toggle = state.refs && state.refs.viewToggle;
      if (!toggle || toggle.dataset.chessBound === '1') return;
      toggle.dataset.chessBound = '1';
      toggle.addEventListener('click', event => {
        const button = event.target.closest('[data-chess-view]');
        if (!button) return;
        const mode = button.getAttribute('data-chess-view');
        if (mode !== '2d' && mode !== '3d') return;
        setViewMode(state, mode);
      });
    },
    sync: function (state) {
      const view = views.get(state);
      if (view) view.sync();
    },
    dispose: function (state) {
      const view = views.get(state);
      if (!view) return;
      view.dispose();
      views.delete(state);
      state.view3d = null;
    }
  };
})();
