// ─── Color Palette ──────────────────────────────────────────
const NODE_COLORS = {
  agent:       '#5eead4',
  tool:        '#86efac',
  mcp_server:  '#c4b5fd',
  data_source: '#fdba74',
  llm_endpoint:'#fde68a',
};

const EDGE_COLORS = {
  tool_call:   '#86efac',
  mcp_call:    '#c4b5fd',
  retrieval:   '#5eead4',
  write:       '#fdba74',
  model_call:  '#fde68a',
  spawn:       '#5eead4',
};

// Pre-computed THREE.Color for GPU vertex color updates
const NODE_COLORS_THREE = {};
for (const [t, hex] of Object.entries(NODE_COLORS)) {
  NODE_COLORS_THREE[t] = new window.THREE.Color(hex);
}
const DEFAULT_COLOR_THREE = new window.THREE.Color('#cccccc');

// Color lerp helper
function lerpColor(a, b, t) {
  const ar = parseInt(a.slice(1,3),16), ag = parseInt(a.slice(3,5),16), ab = parseInt(a.slice(5,7),16);
  const br = parseInt(b.slice(1,3),16), bg = parseInt(b.slice(3,5),16), bb = parseInt(b.slice(5,7),16);
  const r = Math.round(ar + (br-ar)*t), g = Math.round(ag + (bg-ag)*t), bl = Math.round(ab + (bb-ab)*t);
  return '#' + ((1<<24)|(r<<16)|(g<<8)|bl).toString(16).slice(1);
}

// ─── State ──────────────────────────────────────────────────
const state = {
  nodes: new Map(),      // id → node data
  edges: new Map(),      // key → edge data (includes _energy field)
  paused: false,
  pinned: null,
  filterText: '',
  showRaw: false,
  ws: null,
  eventBuffer: [],
  stats: {},
  graphDirty: false,
};

// ─── Edge Energy Config ─────────────────────────────────────
const EDGE_GLOW = {
  FIRE_BOOST: 0.8,     // energy added per firing (cumulative, stacks high)
  DECAY_RATE: 0.985,   // per-tick multiplier (50ms ticks → half-life ~2.3s)
  DECAY_FLOOR: 0.005,
  BASE_WIDTH: 0.15,    // Thin constellation-style base width
  MAX_WIDTH: 0.6,      // Maximum width under energy
};

// Desaturate edge colors (~60% saturation) for subtle constellation lines
const EDGE_COLORS_DESAT = {};
for (const [type, hex] of Object.entries(EDGE_COLORS)) {
  const c = new window.THREE.Color(hex);
  const hsl = {};
  c.getHSL(hsl);
  c.setHSL(hsl.h, hsl.s * 0.6, hsl.l);
  EDGE_COLORS_DESAT[type] = '#' + c.getHexString();
}

const pendingParticles = []; // edge keys that need a glow particle spawned
const activeParticles = [];  // { sprite, srcId, dstId, startTime, duration }
const PARTICLE_DURATION = 1.5; // seconds to traverse full edge (slower, subtler)
const PARTICLE_SIZE_FACTOR = 0.3; // smaller particles for constellation look

// Reusable vectors for edge billboard math (avoid per-frame allocations)
const _edgeDir = new window.THREE.Vector3();
const _toCamera = new window.THREE.Vector3();
const _billUp = new window.THREE.Vector3();
const _faceNormal = new window.THREE.Vector3();
const _basisMat = new window.THREE.Matrix4();

// ─── 3d-force-graph Setup ───────────────────────────────────
const container = document.getElementById('graph-container');

const Graph = ForceGraph3D()(container)
  .backgroundColor('#0a0a1a')
  .nodeColor(node => NODE_COLORS[node.type] || '#cccccc')
  .nodeVal(node => Math.max(1, Math.log2((node.activityCount || 1) + 1) * 2))
  .nodeOpacity(node => {
    if (state.filterText) {
      const filter = state.filterText.toLowerCase();
      const match = node.id.toLowerCase().includes(filter) ||
                    node.label.toLowerCase().includes(filter) ||
                    node.type.toLowerCase().includes(filter);
      return match ? 1.0 : 0.08;
    }
    return node.stale ? 0.3 : 1.0;
  })
  .nodeLabel(node => `${node.type}: ${node.label}`)
  .linkThreeObject(link => {
    // Billboard ribbon mesh with line-glow texture + vertex color gradient
    const geo = new window.THREE.PlaneGeometry(1, 1, 1, 1);
    geo.setAttribute('color', new window.THREE.BufferAttribute(new Float32Array(12), 3));
    const mat = new window.THREE.MeshBasicMaterial({
      map: lineGlowTexture,
      vertexColors: true,
      transparent: true,
      opacity: 0.08,
      blending: window.THREE.AdditiveBlending,
      depthWrite: false,
      side: window.THREE.DoubleSide,
    });
    return new window.THREE.Mesh(geo, mat);
  })
  .linkPositionUpdate((mesh, { start, end }, link) => {
    const dx = end.x - start.x, dy = end.y - start.y, dz = end.z - start.z;
    const len = Math.sqrt(dx * dx + dy * dy + dz * dz);
    if (len < 0.01) return true;

    // Position at edge midpoint
    mesh.position.set(
      (start.x + end.x) / 2,
      (start.y + end.y) / 2,
      (start.z + end.z) / 2
    );

    // Cylindrical billboard: ribbon faces camera while stretching along edge
    _edgeDir.set(dx, dy, dz).divideScalar(len);
    const cam = Graph.camera().position;
    _toCamera.set(
      cam.x - mesh.position.x,
      cam.y - mesh.position.y,
      cam.z - mesh.position.z
    ).normalize();

    _billUp.crossVectors(_edgeDir, _toCamera);
    if (_billUp.lengthSq() < 0.0001) {
      // Edge points directly at camera — use fallback up
      _billUp.set(0, 1, 0);
      _billUp.crossVectors(_edgeDir, _billUp);
    }
    _billUp.normalize();
    _faceNormal.crossVectors(_edgeDir, _billUp).normalize();

    // Orient: local X → edge direction, local Y → billboard up, local Z → face normal
    _basisMat.makeBasis(_edgeDir, _billUp, _faceNormal);
    mesh.quaternion.setFromRotationMatrix(_basisMat);

    // Vertex color gradient: source color → destination color
    // PlaneGeometry(1,1,1,1) vertices: 0=top-left, 1=top-right, 2=bot-left, 3=bot-right
    // X maps left→right = source→destination
    const srcId = typeof link.source === 'object' ? link.source.id : link.source;
    const dstId = typeof link.target === 'object' ? link.target.id : link.target;
    const srcNode = state.nodes.get(srcId);
    const dstNode = state.nodes.get(dstId);
    const srcColor = NODE_COLORS_THREE[(srcNode && srcNode.type)] || DEFAULT_COLOR_THREE;
    const dstColor = NODE_COLORS_THREE[(dstNode && dstNode.type)] || DEFAULT_COLOR_THREE;

    const col = mesh.geometry.attributes.color;
    col.array[0] = srcColor.r; col.array[1]  = srcColor.g; col.array[2]  = srcColor.b;
    col.array[3] = dstColor.r; col.array[4]  = dstColor.g; col.array[5]  = dstColor.b;
    col.array[6] = srcColor.r; col.array[7]  = srcColor.g; col.array[8]  = srcColor.b;
    col.array[9] = dstColor.r; col.array[10] = dstColor.g; col.array[11] = dstColor.b;
    col.needsUpdate = true;

    // Scale: thin constellation lines with energy-driven width
    const edgeData = state.edges.get(link.id);
    const energy = edgeData ? edgeData._energy : 0;
    const glowWidth = Math.min(EDGE_GLOW.MAX_WIDTH, EDGE_GLOW.BASE_WIDTH + energy * 0.5);
    mesh.scale.set(len, glowWidth, 1);

    // Opacity: subtle baseline, flashes on activity
    mesh.material.opacity = Math.min(1.0, 0.12 + energy * 0.9);

    return true;
  })
  .linkLabel(link => `${link.type} (${link.callCount || 1}x)`)
  .onNodeClick(node => {
    state.pinned = { type: 'node', id: node.id };
    showDetail(state.pinned);
  })
  .onBackgroundClick(() => {
    state.pinned = null;
    hideDetail();
  });

