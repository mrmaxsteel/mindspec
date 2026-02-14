// ─── Color Palette ──────────────────────────────────────────
const NODE_COLORS = {
  agent:       '#4fc3f7',
  tool:        '#81c784',
  mcp_server:  '#ce93d8',
  data_source: '#ffb74d',
  llm_endpoint:'#ffd54f',
};

const EDGE_COLORS = {
  tool_call:   '#81c784',
  mcp_call:    '#ce93d8',
  retrieval:   '#4fc3f7',
  write:       '#ffb74d',
  model_call:  '#ffd54f',
};

// ─── State ──────────────────────────────────────────────────
const state = {
  nodes: new Map(),      // id → node data
  edges: new Map(),      // key → edge data
  paused: false,
  pinned: null,
  filterText: '',
  showRaw: false,
  ws: null,
  eventBuffer: [],
  stats: {},
  graphDirty: false,
};

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
  .linkColor(link => EDGE_COLORS[link.type] || '#666666')
  .linkOpacity(link => Math.max(0.05, (link.opacity !== undefined ? link.opacity : 1.0) * 0.8))
  .linkDirectionalParticles(2)
  .linkDirectionalParticleWidth(0.8)
  .linkDirectionalParticleSpeed(0.006)
  .linkWidth(link => Math.max(0.3, (link.callCount || 1) * 0.3))
  .linkLabel(link => `${link.type} (${link.callCount || 1}x)`)
  .onNodeClick(node => {
    state.pinned = { type: 'node', id: node.id };
    showDetail(state.pinned);
  })
  .onBackgroundClick(() => {
    state.pinned = null;
    hideDetail();
  });

// Add starfield via underlying Three.js scene
const THREE = Graph.scene().constructor.__proto__.constructor;
(function createStarfield() {
  const scene = Graph.scene();
  const geo = new window.THREE.BufferGeometry();
  const count = 2000;
  const positions = new Float32Array(count * 3);
  for (let i = 0; i < count * 3; i++) {
    positions[i] = (Math.random() - 0.5) * 2000;
  }
  geo.setAttribute('position', new window.THREE.BufferAttribute(positions, 3));
  const mat = new window.THREE.PointsMaterial({ color: 0x444466, size: 0.5, sizeAttenuation: true });
  scene.add(new window.THREE.Points(geo, mat));
})();

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

// Sync on a timer to batch updates
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
  } else {
    state.edges.set(key, { ...data, id: key });
  }
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
    for (const msg of state.eventBuffer) {
      handleMessage(msg);
    }
    state.eventBuffer = [];
  }
});

document.getElementById('btn-reset').addEventListener('click', () => {
  Graph.cameraPosition({ x: 0, y: 0, z: 300 });
});

document.getElementById('search').addEventListener('input', (e) => {
  state.filterText = e.target.value;
  // Force re-render of node opacities
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
  }
});

// ─── Init ───────────────────────────────────────────────────
connectWS();
