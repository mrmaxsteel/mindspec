import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

// ─── Color Palette ──────────────────────────────────────────
const NODE_COLORS = {
  agent:       0x4fc3f7,
  tool:        0x81c784,
  mcp_server:  0xce93d8,
  data_source: 0xffb74d,
  llm_endpoint:0xffd54f,
};

const EDGE_COLORS = {
  tool_call:   0x81c784,
  mcp_call:    0xce93d8,
  retrieval:   0x4fc3f7,
  write:       0xffb74d,
  model_call:  0xffd54f,
};

// ─── State ──────────────────────────────────────────────────
const state = {
  nodes: new Map(),      // id → { data, mesh, vel }
  edges: new Map(),      // key → { data, line }
  paused: false,
  autoOrbit: true,
  pinned: null,          // pinned detail card target
  filterText: '',
  showRaw: false,
  ws: null,
  eventBuffer: [],       // buffered while paused
  stats: {},
};

// ─── Three.js Setup ─────────────────────────────────────────
const canvas = document.getElementById('canvas');
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x0a0a1a);

const camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 2000);
camera.position.set(0, 0, 150);

const renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.dampingFactor = 0.05;
controls.addEventListener('start', () => { state.autoOrbit = false; });

// ─── Starfield ──────────────────────────────────────────────
function createStarfield() {
  const geo = new THREE.BufferGeometry();
  const count = 2000;
  const positions = new Float32Array(count * 3);
  for (let i = 0; i < count * 3; i++) {
    positions[i] = (Math.random() - 0.5) * 1000;
  }
  geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  const mat = new THREE.PointsMaterial({ color: 0x444466, size: 0.5, sizeAttenuation: true });
  return new THREE.Points(geo, mat);
}
scene.add(createStarfield());

// Ambient light
scene.add(new THREE.AmbientLight(0xffffff, 0.6));
const pointLight = new THREE.PointLight(0x7aa2f7, 1, 500);
pointLight.position.set(0, 50, 50);
scene.add(pointLight);

// ─── Raycasting ─────────────────────────────────────────────
const raycaster = new THREE.Raycaster();
const mouse = new THREE.Vector2();
let hoveredObj = null;

canvas.addEventListener('mousemove', (e) => {
  mouse.x = (e.clientX / window.innerWidth) * 2 - 1;
  mouse.y = -(e.clientY / window.innerHeight) * 2 + 1;
});

canvas.addEventListener('click', () => {
  if (hoveredObj) {
    state.pinned = hoveredObj;
    showDetail(hoveredObj);
  } else {
    state.pinned = null;
    hideDetail();
  }
});

window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    state.pinned = null;
    hideDetail();
  }
});

// ─── Node/Edge Management ───────────────────────────────────
function getNodeSize(activityCount) {
  return Math.max(1.5, Math.min(6, 1.5 + Math.log2(activityCount + 1) * 0.8));
}

function addOrUpdateNode(data) {
  const existing = state.nodes.get(data.id);
  if (existing) {
    existing.data = data;
    const size = getNodeSize(data.activityCount);
    existing.mesh.scale.setScalar(size / 2);
    const color = NODE_COLORS[data.type] || 0xcccccc;
    existing.mesh.material.color.setHex(color);
    existing.mesh.material.emissive.setHex(color);
    existing.mesh.material.opacity = data.stale ? 0.3 : 1.0;
    return;
  }

  const size = getNodeSize(data.activityCount || 1);
  const geo = new THREE.SphereGeometry(size, 16, 16);
  const color = NODE_COLORS[data.type] || 0xcccccc;
  const mat = new THREE.MeshStandardMaterial({
    color: color,
    emissive: color,
    emissiveIntensity: 0.4,
    transparent: true,
    opacity: 1.0,
  });
  const mesh = new THREE.Mesh(geo, mat);
  mesh.userData = { type: 'node', id: data.id };

  // Random initial position
  mesh.position.set(
    (Math.random() - 0.5) * 80,
    (Math.random() - 0.5) * 80,
    (Math.random() - 0.5) * 80
  );

  scene.add(mesh);
  state.nodes.set(data.id, {
    data,
    mesh,
    vel: new THREE.Vector3(),
  });
}