// ─── Multi-Layer Starfield ────────────────────────────────────
function buildStarfieldLayer(count, spread, size, color) {
  const geo = new window.THREE.BufferGeometry();
  const positions = new Float32Array(count * 3);
  for (let i = 0; i < count * 3; i++) {
    positions[i] = (Math.random() - 0.5) * spread * 2;
  }
  geo.setAttribute('position', new window.THREE.BufferAttribute(positions, 3));

  let sizes;
  if (typeof size === 'object') {
    // Random size range { min, max }
    sizes = new Float32Array(count);
    for (let i = 0; i < count; i++) {
      sizes[i] = size.min + Math.random() * (size.max - size.min);
    }
    geo.setAttribute('size', new window.THREE.BufferAttribute(sizes, 1));
  }

  const mat = new window.THREE.PointsMaterial({
    color: new window.THREE.Color(color),
    size: typeof size === 'number' ? size : size.min,
    sizeAttenuation: true,
    transparent: true,
    opacity: 0.9,
    depthWrite: false,
  });
  const points = new window.THREE.Points(geo, mat);
  points.layers.set(0); // Non-bloom layer
  return points;
}

function buildBrightStars(count, spread) {
  // Cross-shaped sprite texture for landmark stars
  const canvas = document.createElement('canvas');
  canvas.width = 64;
  canvas.height = 64;
  const ctx = canvas.getContext('2d');
  const cx = 32, cy = 32;

  // Core glow
  const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, 28);
  grad.addColorStop(0, 'rgba(255,255,255,1.0)');
  grad.addColorStop(0.15, 'rgba(255,255,255,0.8)');
  grad.addColorStop(0.4, 'rgba(255,255,255,0.15)');
  grad.addColorStop(1.0, 'rgba(255,255,255,0.0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, 64, 64);

  // Cross spikes (subtle)
  ctx.globalAlpha = 0.5;
  ctx.strokeStyle = '#ffffff';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(cx, 2); ctx.lineTo(cx, 62);
  ctx.moveTo(2, cy); ctx.lineTo(62, cy);
  ctx.stroke();
  ctx.globalAlpha = 1.0;

  const tex = new window.THREE.CanvasTexture(canvas);
  const group = new window.THREE.Group();
  group.layers.set(0);

  for (let i = 0; i < count; i++) {
    const mat = new window.THREE.SpriteMaterial({
      map: tex,
      color: 0xffffff,
      transparent: true,
      opacity: 0.7 + Math.random() * 0.3,
      depthWrite: false,
    });
    const sprite = new window.THREE.Sprite(mat);
    sprite.position.set(
      (Math.random() - 0.5) * spread * 2,
      (Math.random() - 0.5) * spread * 2,
      (Math.random() - 0.5) * spread * 2
    );
    const s = 2.5 + Math.random() * 1.5;
    sprite.scale.set(s, s, 1);
    sprite.layers.set(0);
    group.add(sprite);
  }
  return group;
}

(function createStarfield() {
  const scene = Graph.scene();
  scene.add(buildStarfieldLayer(8000, 3000, 0.3, '#333355'));   // Far
  scene.add(buildStarfieldLayer(3000, 2000, 0.6, '#555577'));   // Mid
  scene.add(buildStarfieldLayer(500, 1000, { min: 1.0, max: 2.0 }, '#8888aa')); // Near
  scene.add(buildBrightStars(20, 1200));  // Bright landmarks
})();

