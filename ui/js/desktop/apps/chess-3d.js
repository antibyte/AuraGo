/* AuraGo chess - 3D board.
 *
 * Renders the live position with the original AuraGo chess piece kit from
 * /3d/chess/v1 on top of the vendored Three.js runtime. The module is a
 * companion of chess.js: it never owns game state. It mirrors the snapshot it
 * is given and reports player intent (select, move) back to the app.
 */
(function () {
  'use strict';

  const PIECE_FILES = { p: 'pawn', n: 'knight', b: 'bishop', r: 'rook', q: 'queen', k: 'king' };
  const FILES = 'abcdefgh';
  const SQUARE = 0.055;
  const BOARD_SIZE = SQUARE * 8;
  const FRAME_WIDTH = 0.022;
  const BASE_THICKNESS = 0.015;
  const SQUARE_LIFT = 0.0004;
  const MARKER_LIFT = 0.0011;
  const ASSET_BASE = '/3d/chess/v1/';
  const MOVE_MS = 300;
  const FLIP_MS = 420;
  const PIECE_LIFT = 0.0016;
  const SELECT_LIFT = 0.0065;
  const PIECE_TOP = 0.1;
  const FIT_MARGIN = 1.045;

  /* Square, frame, brass, coordinate and floor colours mirror the skin
     variable sets of css/desktop-app-chess.css. */
  const SKINS = {
    green: { light: 0xececcf, dark: 0x6f9059, frame: 0x5d3d25, frameHi: 0x8a6239, brass: 0xdcbd7c, coord: '#efdcb0', floor: 0x0f1713 },
    blue: { light: 0xdfe7ef, dark: 0x7391b3, frame: 0x33261e, frameHi: 0x55402f, brass: 0xd3dae3, coord: '#e3e9f1', floor: 0x0e141c },
    'chess-club': { light: 0xe7d3ae, dark: 0x9b6641, frame: 0x4a2f1b, frameHi: 0x74502f, brass: 0xd7b877, coord: '#f1e0b9', floor: 0x17120e },
    default: { light: 0xf0d9b5, dark: 0xb58863, frame: 0x523924, frameHi: 0x7a5636, brass: 0xd9ba7a, coord: '#f0dfb8', floor: 0x12161d },
    marble: { light: 0xe7e5e0, dark: 0x2f3238, frame: 0x26232a, frameHi: 0x3a3540, brass: 0xe3e7ee, coord: '#d9dde6', floor: 0x0b0b0e },
    slate: { light: 0xb8c0ca, dark: 0x4b5564, frame: 0x343941, frameHi: 0x4b515c, brass: 0xc8d0da, coord: '#e2e7ee', floor: 0x0c0e12 }
  };

  /* Boxwood and ebony with a satin oil finish; the rougher values keep the
     highlights soft instead of plastic. */
  const PIECE_MATERIALS = {
    w: { color: 0xf6f0e2, roughness: 0.4, metalness: 0.02, pad: 0x265c42 },
    b: { color: 0x24262e, roughness: 0.36, metalness: 0.06, pad: 0x6b2626 }
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

  const {
    createGrainTexture, createGrainVariants, createStudioEnvironment, createCoordTexture, createPoolTexture
  } = window.ChessStage3D || {};

  function createChess3D(config) {
    const options = config || {};
    const container = options.container;
    const app = options.app || {};
    const api = options.api || {};
    if (!container || !window.THREE || !window.THREE.GLTFLoader || !window.ChessStage3D) return null;

    const THREE = window.THREE;
    const t = typeof options.t === 'function' ? options.t : (key) => key;
    const reduced = prefersReducedMotion();
    const ownedTextures = [];
    const ownTex = tex => { if (tex) ownedTextures.push(tex); return tex; };

    /* Board silhouette in world space: the four board corners with and
       without the tallest piece on top. */
    const fitCorners = (() => {
      const half = (BOARD_SIZE + FRAME_WIDTH) / 2;
      const corners = [];
      for (const x of [-half, half]) {
        for (const z of [-half, half]) {
          corners.push(new THREE.Vector3(x, 0, z));
          corners.push(new THREE.Vector3(x, PIECE_TOP, z));
        }
      }
      return corners;
    })();

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0d1017);
    scene.fog = new THREE.Fog(0x0d1017, 1.7, 3.6);

    const camera = new THREE.PerspectiveCamera(31, 1, 0.05, 12);
    camera.position.set(0, 0.62, -0.72);

    let renderer = null;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'low-power' });
    } catch (error) {
      return null;
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap || THREE.PCFShadowMap;
    if ('toneMapping' in renderer && THREE.ACESFilmicToneMapping) {
      renderer.toneMapping = THREE.ACESFilmicToneMapping;
      renderer.toneMappingExposure = 1.1;
    }
    if ('outputColorSpace' in renderer) renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.className = 'vd-chess-3d-canvas';
    renderer.domElement.setAttribute('aria-label', t('desktop.chess_3d_canvas'));
    renderer.domElement.setAttribute('title', t('desktop.chess_3d_hint'));
    renderer.domElement.setAttribute('role', 'img');
    container.appendChild(renderer.domElement);

    const envTarget = createStudioEnvironment(THREE, renderer);
    if (envTarget) scene.environment = envTarget.texture;

    const hemi = new THREE.HemisphereLight(0xe4eefb, 0x221c16, 0.72);
    scene.add(hemi);
    const key = new THREE.DirectionalLight(0xfff3e0, 1.55);
    key.position.set(0.34, 0.58, 0.34);
    key.castShadow = true;
    key.shadow.mapSize.set(1024, 1024);
    key.shadow.camera.near = 0.05;
    key.shadow.camera.far = 2;
    key.shadow.camera.left = -0.42;
    key.shadow.camera.right = 0.42;
    key.shadow.camera.top = 0.42;
    key.shadow.camera.bottom = -0.42;
    key.shadow.bias = -0.0007;
    key.shadow.normalBias = 0.0035;
    scene.add(key);
    const rim = new THREE.DirectionalLight(0xa9c7ff, 0.48);
    rim.position.set(-0.44, 0.3, -0.36);
    scene.add(rim);

    const boardGroup = new THREE.Group();
    scene.add(boardGroup);
    const pieceGroup = new THREE.Group();
    boardGroup.add(pieceGroup);
    const markerGroup = new THREE.Group();
    boardGroup.add(markerGroup);
    const coordGroup = new THREE.Group();
    boardGroup.add(coordGroup);

    const squareGrain = createGrainVariants(THREE, 20, 5);
    squareGrain.forEach(ownTex);
    const frameGrain = ownTex(createGrainTexture(THREE, 24, 9));

    const squareGeometry = new THREE.PlaneGeometry(SQUARE, SQUARE);
    squareGeometry.rotateX(-Math.PI / 2);
    const markerGeometry = new THREE.PlaneGeometry(SQUARE * 0.93, SQUARE * 0.93);
    markerGeometry.rotateX(-Math.PI / 2);
    const dotGeometry = new THREE.CircleGeometry(SQUARE * 0.135, 24);
    dotGeometry.rotateX(-Math.PI / 2);
    const ringGeometry = new THREE.RingGeometry(SQUARE * 0.28, SQUARE * 0.38, 32);
    ringGeometry.rotateX(-Math.PI / 2);
    const padGeometry = new THREE.CylinderGeometry(SQUARE * 0.28, SQUARE * 0.28, 0.0012, 24);

    const squares = new Map();
    const baseMaterial = new THREE.MeshStandardMaterial({ color: 0x4f3520, map: frameGrain, roughness: 0.58, metalness: 0.04 });
    const base = new THREE.Mesh(
      new THREE.BoxGeometry(BOARD_SIZE + FRAME_WIDTH * 2, BASE_THICKNESS, BOARD_SIZE + FRAME_WIDTH * 2),
      baseMaterial
    );
    base.position.y = -BASE_THICKNESS / 2;
    base.receiveShadow = true;
    boardGroup.add(base);

    const frameMaterial = new THREE.MeshStandardMaterial({ color: 0x5a3f28, map: frameGrain, roughness: 0.42, metalness: 0.06 });
    const brassMaterial = new THREE.MeshStandardMaterial({ color: 0xc9a45c, roughness: 0.28, metalness: 0.78 });
    const frame = new THREE.Group();
    const frameHeight = BASE_THICKNESS * 0.92;
    const frameTopY = frameHeight * 0.42 + frameHeight / 2;
    const frameLong = new THREE.BoxGeometry(BOARD_SIZE + FRAME_WIDTH * 2, frameHeight, FRAME_WIDTH);
    const frameShort = new THREE.BoxGeometry(FRAME_WIDTH, frameHeight, BOARD_SIZE);
    [
      [frameLong, 0, (BOARD_SIZE + FRAME_WIDTH) / 2],
      [frameLong, 0, -(BOARD_SIZE + FRAME_WIDTH) / 2],
      [frameShort, (BOARD_SIZE + FRAME_WIDTH) / 2, 0],
      [frameShort, -(BOARD_SIZE + FRAME_WIDTH) / 2, 0]
    ].forEach(([geometry, x, z]) => {
      const part = new THREE.Mesh(geometry, frameMaterial);
      part.position.set(x, frameHeight * 0.42, z);
      part.castShadow = true;
      part.receiveShadow = true;
      frame.add(part);
    });

    // Subtle inlaid brass corner accents + inner fillet rim
    const cornerGeom = new THREE.BoxGeometry(FRAME_WIDTH * 0.46, 0.001, FRAME_WIDTH * 0.46);
    const cornerOffset = (BOARD_SIZE + FRAME_WIDTH * 1.15) / 2;
    for (const sx of [-1, 1]) {
      for (const sz of [-1, 1]) {
        const inlay = new THREE.Mesh(cornerGeom, brassMaterial);
        inlay.position.set(sx * cornerOffset, frameTopY + 0.0004, sz * cornerOffset);
        frame.add(inlay);
      }
    }
    const filletRing = new THREE.RingGeometry(BOARD_SIZE * 0.7071, (BOARD_SIZE + 0.0032) * 0.7071, 4);
    filletRing.rotateZ(Math.PI / 4);
    filletRing.rotateX(-Math.PI / 2);
    const filletMesh = new THREE.Mesh(filletRing, brassMaterial);
    filletMesh.position.y = frameTopY + 0.0003;
    frame.add(filletMesh);
    boardGroup.add(frame);

    /* The lamp pool is emissive, so it stays soft without another shadow pass. */
    const poolTexture = ownTex(createPoolTexture(THREE));
    const floorMaterial = new THREE.MeshStandardMaterial({
      color: 0x0f1713, roughness: 0.94, metalness: 0,
      emissive: 0xffdca8, emissiveIntensity: 0.2, emissiveMap: poolTexture || null
    });
    const floor = new THREE.Mesh(new THREE.PlaneGeometry(4, 4), floorMaterial);
    floor.rotation.x = -Math.PI / 2;
    floor.position.y = -BASE_THICKNESS - 0.0005;
    floor.receiveShadow = true;
    scene.add(floor);

    for (let rank = 0; rank < 8; rank += 1) {
      for (let file = 0; file < 8; file += 1) {
        const name = squareName(file, rank);
        const material = new THREE.MeshStandardMaterial({
          color: 0xffffff,
          map: squareGrain[(rank * 3 + file * 5) % squareGrain.length] || null,
          roughness: 0.62,
          metalness: 0.02
        });
        const mesh = new THREE.Mesh(squareGeometry, material);
        mesh.position.set((3.5 - file) * SQUARE, SQUARE_LIFT, (rank - 3.5) * SQUARE);
        mesh.receiveShadow = true;
        mesh.userData.square = name;
        boardGroup.add(mesh);
        squares.set(name, mesh);
      }
    }

    const coordMeshes = [];
    const coordGeom = new THREE.PlaneGeometry(FRAME_WIDTH * 0.62, FRAME_WIDTH * 0.62);
    coordGeom.rotateX(-Math.PI / 2);

    function rebuildCoordinates(colorStr, orientation) {
      while (coordMeshes.length) {
        const item = coordMeshes.pop();
        coordGroup.remove(item.mesh);
        item.material.dispose();
        if (item.texture) item.texture.dispose();
      }
      const rotY = orientation === 'b' ? Math.PI : 0;
      const rimCoord = (BOARD_SIZE + FRAME_WIDTH) / 2;
      for (let i = 0; i < 8; i++) {
        const fileChar = FILES[i].toUpperCase();
        const fileTex = createCoordTexture(THREE, fileChar, colorStr);
        if (fileTex) {
          const mat = new THREE.MeshBasicMaterial({ map: fileTex, transparent: true, opacity: 0.82, depthWrite: false });
          const zEdge = orientation === 'b' ? rimCoord : -rimCoord;
          const m = new THREE.Mesh(coordGeom, mat);
          m.position.set((3.5 - i) * SQUARE, frameTopY + 0.0006, zEdge);
          m.rotation.y = rotY;
          coordGroup.add(m);
          coordMeshes.push({ mesh: m, material: mat, texture: fileTex });
        }
        const rankChar = String(i + 1);
        const rankTex = createCoordTexture(THREE, rankChar, colorStr);
        if (rankTex) {
          const mat = new THREE.MeshBasicMaterial({ map: rankTex, transparent: true, opacity: 0.82, depthWrite: false });
          const xEdge = orientation === 'b' ? -rimCoord : rimCoord;
          const m = new THREE.Mesh(coordGeom, mat);
          m.position.set(xEdge, frameTopY + 0.0006, (i - 3.5) * SQUARE);
          m.rotation.y = rotY;
          coordGroup.add(m);
          coordMeshes.push({ mesh: m, material: mat, texture: rankTex });
        }
      }
    }

    let introTween = null;
    const controls = window.THREE.OrbitControls
      ? new window.THREE.OrbitControls(camera, renderer.domElement)
      : null;
    if (controls) {
      controls.enableDamping = true;
      controls.dampingFactor = 0.09;
      controls.enablePan = false;
      controls.minDistance = 0.34;
      controls.maxDistance = 1.8;
      controls.minPolarAngle = 0.2;
      controls.maxPolarAngle = 1.32;
      controls.target.set(0, 0.02, 0);
      controls.update();
      controls.addEventListener('start', () => {
        state.userMoved = true;
        if (introTween) { introTween.duration = 0; introTween = null; }
      });
    }

    const state = {
      prototypes: new Map(),
      pieces: new Map(),
      markers: [],
      selected: null,
      hovered: null,
      targets: [],
      fen: '',
      skin: null,
      coordSide: null,
      orientation: 0,
      flipTween: null,
      tweens: [],
      loaded: false,
      built: false,
      failed: false,
      disposed: false,
      frame: 0,
      ready: false,
      userMoved: false,
      introPending: false
    };

    function squarePosition(name) {
      const coords = squareCoords(name);
      return new THREE.Vector3(
        (3.5 - coords.file) * SQUARE,
        PIECE_LIFT,
        (coords.rank - 3.5) * SQUARE
      );
    }

    function applySkin(skin, orientationSide) {
      const palette = SKINS[skin] || SKINS.green;
      squares.forEach((mesh) => {
        const coords = squareCoords(mesh.userData.square);
        const dark = (coords.file + coords.rank) % 2 === 0;
        mesh.material.color.setHex(dark ? palette.dark : palette.light);
        mesh.material.roughness = dark ? 0.56 : 0.64;
      });
      baseMaterial.color.setHex(palette.frame);
      frameMaterial.color.setHex(palette.frameHi);
      brassMaterial.color.setHex(palette.brass);
      floorMaterial.color.setHex(palette.floor);
      scene.background.setHex(palette.floor);
      if (scene.fog) scene.fog.color.setHex(palette.floor);
      rebuildCoordinates(palette.coord, orientationSide);
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

    function pieceMaterial(side) {
      const preset = PIECE_MATERIALS[side === 'w' ? 'w' : 'b'];
      return new THREE.MeshStandardMaterial({
        color: preset.color,
        roughness: preset.roughness,
        metalness: preset.metalness,
        envMapIntensity: side === 'w' ? 0.65 : 1.05
      });
    }

    function createPiece(code, name) {
      const file = PIECE_FILES[code.toLowerCase()];
      const prototype = state.prototypes.get(file);
      if (!prototype) return null;
      const side = code === code.toUpperCase() ? 'w' : 'b';
      const holder = new THREE.Group();
      const mesh = prototype.clone(true);
      const material = pieceMaterial(side);
      mesh.traverse((node) => {
        if (node.isMesh) {
          node.material = material;
          node.castShadow = true;
          node.receiveShadow = true;
        }
      });
      // Tournament baize felt base pad
      const padMat = new THREE.MeshStandardMaterial({
        color: PIECE_MATERIALS[side].pad,
        roughness: 0.92,
        metalness: 0
      });
      const pad = new THREE.Mesh(padGeometry, padMat);
      pad.position.y = 0.0005;
      holder.add(pad);
      holder.add(mesh);
      holder.position.copy(squarePosition(name));
      pieceGroup.add(holder);
      return { mesh: holder, code: code, material: material, padMaterial: padMat };
    }

    function removePiece(entry) {
      if (!entry) return;
      pieceGroup.remove(entry.mesh);
      entry.material.dispose();
      if (entry.padMaterial) entry.padMaterial.dispose();
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
        entry.mesh.position.y += Math.sin(progress * Math.PI) * 0.016;
      });
    }

    function spawn(entry) {
      if (reduced) return;
      const basePos = entry.mesh.position.clone();
      entry.mesh.scale.setScalar(0.001);
      tween(MOVE_MS, (progress) => {
        const eased = easeInOut(progress);
        entry.mesh.scale.setScalar(0.001 + eased * 0.999);
        entry.mesh.position.copy(basePos);
        entry.mesh.position.y += (1 - eased) * 0.03;
      });
    }

    function updateSelectedLift() {
      state.pieces.forEach((entry, sq) => {
        const targetY = (state.selected === sq && !reduced) ? (PIECE_LIFT + SELECT_LIFT) : PIECE_LIFT;
        if (!state.tweens.length) {
          entry.mesh.position.y = targetY;
        }
      });
    }

    function detectCastlingRookMove(lastMove, movingCode) {
      if (!lastMove || (movingCode !== 'K' && movingCode !== 'k')) return null;
      if (lastMove.from === 'e1' && lastMove.to === 'g1') return { from: 'h1', to: 'f1' };
      if (lastMove.from === 'e1' && lastMove.to === 'c1') return { from: 'a1', to: 'd1' };
      if (lastMove.from === 'e8' && lastMove.to === 'g8') return { from: 'h8', to: 'f8' };
      if (lastMove.from === 'e8' && lastMove.to === 'c8') return { from: 'a8', to: 'd8' };
      return null;
    }

    function syncPieces(position, lastMove) {
      const previous = state.pieces;
      const next = new Map();
      const live = new Set();
      previous.forEach((entry, name) => {
        if (position.get(name) === entry.code) {
          next.set(name, entry);
          live.add(entry);
        }
      });
      if (lastMove && !next.has(lastMove.to)) {
        const moving = previous.get(lastMove.from);
        const code = position.get(lastMove.to);
        if (moving && code && !live.has(moving)) {
          const captured = previous.get(lastMove.to);
          if (captured && captured !== moving) removePiece(captured);
          if (code === moving.code) {
            animateTo(moving, lastMove.to);
            next.set(lastMove.to, moving);
            live.add(moving);
            const rookCastle = detectCastlingRookMove(lastMove, moving.code);
            if (rookCastle && !next.has(rookCastle.to)) {
              const rookEntry = previous.get(rookCastle.from);
              if (rookEntry && !live.has(rookEntry) && position.get(rookCastle.to) === rookEntry.code) {
                animateTo(rookEntry, rookCastle.to);
                next.set(rookCastle.to, rookEntry);
                live.add(rookEntry);
              }
            }
          } else {
            removePiece(moving);
          }
        }
      }
      position.forEach((code, name) => {
        if (next.has(name)) return;
        const entry = createPiece(code, name);
        if (!entry) return;
        spawn(entry);
        next.set(name, entry);
        live.add(entry);
      });
      previous.forEach((entry) => {
        if (!live.has(entry)) removePiece(entry);
      });
      state.pieces = next;
      updateSelectedLift();
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

    function renderMarkers(snapshotData) {
      clearMarkers();
      if (snapshotData.lastMove) {
        addMarker(markerGeometry, snapshotData.lastMove.from, 0xf2b84b, 0.34);
        addMarker(markerGeometry, snapshotData.lastMove.to, 0xf2b84b, 0.42);
      }
      if (snapshotData.hintMove && snapshotData.hintMove.length >= 4) {
        addMarker(ringGeometry, snapshotData.hintMove.slice(0, 2), 0x27c7a6, 0.72);
        addMarker(ringGeometry, snapshotData.hintMove.slice(2, 4), 0x27c7a6, 0.85);
      }
      if (snapshotData.checkSquare) {
        addMarker(markerGeometry, snapshotData.checkSquare, 0xe64848, 0.52);
      }
      if (state.selected) {
        addMarker(markerGeometry, state.selected, 0x3b9eff, 0.38);
      }
      if (state.hovered && state.hovered !== state.selected) {
        addMarker(markerGeometry, state.hovered, 0xffffff, 0.14);
      }
      state.targets.forEach((target) => {
        if (target.captured) {
          addMarker(ringGeometry, target.to, 0x16221b, 0.62);
        } else {
          addMarker(dotGeometry, target.to, 0x16221b, 0.52);
        }
      });
    }

    function snapshot() {
      const game = app.game;
      if (!game) return null;
      const liveFen = typeof game.fen === 'function' ? game.fen() : '';
      const fen = (app.reviewActive && app.reviewFen) ? app.reviewFen : liveFen;
      const turn = typeof game.turn === 'function' ? game.turn() : 'w';
      const inCheck = !app.reviewActive && (typeof game.inCheck === 'function'
        ? !!game.inCheck()
        : (typeof game.in_check === 'function' ? !!game.in_check() : false));
      const player = app.playerColor === 'black' ? 'b' : 'w';
      const orientation = app.flipped ? (player === 'w' ? 'b' : 'w') : player;
      return {
        fen: fen,
        turn: turn,
        mode: app.mode || 'computer',
        flipped: !!app.flipped,
        orientation: orientation,
        skin: app.boardSkin || 'green',
        lastMove: app.reviewActive ? (app.reviewMove || null) : (app.lastMove || null),
        hintMove: app.reviewActive ? null : (app.hintMove || null),
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
      if (current.skin !== state.skin || current.orientation !== state.coordSide) {
        state.skin = current.skin;
        state.coordSide = current.orientation;
        applySkin(current.skin, current.orientation);
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
      state.selected = null;
      state.targets = [];
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

    function canSelectPieceAt(current, name) {
      if (!current || current.review) return false;
      const canMove = typeof api.canMove === 'function' ? api.canMove() : false;
      if (!canMove) return false;
      const piece = state.pieces.get(name);
      if (!piece) return false;
      const pieceSide = piece.code === piece.code.toUpperCase() ? 'w' : 'b';
      const allowedSide = current.mode === 'local' ? current.turn : current.playerColor;
      return pieceSide === allowedSide && current.turn === allowedSide;
    }

    function selectSquare(name) {
      const current = snapshot();
      if (!current) return;
      if (canSelectPieceAt(current, name)) {
        state.selected = name;
        state.targets = typeof api.legalMoves === 'function' ? api.legalMoves(name) : [];
        if (typeof api.onSelect === 'function') api.onSelect(name);
        updateSelectedLift();
        renderMarkers(current);
        return;
      }
      state.selected = null;
      state.targets = [];
      updateSelectedLift();
      renderMarkers(current);
    }

    function clickSquare(name) {
      const current = snapshot();
      if (!current) return;
      if (state.selected === name) {
        state.selected = null;
        state.targets = [];
        updateSelectedLift();
        renderMarkers(current);
        return;
      }
      const target = state.targets.find((entry) => entry.to === name);
      if (state.selected && target) {
        const from = state.selected;
        state.selected = null;
        state.targets = [];
        updateSelectedLift();
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

    function onPointerMove(event) {
      if (pointerDown) return;
      const current = snapshot();
      if (!current || current.review) {
        if (state.hovered) {
          state.hovered = null;
          renderer.domElement.style.cursor = '';
          if (current) renderMarkers(current);
        }
        return;
      }
      const sq = squareFromPointer(event);
      const interactive = sq && (canSelectPieceAt(current, sq) || state.targets.some(item => item.to === sq));
      renderer.domElement.style.cursor = interactive ? 'pointer' : 'grab';
      const nextHover = interactive ? sq : null;
      if (nextHover !== state.hovered) {
        state.hovered = nextHover;
        renderMarkers(current);
      }
    }

    function onPointerLeave() {
      if (state.hovered) {
        state.hovered = null;
        renderer.domElement.style.cursor = '';
        const current = snapshot();
        if (current) renderMarkers(current);
      }
    }

    renderer.domElement.addEventListener('pointerdown', onPointerDown);
    renderer.domElement.addEventListener('pointerup', onPointerUp);
    renderer.domElement.addEventListener('pointermove', onPointerMove);
    renderer.domElement.addEventListener('pointerleave', onPointerLeave);
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
      const halfWidth = half * Math.max(camera.aspect, 0.25);
      const direction = camera.position.clone().sub(target);
      if (direction.lengthSq() < 1e-8) direction.set(0, 0.62, -0.72);
      direction.normalize();
      let distance = 1;
      for (let step = 0; step < 4; step += 1) {
        camera.position.copy(target).addScaledVector(direction, distance);
        camera.lookAt(target);
        camera.updateMatrixWorld(true);
        let need = 0;
        for (let index = 0; index < fitCorners.length; index += 1) {
          const point = fitCorners[index].clone().applyMatrix4(camera.matrixWorldInverse);
          const depth = Math.max(-point.z, 0.05);
          need = Math.max(need, Math.abs(point.x) / depth / halfWidth, Math.abs(point.y) / depth / half);
        }
        const next = distance * need * FIT_MARGIN;
        if (Math.abs(next - distance) < 0.002) { distance = next; break; }
        distance = next;
      }
      camera.position.copy(target).addScaledVector(direction, distance);
      camera.lookAt(target);
      if (controls) {
        controls.minDistance = distance * 0.55;
        controls.maxDistance = distance * 2.2;
        controls.update();
      }
    }

    function defaultTilt(aspect) {
      if (aspect >= 2.1) return 0.34;
      if (aspect >= 1.6) return 0.46;
      return 0.62;
    }

    function applyDefaultAngle() {
      if (!state.userMoved) {
        const target = controls ? controls.target : new THREE.Vector3(0, 0.02, 0);
        const tilt = defaultTilt(camera.aspect);
        camera.position.set(0, tilt, -0.72).normalize().multiplyScalar(0.95);
        camera.lookAt(target);
      }
    }

    /* The camera glides in from a higher, wider stand when the view opens or resets. */
    function intro() {
      if (reduced || introTween) return;
      const target = controls ? controls.target.clone() : new THREE.Vector3(0, 0.02, 0);
      const endPos = camera.position.clone();
      const startPos = endPos.clone().sub(target).multiplyScalar(1.5);
      startPos.y = Math.max(startPos.y, 0.78);
      startPos.add(target);
      introTween = tween(820, (progress) => {
        camera.position.lerpVectors(startPos, endPos, easeInOut(progress));
        camera.lookAt(target);
        if (progress >= 1) introTween = null;
      });
    }

    function resetCamera() {
      state.userMoved = false;
      if (controls) controls.target.set(0, 0.02, 0);
      applyDefaultAngle();
      fitCamera();
      intro();
    }

    function resize() {
      const width = container.clientWidth || 1;
      const height = container.clientHeight || 1;
      renderer.setSize(width, height);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
      applyDefaultAngle();
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
      /* The first frame compiles the shaders and can stall; the glide starts
         once that frame is on screen so the stall cannot eat it. */
      if (state.introPending) {
        state.introPending = false;
        intro();
      }
    }

    resize();
    state.introPending = true;
    loadPieces();
    state.ready = true;
    sync();
    state.frame = window.requestAnimationFrame(render);

    return {
      sync: sync,
      resetCamera: resetCamera,
      dispose: function () {
        if (state.disposed) return;
        state.disposed = true;
        window.cancelAnimationFrame(state.frame);
        window.removeEventListener('resize', resize);
        if (state.observer) state.observer.disconnect();
        renderer.domElement.removeEventListener('pointerdown', onPointerDown);
        renderer.domElement.removeEventListener('pointerup', onPointerUp);
        renderer.domElement.removeEventListener('pointermove', onPointerMove);
        renderer.domElement.removeEventListener('pointerleave', onPointerLeave);
        clearMarkers();
        while (coordMeshes.length) {
          const item = coordMeshes.pop();
          coordGroup.remove(item.mesh);
          item.material.dispose();
          if (item.texture) item.texture.dispose();
        }
        state.pieces.forEach((entry) => removePiece(entry));
        state.pieces.clear();
        squares.forEach((mesh) => mesh.material.dispose());
        baseMaterial.dispose();
        frameMaterial.dispose();
        brassMaterial.dispose();
        floorMaterial.dispose();
        squareGeometry.dispose();
        markerGeometry.dispose();
        dotGeometry.dispose();
        ringGeometry.dispose();
        padGeometry.dispose();
        coordGeom.dispose();
        cornerGeom.dispose();
        filletRing.dispose();
        ownedTextures.forEach(tex => tex.dispose());
        if (envTarget) envTarget.dispose();
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
    if (refs.resetCam) refs.resetCam.hidden = mode !== '3d';
    if (refs.board) refs.board.hidden = mode === '3d';
    if (refs.board3d) refs.board3d.hidden = mode !== '3d';
    if (refs.root) refs.root.classList.toggle('is-3d', mode === '3d');
    if (typeof state.refitBoard === 'function') state.refitBoard();
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
      } else {
        views.get(state).sync();
        return;
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
        if (typeof state.onViewChanged === 'function') state.onViewChanged(mode);
      });
    },
    setMode: function (state, mode) {
      if (mode === '2d' || mode === '3d') setViewMode(state, mode);
    },
    resetCamera: function (state) {
      const view = views.get(state);
      if (view && typeof view.resetCamera === 'function') view.resetCamera();
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