function addOrUpdateEdge(data) {
  const key = data.src + '|' + data.dst + '|' + data.type;
  const existing = state.edges.get(key);

  if (existing) {
    existing.data = data;
    return;
  }

  const srcNode = state.nodes.get(data.src);
  const dstNode = state.nodes.get(data.dst);
  if (!srcNode || !dstNode) return;

  const color = EDGE_COLORS[data.type] || 0x666666;
  const geo = new THREE.BufferGeometry().setFromPoints([
    srcNode.mesh.position.clone(),
    dstNode.mesh.position.clone(),
  ]);
  const mat = new THREE.LineBasicMaterial({
    color: color,
    transparent: true,
    opacity: data.faded ? 0.15 : 0.6,
    linewidth: 1,
  });
  const line = new THREE.Line(geo, mat);
  line.userData = { type: 'edge', key };
  scene.add(line);
  state.edges.set(key, { data, line });
}

function updateEdgePositions() {
  for (const [key, edge] of state.edges) {
    const srcNode = state.nodes.get(edge.data.src);
    const dstNode = state.nodes.get(edge.data.dst);
    if (!srcNode || !dstNode) continue;

    const positions = edge.line.geometry.attributes.position;
    positions.setXYZ(0, srcNode.mesh.position.x, srcNode.mesh.position.y, srcNode.mesh.position.z);
    positions.setXYZ(1, dstNode.mesh.position.x, dstNode.mesh.position.y, dstNode.mesh.position.z);
    positions.needsUpdate = true;

    // Update opacity based on fade
    edge.line.material.opacity = edge.data.faded ? 0.15 : 0.6;
  }
}

// ─── Force-Directed Layout ──────────────────────────────────
function applyForces() {
  const nodes = Array.from(state.nodes.values());
  const repulsionStrength = 200;
  const attractionStrength = 0.01;
  const damping = 0.9;
  const centerPull = 0.002;

  // Repulsion between all nodes
  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      const a = nodes[i], b = nodes[j];
      const diff = new THREE.Vector3().subVectors(a.mesh.position, b.mesh.position);
      const dist = Math.max(diff.length(), 1);
      const force = repulsionStrength / (dist * dist);
      diff.normalize().multiplyScalar(force);
      a.vel.add(diff);
      b.vel.sub(diff);
    }
  }

  // Attraction along edges
  for (const edge of state.edges.values()) {
    const src = state.nodes.get(edge.data.src);
    const dst = state.nodes.get(edge.data.dst);
    if (!src || !dst) continue;

    const diff = new THREE.Vector3().subVectors(dst.mesh.position, src.mesh.position);
    const dist = diff.length();
    const weight = edge.data.faded ? 0.2 : 1.0;
    const force = dist * attractionStrength * weight;
    diff.normalize().multiplyScalar(force);
    src.vel.add(diff);
    dst.vel.sub(diff);
  }

  // Center pull + apply velocity
  for (const node of nodes) {
    node.vel.add(node.mesh.position.clone().negate().multiplyScalar(centerPull));
    node.vel.multiplyScalar(damping);
    node.mesh.position.add(node.vel);
  }
}

// ─── Filter ─────────────────────────────────────────────────
function applyFilter() {
  const filter = state.filterText.toLowerCase();
  for (const node of state.nodes.values()) {
    if (!filter) {
      node.mesh.material.opacity = node.data.stale ? 0.3 : 1.0;
      continue;
    }
    const match = node.data.id.toLowerCase().includes(filter) ||
                  node.data.label.toLowerCase().includes(filter) ||
                  node.data.type.toLowerCase().includes(filter);
    node.mesh.material.opacity = match ? 1.0 : 0.08;
  }
}

// ─── Detail Card ────────────────────────────────────────────
const detailCard = document.getElementById('detail-card');
const detailContent = document.getElementById('detail-content');