// ─── Star-Point Node Textures ────────────────────────────────
function createCoreTexture(size) {
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  const c = size / 2;
  const grad = ctx.createRadialGradient(c, c, 0, c, c, c);
  grad.addColorStop(0, 'rgba(255,255,255,1.0)');
  grad.addColorStop(0.2, 'rgba(255,255,255,1.0)');
  grad.addColorStop(0.4, 'rgba(255,255,255,0.6)');
  grad.addColorStop(0.7, 'rgba(255,255,255,0.1)');
  grad.addColorStop(1.0, 'rgba(255,255,255,0.0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, size, size);
  return new window.THREE.CanvasTexture(canvas);
}

function createHaloTexture(size) {
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  const c = size / 2;
  const grad = ctx.createRadialGradient(c, c, 0, c, c, c);
  grad.addColorStop(0, 'rgba(255,255,255,0.5)');
  grad.addColorStop(0.2, 'rgba(255,255,255,0.3)');
  grad.addColorStop(0.5, 'rgba(255,255,255,0.08)');
  grad.addColorStop(0.8, 'rgba(255,255,255,0.02)');
  grad.addColorStop(1.0, 'rgba(255,255,255,0.0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, size, size);
  return new window.THREE.CanvasTexture(canvas);
}

function createDiffractionTexture(size) {
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  const c = size / 2;

  // 4-point cross spikes
  ctx.strokeStyle = '#ffffff';
  ctx.lineWidth = 1.5;
  ctx.globalAlpha = 0.6;
  ctx.beginPath();
  ctx.moveTo(c, 4); ctx.lineTo(c, size - 4);
  ctx.moveTo(4, c); ctx.lineTo(size - 4, c);
  ctx.stroke();

  // Subtle center glow
  ctx.globalAlpha = 0.3;
  const grad = ctx.createRadialGradient(c, c, 0, c, c, c * 0.3);
  grad.addColorStop(0, 'rgba(255,255,255,0.5)');
  grad.addColorStop(1, 'rgba(255,255,255,0.0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, size, size);

  return new window.THREE.CanvasTexture(canvas);
}

const coreTexture = createCoreTexture(64);
const haloTexture = createHaloTexture(128);
const diffractionTexture = createDiffractionTexture(128);

// Line glow: vertical gradient for edge ribbons
function createLineGlowTexture(size) {
  const canvas = document.createElement('canvas');
  canvas.width = 4;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  const grad = ctx.createLinearGradient(0, 0, 0, size);
  grad.addColorStop(0, 'rgba(255,255,255,0)');
  grad.addColorStop(0.25, 'rgba(255,255,255,0.06)');
  grad.addColorStop(0.45, 'rgba(255,255,255,0.5)');
  grad.addColorStop(0.5, 'rgba(255,255,255,1)');
  grad.addColorStop(0.55, 'rgba(255,255,255,0.5)');
  grad.addColorStop(0.75, 'rgba(255,255,255,0.06)');
  grad.addColorStop(1, 'rgba(255,255,255,0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  return new window.THREE.CanvasTexture(canvas);
}

const lineGlowTexture = createLineGlowTexture(64);

// Material cache for glow particles
const glowMaterialCache = new Map();
function getGlowMaterial(hexColor) {
  if (glowMaterialCache.has(hexColor)) return glowMaterialCache.get(hexColor);
  const mat = new window.THREE.SpriteMaterial({
    map: coreTexture,
    color: new window.THREE.Color(hexColor),
    transparent: true,
    depthWrite: false,
    blending: window.THREE.AdditiveBlending,
  });
  glowMaterialCache.set(hexColor, mat);
  return mat;
}

// Track max activity for relative glow brightness
let maxActivityCount = 1;

Graph.nodeThreeObject(node => {
  const color = NODE_COLORS[node.type] || '#cccccc';
  const activity = node.activityCount || 1;
  if (activity > maxActivityCount) maxActivityCount = activity;

  const coreSize = Math.min(20, 4 + Math.log2(activity + 1) * 2);
  const haloSize = coreSize * 3.5;

  // Opacity based on filter/stale state
  let opacity = 1.0;
  if (state.filterText) {
    const filter = state.filterText.toLowerCase();
    const match = node.id.toLowerCase().includes(filter) ||
                  node.label.toLowerCase().includes(filter) ||
                  node.type.toLowerCase().includes(filter);
    opacity = match ? 1.0 : 0.08;
  } else if (node.stale) {
    opacity = 0.3;
  }

  const group = new window.THREE.Group();

  // Halo sprite (larger, softer glow — bloom-eligible)
  const haloMat = new window.THREE.SpriteMaterial({
    map: haloTexture,
    color: new window.THREE.Color(color),
    transparent: true,
    opacity: Math.min(0.8, Math.max(0.15, activity / Math.max(maxActivityCount, 1))) * opacity,
    depthWrite: false,
    blending: window.THREE.AdditiveBlending,
  });
  const halo = new window.THREE.Sprite(haloMat);
  halo.scale.set(haloSize, haloSize, 1);
  halo.layers.enable(1); // Bloom layer
  group.add(halo);

  // Core sprite (bright center)
  const coreMat = new window.THREE.SpriteMaterial({
    map: coreTexture,
    color: new window.THREE.Color(color),
    transparent: true,
    opacity: opacity,
    depthWrite: false,
    blending: window.THREE.AdditiveBlending,
  });
  const core = new window.THREE.Sprite(coreMat);
  core.scale.set(coreSize, coreSize, 1);
  core.layers.enable(1); // Bloom layer
  group.add(core);

  // Diffraction spikes for high-activity nodes
  if (activity > 10) {
    const diffMat = new window.THREE.SpriteMaterial({
      map: diffractionTexture,
      color: new window.THREE.Color(color),
      transparent: true,
      opacity: Math.min(0.6, (activity - 10) / 40) * opacity,
      depthWrite: false,
      blending: window.THREE.AdditiveBlending,
    });
    const diff = new window.THREE.Sprite(diffMat);
    const diffSize = coreSize * 2.5;
    diff.scale.set(diffSize, diffSize, 1);
    diff.layers.enable(1);
    group.add(diff);
  }

  // Store reference for label system
  group.userData = { nodeId: node.id, nodeType: node.type };

  return group;
});

// ─── Token Animation System (DOM overlay) ───────────────────
const TOKEN_ANIM = {
  DURATION: 3.0,
  BOUNCE_END: 0.2,
  FADE_START: 0.4,
  DRIFT_PX: 120,
  H_OFFSET_PX: 30,
  STAGGER_PX: 28,
  MAX_PER_NODE: 3,
  MAX_GLOBAL: 50,
  INPUT_COLOR: '#5eead4',
  OUTPUT_COLOR: '#fde68a',
};

const tokenLayer = document.createElement('div');
tokenLayer.id = 'token-layer';
tokenLayer.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:5;overflow:hidden;';
document.body.appendChild(tokenLayer);

const labelsByNode = new Map();
const allLabels = [];

function formatTokenCount(n) {
  return n.toLocaleString('en-US');
}

function spawnTokenLabel(nodeId, count, isOutput) {
  if (!count || count <= 0) return;

  const nodeLabels = labelsByNode.get(nodeId) || [];

  if (nodeLabels.length >= TOKEN_ANIM.MAX_PER_NODE) {
    const recent = nodeLabels[nodeLabels.length - 1];
    recent.el.textContent = isOutput ? '+' + formatTokenCount(count) : formatTokenCount(count);
    recent.el.style.color = isOutput ? TOKEN_ANIM.OUTPUT_COLOR : TOKEN_ANIM.INPUT_COLOR;
    recent.spawnTime = performance.now() / 1000;
    return;
  }

  if (allLabels.length >= TOKEN_ANIM.MAX_GLOBAL) {
    removeLabel(allLabels[0]);
  }

  const graphData = Graph.graphData();
  const node = graphData.nodes.find(n => n.id === nodeId);
  if (!node) return;

  const text = isOutput ? '+' + formatTokenCount(count) : formatTokenCount(count);

  const el = document.createElement('div');
  el.textContent = text;
  el.style.cssText = `
    position: absolute;
    font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
    font-size: 14px;
    font-weight: 600;
    color: ${isOutput ? TOKEN_ANIM.OUTPUT_COLOR : TOKEN_ANIM.INPUT_COLOR};
    text-shadow: 0 0 8px ${isOutput ? TOKEN_ANIM.OUTPUT_COLOR : TOKEN_ANIM.INPUT_COLOR}80,
                 0 0 16px ${isOutput ? TOKEN_ANIM.OUTPUT_COLOR : TOKEN_ANIM.INPUT_COLOR}40;
    white-space: nowrap;
    opacity: 0;
    transform: scale(0.5);
    will-change: transform, opacity;
  `;
  tokenLayer.appendChild(el);

  const now = performance.now() / 1000;
  const staggerIdx = nodeLabels.length;
  const entry = { el, spawnTime: now, nodeId, isOutput, staggerIdx,
                   worldX: node.x || 0, worldY: node.y || 0, worldZ: node.z || 0 };

  nodeLabels.push(entry);
  labelsByNode.set(nodeId, nodeLabels);
  allLabels.push(entry);
}

function removeLabel(entry) {
  entry.el.remove();

  const nodeLabels = labelsByNode.get(entry.nodeId);
  if (nodeLabels) {
    const idx = nodeLabels.indexOf(entry);
    if (idx >= 0) nodeLabels.splice(idx, 1);
    if (nodeLabels.length === 0) labelsByNode.delete(entry.nodeId);
  }

  const gIdx = allLabels.indexOf(entry);
  if (gIdx >= 0) allLabels.splice(gIdx, 1);
}

function elasticOut(t) {
  if (t <= 0) return 0;
  if (t >= 1) return 1;
  return Math.pow(2, -10 * t) * Math.sin((t - 0.075) * (2 * Math.PI) / 0.3) + 1;
}

// Always-on label state (must be before animate loop)
const LABEL_CONFIG = {
  TOP_N: 8,
  RECALC_MS: 2000,
  FADE_MS: 500,
  FONT_SIZE: 3,
  OPACITY: 0.5,
};
const activeLabels = new Map(); // nodeId → { sprite, fadeStart }

let lastDecayTime = performance.now();

function animate() {
  requestAnimationFrame(animate);

  const now = performance.now();

  // ── Energy decay (time-based, does NOT touch graphDirty) ──
  const dt = now - lastDecayTime;
  lastDecayTime = now;
  const decayFactor = Math.pow(EDGE_GLOW.DECAY_RATE, dt / 50);
  for (const edge of state.edges.values()) {
    if (edge._energy > 0) {
      edge._energy *= decayFactor;
      if (edge._energy < EDGE_GLOW.DECAY_FLOOR) edge._energy = 0;
    }
  }

  // ── Custom glow particles (same look as nodes, half size) ──
  const nowSec = now / 1000;

  // Spawn pending particles
  while (pendingParticles.length > 0) {
    const edgeKey = pendingParticles.shift();
    const edge = state.edges.get(edgeKey);
    if (!edge) continue;
    const srcNode = state.nodes.get(edge.src);
    if (!srcNode) continue;
    const color = NODE_COLORS[srcNode.type] || '#cccccc';
    const mat = getGlowMaterial(color).clone();
    const sprite = new window.THREE.Sprite(mat);
    const baseSize = 6 + Math.log2((srcNode.activityCount || 1) + 1) * 2;
    const pSize = baseSize * PARTICLE_SIZE_FACTOR;
    sprite.scale.set(pSize, pSize, 1);
    Graph.scene().add(sprite);
    activeParticles.push({
      sprite, srcId: edge.src, dstId: edge.dst,
      startTime: nowSec, duration: PARTICLE_DURATION,
    });
  }

  // Animate active particles along their edges
  if (activeParticles.length > 0) {
    const graphNodes = Graph.graphData().nodes;
    const nodeById = new Map();
    for (const n of graphNodes) nodeById.set(n.id, n);

    for (let i = activeParticles.length - 1; i >= 0; i--) {
      const p = activeParticles[i];
      const t = (nowSec - p.startTime) / p.duration;

      if (t >= 1) {
        Graph.scene().remove(p.sprite);
        p.sprite.material.dispose();
        activeParticles.splice(i, 1);
        continue;
      }

      const src = nodeById.get(p.srcId);
      const dst = nodeById.get(p.dstId);
      if (!src || !dst || src.x == null || dst.x == null) continue;

      // Lerp position along edge
      p.sprite.position.set(
        src.x + (dst.x - src.x) * t,
        src.y + (dst.y - src.y) * t,
        src.z + (dst.z - src.z) * t
      );

      // Color gradient: source → destination as particle travels
      const srcN = state.nodes.get(p.srcId);
      const dstN = state.nodes.get(p.dstId);
      const srcC = NODE_COLORS_THREE[(srcN && srcN.type)] || DEFAULT_COLOR_THREE;
      const dstC = NODE_COLORS_THREE[(dstN && dstN.type)] || DEFAULT_COLOR_THREE;
      p.sprite.material.color.copy(srcC).lerp(dstC, t);

      // Fade in at start, fade out at end
      const fade = t < 0.1 ? t / 0.1 : t > 0.85 ? (1 - t) / 0.15 : 1;
      p.sprite.material.opacity = fade;
    }
  }

  // ── Always-on label fading ──
  tickLabels();

  // ── Token label animation ──

  for (let i = allLabels.length - 1; i >= 0; i--) {
    const entry = allLabels[i];
    const elapsed = nowSec - entry.spawnTime;

    if (elapsed >= TOKEN_ANIM.DURATION) {
      removeLabel(entry);
      continue;
    }

    const screen = Graph.graph2ScreenCoords(entry.worldX, entry.worldY, entry.worldZ);
    if (!screen) continue;

    const hOff = entry.isOutput ? TOKEN_ANIM.H_OFFSET_PX : -TOKEN_ANIM.H_OFFSET_PX;
    const staggerOff = entry.staggerIdx * TOKEN_ANIM.STAGGER_PX;

    let opacity = 1;
    let scale = 1;
    const progress = elapsed / TOKEN_ANIM.DURATION;
    const driftY = -TOKEN_ANIM.DRIFT_PX * Math.sqrt(progress);

    if (elapsed < TOKEN_ANIM.BOUNCE_END) {
      const t = elapsed / TOKEN_ANIM.BOUNCE_END;
      opacity = t;
      scale = 0.7 + t * 0.3;
    } else {
      const fadeProgress = Math.max(0, (elapsed - TOKEN_ANIM.FADE_START) / (TOKEN_ANIM.DURATION - TOKEN_ANIM.FADE_START));
      opacity = 1 - fadeProgress * fadeProgress;
      scale = 1;
    }

    const x = screen.x + hOff;
    const y = screen.y + driftY - staggerOff - 20;

    entry.el.style.opacity = opacity;
    entry.el.style.transform = `translate(${x}px, ${y}px) translate(-50%, -100%) scale(${scale})`;
  }
}

animate();

// ─── Graph Data Sync ────────────────────────────────────────
function syncGraphData() {
  if (!state.graphDirty) return;
  state.graphDirty = false;

  const nodes = Array.from(state.nodes.values());
  const links = Array.from(state.edges.values()).map(e => ({
    source: e.src,
    target: e.dst,
    ...e,
  }));

  Graph.graphData({ nodes, links });
}

setInterval(syncGraphData, 200);

// ─── Node/Edge Management ───────────────────────────────────
function addOrUpdateNode(data) {
  const existing = state.nodes.get(data.id);
  if (existing) {
    Object.assign(existing, data);
  } else {
    state.nodes.set(data.id, { ...data });
  }
  state.graphDirty = true;
}

function addOrUpdateEdge(data) {
  const key = data.src + '|' + data.dst + '|' + data.type;
  const existing = state.edges.get(key);
  if (existing) {
    Object.assign(existing, data);
    existing._energy = (existing._energy || 0) + EDGE_GLOW.FIRE_BOOST;
  } else {
    state.edges.set(key, { ...data, id: key, _energy: EDGE_GLOW.FIRE_BOOST });
  }
  pendingParticles.push(key);
  state.graphDirty = true;
}

// ─── Detail Card ────────────────────────────────────────────
const detailCard = document.getElementById('detail-card');
const detailContent = document.getElementById('detail-content');

function showDetail(obj) {
  detailCard.style.display = 'block';
  if (obj.type === 'node') {
    const d = state.nodes.get(obj.id);
    if (!d) return;
    const pipColor = NODE_COLORS[d.type] || '#cccccc';
    let html = `<div class="detail-type"><span class="detail-pip" style="background:${pipColor};box-shadow:0 0 6px ${pipColor}"></span>${d.type.toUpperCase()}: ${escapeHtml(d.label)}</div>`;
    html += row('ID', d.id);
    html += row('Type', d.type);
    html += row('Activity', d.activityCount);
    if (d.cumulativeTokens) {
      html += `<div class="detail-row"><span class="detail-key">Tokens</span><span class="detail-value tokens-in">${formatNum(d.cumulativeTokens)}</span></div>`;
    }
    if (d.cumulativeCost) {
      html += `<div class="detail-row"><span class="detail-key">Cost</span><span class="detail-value cost">$${d.cumulativeCost.toFixed(4)}</span></div>`;
    }
    html += row('Last Seen', d.lastSeen || '—');
    html += row('Stale', d.stale ? 'Yes' : 'No');
    if (state.showRaw && d.attributes) {
      html += '<div style="margin-top:8px;color:#888">Attributes:</div>';
      for (const [k, v] of Object.entries(d.attributes)) {
        html += row(k, JSON.stringify(v));
      }
    }
    detailContent.innerHTML = html;
  }
}

function hideDetail() {
  detailCard.style.display = 'none';
}

function row(key, value) {
  return `<div class="detail-row"><span class="detail-key">${escapeHtml(key)}</span><span class="detail-value">${escapeHtml(String(value))}</span></div>`;
}

function escapeHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// ─── HUD Updates ────────────────────────────────────────────
function updateHUD() {
  document.getElementById('hud-nodes').textContent = state.nodes.size;
  document.getElementById('hud-edges').textContent = state.edges.size;

  if (state.stats.eventsPerSec !== undefined) {
    document.getElementById('hud-eps').textContent = state.stats.eventsPerSec.toFixed(1);
  }
  if (state.stats.errorCount !== undefined) {
    document.getElementById('hud-errors').textContent = state.stats.errorCount;
  }
  if (state.stats.avgLatencyMs !== undefined) {
    document.getElementById('hud-latency').textContent = state.stats.avgLatencyMs.toFixed(1) + 'ms';
  }

  const statusEl = document.getElementById('hud-status');
  if (state.paused) {
    statusEl.textContent = 'paused';
    statusEl.style.color = '#f7768e';
  } else if (state.ws && state.ws.readyState === WebSocket.OPEN) {
    statusEl.textContent = state.stats.mode || 'connected';
    statusEl.style.color = '#86efac';
  } else {
    statusEl.textContent = 'disconnected';
    statusEl.style.color = '#f7768e';
  }

  const cappedRow = document.getElementById('hud-capped-row');
  if (state.stats.capped) {
    cappedRow.style.display = 'flex';
  }

  const droppedRow = document.getElementById('hud-dropped-row');
  if (state.stats.dropped > 0) {
    droppedRow.style.display = 'flex';
    document.getElementById('hud-dropped').textContent = state.stats.dropped;
  }

  const samplingRow = document.getElementById('hud-sampling-row');
  samplingRow.style.display = state.stats.sampling ? 'flex' : 'none';
}

setInterval(updateHUD, 500);

// ─── WebSocket ──────────────────────────────────────────────
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${proto}//${location.host}/ws`);
  state.ws = ws;

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (state.paused) {
      state.eventBuffer.push(msg);
      return;
    }
    handleMessage(msg);
  };

  ws.onclose = () => {
    setTimeout(connectWS, 2000);
  };

  ws.onerror = () => {
    ws.close();
  };
}

function handleMessage(msg) {
  switch (msg.type) {
    case 'snapshot':
      handleSnapshot(msg.data);
      break;
    case 'update':
      handleUpdate(msg.data);
      break;
    case 'stats':
      state.stats = msg.data;
      break;
  }
}

function handleSnapshot(data) {
  // A snapshot replaces the entire graph state (e.g. after a reset)
  state.nodes.clear();
  state.edges.clear();

  if (data.nodes) {
    for (const n of data.nodes) {
      addOrUpdateNode(n);
    }
  }
  if (data.edges) {
    for (const e of data.edges) {
      addOrUpdateEdge(e);
    }
  }
  state.graphDirty = true;
}

function handleUpdate(data) {
  if (data.nodes) {
    for (const n of data.nodes) {
      addOrUpdateNode(n);
      if (recording.active) {
        recording.nodesCreated.add(n.id);
      }
    }
  }
  if (data.edges) {
    for (const e of data.edges) {
      addOrUpdateEdge(e);
      if (e.type === 'model_call' && e.attributes) {
        const inTok = e.attributes.input_tokens;
        const outTok = e.attributes.output_tokens;
        if (inTok && inTok > 0) spawnTokenLabel(e.dst, inTok, false);
        if (outTok && outTok > 0) spawnTokenLabel(e.dst, outTok, true);
      }
      // Track during recording
      if (recording.active) {
        recording.totalEvents++;
        recording.edgesSeen.add(e.src + '|' + e.dst + '|' + e.type);
        if (e.type === 'model_call') {
          recording.apiRequests++;
          if (e.attributes) {
            recording.inputTokens += Number(e.attributes.input_tokens) || 0;
            recording.outputTokens += Number(e.attributes.output_tokens) || 0;
            recording.cost += Number(e.attributes.cost_usd) || 0;
            const model = e.attributes.model || 'unknown';
            recording.models[model] = (recording.models[model] || 0) + 1;
          }
        } else if (e.type === 'tool_call') {
          const tool = (e.attributes && e.attributes.tool_name) || e.dst || 'unknown';
          recording.toolCalls[tool] = (recording.toolCalls[tool] || 0) + 1;
        } else if (e.type === 'mcp_call') {
          const server = (e.attributes && e.attributes.server_name) || e.dst || 'unknown';
          recording.mcpCalls[server] = (recording.mcpCalls[server] || 0) + 1;
        }
      }
    }
  }
}

// ─── Controls ───────────────────────────────────────────────
document.getElementById('btn-pause').addEventListener('click', function() {
  state.paused = !state.paused;
  this.textContent = state.paused ? 'Resume' : 'Pause';
  this.classList.toggle('active', state.paused);
  if (!state.paused) {
    for (const msg of state.eventBuffer) {
      handleMessage(msg);
    }
    state.eventBuffer = [];
  }
});

document.getElementById('btn-reset').addEventListener('click', () => {
  Graph.cameraPosition({ x: 0, y: 0, z: 300 });
});

document.getElementById('btn-clear').addEventListener('click', () => {
  state.nodes.clear();
  state.edges.clear();
  state.pinned = null;
  document.getElementById('detail-card').style.display = 'none';
  state.graphDirty = true;
  syncGraphData();
});

document.getElementById('search').addEventListener('input', (e) => {
  state.filterText = e.target.value;
  Graph.nodeColor(Graph.nodeColor());
});

document.getElementById('chk-raw').addEventListener('change', (e) => {
  state.showRaw = e.target.checked;
  if (state.pinned) showDetail(state.pinned);
});

document.getElementById('detail-close').addEventListener('click', () => {
  state.pinned = null;
  hideDetail();
});

window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    state.pinned = null;
    hideDetail();
    document.getElementById('record-dashboard').style.display = 'none';
  }
});

// ─── Recording ──────────────────────────────────────────────
const recording = {
  active: false,
  startTime: 0,
  totalEvents: 0,
  apiRequests: 0,
  inputTokens: 0,
  outputTokens: 0,
  cost: 0,
  toolCalls: {},    // tool name → count
  mcpCalls: {},     // server name → count
  models: {},       // model name → count
  nodesCreated: new Set(),
  edgesSeen: new Set(),
};

function resetRecording() {
  recording.totalEvents = 0;
  recording.apiRequests = 0;
  recording.inputTokens = 0;
  recording.outputTokens = 0;
  recording.cost = 0;
  recording.toolCalls = {};
  recording.mcpCalls = {};
  recording.models = {};
  recording.nodesCreated = new Set();
  recording.edgesSeen = new Set();
}

function formatNum(n) {
  return n.toLocaleString('en-US');
}

function formatDuration(ms) {
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return m > 0 ? `${m}m ${sec}s` : `${sec}s`;
}

function showRecordDashboard() {
  const dur = Date.now() - recording.startTime;
  const totalToolCalls = Object.values(recording.toolCalls).reduce((a, b) => a + b, 0);
  const totalMcpCalls = Object.values(recording.mcpCalls).reduce((a, b) => a + b, 0);
  const totalTokens = recording.inputTokens + recording.outputTokens;

  let html = '<div class="dash-title">Recording Summary</div>';

  // Overview
  html += '<div class="dash-section">';
  html += '<div class="dash-section-title">Overview</div>';
  html += `<div class="dash-row"><span class="dash-key">Duration</span><span class="dash-val highlight">${formatDuration(dur)}</span></div>`;
  html += `<div class="dash-row"><span class="dash-key">Total Events</span><span class="dash-val">${formatNum(recording.totalEvents)}</span></div>`;
  html += `<div class="dash-row"><span class="dash-key">Nodes</span><span class="dash-val">${formatNum(recording.nodesCreated.size)}</span></div>`;
  html += `<div class="dash-row"><span class="dash-key">Edges</span><span class="dash-val">${formatNum(recording.edgesSeen.size)}</span></div>`;
  html += '</div>';

  // Tokens
  html += '<div class="dash-section">';
  html += '<div class="dash-section-title">Tokens</div>';
  html += `<div class="dash-row"><span class="dash-key">Input</span><span class="dash-val">${formatNum(recording.inputTokens)}</span></div>`;
  html += `<div class="dash-row"><span class="dash-key">Output</span><span class="dash-val">${formatNum(recording.outputTokens)}</span></div>`;
  html += `<div class="dash-row"><span class="dash-key">Total</span><span class="dash-val highlight">${formatNum(totalTokens)}</span></div>`;
  if (recording.cost > 0) {
    html += `<div class="dash-row"><span class="dash-key">Est. Cost</span><span class="dash-val cost">$${recording.cost.toFixed(4)}</span></div>`;
  }
  html += '</div>';

  // API Requests
  html += '<div class="dash-section">';
  html += '<div class="dash-section-title">LLM Calls</div>';
  html += `<div class="dash-row"><span class="dash-key">Requests</span><span class="dash-val">${formatNum(recording.apiRequests)}</span></div>`;
  const modelEntries = Object.entries(recording.models).sort((a, b) => b[1] - a[1]);
  for (const [model, count] of modelEntries) {
    html += `<div class="dash-row"><span class="dash-key">${escapeHtml(model)}</span><span class="dash-val">${formatNum(count)}</span></div>`;
  }
  html += '</div>';

  // Tool Calls
  if (totalToolCalls > 0) {
    html += '<div class="dash-section">';
    html += '<div class="dash-section-title">Tool Calls</div>';
    const toolEntries = Object.entries(recording.toolCalls).sort((a, b) => b[1] - a[1]);
    const maxToolCount = toolEntries[0] ? toolEntries[0][1] : 1;
    for (const [tool, count] of toolEntries) {
      const pct = (count / maxToolCount * 100).toFixed(0);
      const color = NODE_COLORS.tool;
      html += `<div class="dash-bar-row">`;
      html += `<span class="dash-bar-label">${escapeHtml(tool)}</span>`;
      html += `<div class="dash-bar"><div class="dash-bar-fill" style="width:${pct}%;background:${color}"></div></div>`;
      html += `<span class="dash-bar-count">${formatNum(count)}</span>`;
      html += '</div>';
    }
    html += '</div>';
  }

  // MCP Calls
  if (totalMcpCalls > 0) {
    html += '<div class="dash-section">';
    html += '<div class="dash-section-title">MCP Calls</div>';
    const mcpEntries = Object.entries(recording.mcpCalls).sort((a, b) => b[1] - a[1]);
    const maxMcpCount = mcpEntries[0] ? mcpEntries[0][1] : 1;
    for (const [server, count] of mcpEntries) {
      const pct = (count / maxMcpCount * 100).toFixed(0);
      const color = NODE_COLORS.mcp_server;
      html += `<div class="dash-bar-row">`;
      html += `<span class="dash-bar-label">${escapeHtml(server)}</span>`;
      html += `<div class="dash-bar"><div class="dash-bar-fill" style="width:${pct}%;background:${color}"></div></div>`;
      html += `<span class="dash-bar-count">${formatNum(count)}</span>`;
      html += '</div>';
    }
    html += '</div>';
  }

  document.getElementById('record-dash-content').innerHTML = html;
  document.getElementById('record-dashboard').style.display = 'block';
}