function showDetail(obj) {
  detailCard.style.display = 'block';
  if (obj.type === 'node') {
    const node = state.nodes.get(obj.id);
    if (!node) return;
    const d = node.data;
    let html = `<div class="detail-type">${d.type.toUpperCase()}: ${escapeHtml(d.label)}</div>`;
    html += row('ID', d.id);
    html += row('Type', d.type);
    html += row('Activity', d.activityCount);
    html += row('Last Seen', d.lastSeen || '—');
    html += row('Stale', d.stale ? 'Yes' : 'No');
    if (state.showRaw && d.attributes) {
      html += '<div style="margin-top:8px;color:#888">Attributes:</div>';
      for (const [k, v] of Object.entries(d.attributes)) {
        html += row(k, JSON.stringify(v));
      }
    }
    detailContent.innerHTML = html;
  } else if (obj.type === 'edge') {
    const edge = state.edges.get(obj.key);
    if (!edge) return;
    const d = edge.data;
    let html = `<div class="detail-type">EDGE: ${d.type}</div>`;
    html += row('Source', d.src);
    html += row('Destination', d.dst);
    html += row('Status', d.status || '—');
    html += row('Duration', d.duration ? formatDuration(d.duration) : '—');
    html += row('Calls', d.callCount);
    html += row('Faded', d.faded ? 'Yes' : 'No');
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

function formatDuration(ns) {
  // Duration comes as nanoseconds from Go
  const ms = ns / 1e6;
  if (ms < 1000) return ms.toFixed(1) + 'ms';
  return (ms / 1000).toFixed(2) + 's';
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
    statusEl.style.color = '#81c784';
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
}

function handleUpdate(data) {
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
}

// ─── Controls ───────────────────────────────────────────────
document.getElementById('btn-pause').addEventListener('click', function() {
  state.paused = !state.paused;
  this.textContent = state.paused ? 'Resume' : 'Pause';
  if (!state.paused) {
    // Flush buffer
    for (const msg of state.eventBuffer) {
      handleMessage(msg);
    }
    state.eventBuffer = [];
  }
});

document.getElementById('btn-reset').addEventListener('click', () => {
  camera.position.set(0, 0, 150);
  camera.lookAt(0, 0, 0);
  controls.reset();
  state.autoOrbit = true;
});

document.getElementById('search').addEventListener('input', (e) => {
  state.filterText = e.target.value;
  applyFilter();
});

document.getElementById('chk-raw').addEventListener('change', (e) => {
  state.showRaw = e.target.checked;
  if (state.pinned) showDetail(state.pinned);
});

document.getElementById('detail-close').addEventListener('click', () => {
  state.pinned = null;
  hideDetail();
});

// ─── Resize ─────────────────────────────────────────────────
window.addEventListener('resize', () => {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
});

// ─── Render Loop ────────────────────────────────────────────
let orbitAngle = 0;

function animate() {
  requestAnimationFrame(animate);

  // Force layout
  if (state.nodes.size > 0) {
    applyForces();
  }

  // Update edge positions
  updateEdgePositions();

  // Auto-orbit
  if (state.autoOrbit) {
    orbitAngle += 0.002;
    const radius = 150;
    camera.position.x = Math.cos(orbitAngle) * radius;
    camera.position.z = Math.sin(orbitAngle) * radius;
    camera.lookAt(0, 0, 0);
  }

  controls.update();

  // Raycasting for hover
  raycaster.setFromCamera(mouse, camera);
  const meshes = Array.from(state.nodes.values()).map(n => n.mesh);
  const intersects = raycaster.intersectObjects(meshes);

  if (intersects.length > 0) {
    const obj = intersects[0].object;
    hoveredObj = obj.userData;
    canvas.style.cursor = 'pointer';
    if (!state.pinned) {
      showDetail(hoveredObj);
    }
  } else {
    hoveredObj = null;
    canvas.style.cursor = 'default';
    if (!state.pinned) {
      hideDetail();
    }
  }

  // HUD
  updateHUD();

  renderer.render(scene, camera);
}

// ─── Init ───────────────────────────────────────────────────
connectWS();
animate();