const btnRecord = document.getElementById('btn-record');
btnRecord.addEventListener('click', () => {
  if (!recording.active) {
    // Start recording
    resetRecording();
    recording.active = true;
    recording.startTime = Date.now();
    btnRecord.textContent = 'Stop';
    btnRecord.classList.add('recording');
    document.getElementById('record-dashboard').style.display = 'none';
  } else {
    // Stop recording
    recording.active = false;
    btnRecord.textContent = 'Record';
    btnRecord.classList.remove('recording');
    showRecordDashboard();
  }
});

document.getElementById('record-dash-close').addEventListener('click', () => {
  document.getElementById('record-dashboard').style.display = 'none';
});

// ─── Load Session (replay from UI) ─────────────────────────
const fileInput = document.getElementById('file-input');
const btnLoad = document.getElementById('btn-load');
const speedPicker = document.getElementById('speed-picker');
let pendingFile = null;
let selectedSpeed = 1;

btnLoad.addEventListener('click', () => {
  fileInput.click();
});

fileInput.addEventListener('change', () => {
  if (!fileInput.files || fileInput.files.length === 0) return;
  pendingFile = fileInput.files[0];
  document.getElementById('speed-filename').textContent = pendingFile.name;
  speedPicker.style.display = 'block';
  fileInput.value = ''; // reset so same file can be re-selected
});

// Speed button selection
document.getElementById('speed-options').addEventListener('click', (e) => {
  const btn = e.target.closest('.speed-btn');
  if (!btn) return;
  document.querySelectorAll('.speed-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  selectedSpeed = parseFloat(btn.dataset.speed);
});

document.getElementById('speed-go').addEventListener('click', async () => {
  if (!pendingFile) return;
  speedPicker.style.display = 'none';

  // Clear frontend graph state
  state.nodes.clear();
  state.edges.clear();
  state.graphDirty = true;
  syncGraphData();

  btnLoad.textContent = 'Loading...';
  btnLoad.classList.add('loading');

  try {
    const body = await pendingFile.arrayBuffer();
    const res = await fetch(`/api/replay?speed=${selectedSpeed}`, {
      method: 'POST',
      body: body,
    });
    if (!res.ok) {
      const text = await res.text();
      console.error('Replay upload failed:', text);
      btnLoad.textContent = 'Load Session';
      btnLoad.classList.remove('loading');
      return;
    }
    btnLoad.textContent = 'Load Session';
    btnLoad.classList.remove('loading');
  } catch (err) {
    console.error('Replay upload error:', err);
    btnLoad.textContent = 'Load Session';
    btnLoad.classList.remove('loading');
  }

  pendingFile = null;
});

// Close speed picker on Escape
window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && speedPicker.style.display !== 'none') {
    speedPicker.style.display = 'none';
    pendingFile = null;
  }
});

// ─── Bloom Post-Processing ───────────────────────────────────
(async function initBloom() {
  try {
    const { EffectComposer } = await import('three/addons/postprocessing/EffectComposer.js');
    const { RenderPass } = await import('three/addons/postprocessing/RenderPass.js');
    const { UnrealBloomPass } = await import('three/addons/postprocessing/UnrealBloomPass.js');
    const { ShaderPass } = await import('three/addons/postprocessing/ShaderPass.js');

    const renderer = Graph.renderer();
    const scene = Graph.scene();
    const camera = Graph.camera();

    // Half-resolution bloom for performance
    const bloomWidth = Math.floor(renderer.domElement.width / 2);
    const bloomHeight = Math.floor(renderer.domElement.height / 2);

    const bloomComposer = new EffectComposer(renderer);
    bloomComposer.renderToScreen = false;
    bloomComposer.addPass(new RenderPass(scene, camera));

    const bloomPass = new UnrealBloomPass(
      new window.THREE.Vector2(bloomWidth, bloomHeight),
      1.2,  // strength
      0.8,  // radius
      0.6   // threshold
    );
    bloomComposer.addPass(bloomPass);

    // Final composite shader: blend bloom on top of regular render
    const compositeShader = {
      uniforms: {
        baseTexture: { value: null },
        bloomTexture: { value: bloomComposer.renderTarget2.texture },
      },
      vertexShader: `
        varying vec2 vUv;
        void main() {
          vUv = uv;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: `
        uniform sampler2D baseTexture;
        uniform sampler2D bloomTexture;
        varying vec2 vUv;
        void main() {
          vec4 base = texture2D(baseTexture, vUv);
          vec4 bloom = texture2D(bloomTexture, vUv);
          gl_FragColor = base + bloom;
        }
      `,
    };

    const finalComposer = new EffectComposer(renderer);
    const renderPass = new RenderPass(scene, camera);
    finalComposer.addPass(renderPass);

    const compositePass = new ShaderPass(new window.THREE.ShaderMaterial(compositeShader), 'baseTexture');
    compositePass.needsSwap = true;
    finalComposer.addPass(compositePass);

    // Override the default render loop
    const origTick = Graph._animationCycle || null;
    Graph.postProcessingComposer = () => finalComposer;

    // Custom render with selective bloom via layers
    const darkMaterial = new window.THREE.MeshBasicMaterial({ color: 0x000000 });
    const materials = {};

    function darkenNonBloom(obj) {
      if (obj.isMesh || obj.isSprite) {
        if (!obj.layers.test(new window.THREE.Layers().set(1))) {
          // Not on bloom layer — temporarily darken
          materials[obj.uuid] = obj.material;
          obj.material = darkMaterial;
        }
      }
    }

    function restoreMaterials(obj) {
      if (materials[obj.uuid]) {
        obj.material = materials[obj.uuid];
        delete materials[obj.uuid];
      }
    }

    // Hook into the animation frame
    const onBeforeRender = () => {
      // Bloom pass: darken non-bloom objects
      scene.traverse(darkenNonBloom);
      bloomComposer.render();
      scene.traverse(restoreMaterials);

      // Final composite
      finalComposer.render();
    };

    // Replace renderer's render function
    const origRender = renderer.render.bind(renderer);
    let bloomEnabled = true;
    renderer.render = function(s, c) {
      if (bloomEnabled && s === scene) {
        onBeforeRender();
        return;
      }
      origRender(s, c);
    };

    // Handle resize
    window.addEventListener('resize', () => {
      const w = window.innerWidth;
      const h = window.innerHeight;
      bloomComposer.setSize(Math.floor(w / 2), Math.floor(h / 2));
      finalComposer.setSize(w, h);
    });

    console.log('[AgentMind] Bloom post-processing initialized');
  } catch (err) {
    console.warn('[AgentMind] Bloom unavailable (import map or CDN issue):', err.message);
  }
})();

// ─── Always-On Node Labels ──────────────────────────────────

function updateActiveLabels() {
  // Rank nodes by activityCount
  const ranked = Array.from(state.nodes.values())
    .filter(n => !n.stale)
    .sort((a, b) => (b.activityCount || 0) - (a.activityCount || 0))
    .slice(0, LABEL_CONFIG.TOP_N);

  const topIds = new Set(ranked.map(n => n.id));

  // Fade out labels for nodes no longer in top N
  for (const [nodeId, label] of activeLabels) {
    if (!topIds.has(nodeId) && !label.fadeStart) {
      label.fadeStart = performance.now();
    }
  }

  // Add labels for new top-N nodes
  for (const node of ranked) {
    if (activeLabels.has(node.id)) {
      // Re-entered top N — cancel fade
      const existing = activeLabels.get(node.id);
      existing.fadeStart = null;
      continue;
    }

    // Check if SpriteText is available
    if (typeof SpriteText === 'undefined') continue;

    const sprite = new SpriteText(node.label, LABEL_CONFIG.FONT_SIZE, '#e0e0e0');
    sprite.fontFace = 'Menlo, Consolas, monospace';
    sprite.material.opacity = LABEL_CONFIG.OPACITY;
    sprite.material.transparent = true;
    sprite.material.depthWrite = false;
    sprite.position.set(0, 8, 0); // Above node
    sprite.layers.set(0); // Don't bloom labels

    // Find the node's THREE.Group and attach the label
    const graphData = Graph.graphData();
    const gNode = graphData.nodes.find(n => n.id === node.id);
    if (gNode && gNode.__threeObj) {
      gNode.__threeObj.add(sprite);
      activeLabels.set(node.id, { sprite, fadeStart: null, parentObj: gNode.__threeObj });
    }
  }
}

function tickLabels() {
  const now = performance.now();
  for (const [nodeId, label] of activeLabels) {
    if (label.fadeStart) {
      const elapsed = now - label.fadeStart;
      const t = Math.min(1, elapsed / LABEL_CONFIG.FADE_MS);
      label.sprite.material.opacity = LABEL_CONFIG.OPACITY * (1 - t);
      if (t >= 1) {
        // Remove fully faded label
        if (label.parentObj) label.parentObj.remove(label.sprite);
        activeLabels.delete(nodeId);
      }
    }
  }
}

setInterval(updateActiveLabels, LABEL_CONFIG.RECALC_MS);

// ─── Init ───────────────────────────────────────────────────
connectWS();
