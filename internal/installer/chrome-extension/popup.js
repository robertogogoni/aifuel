// AIFuel Popup — Usage dashboard, conversation search, token estimator

// ── Tab switching ──────────────────────────────────────────────────────
document.querySelectorAll(".tab").forEach(tab => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
    document.querySelectorAll(".tab-content").forEach(c => c.classList.remove("active"));
    tab.classList.add("active");
    document.getElementById("tab-" + tab.dataset.tab).classList.add("active");

    // Load conversations on first switch
    if (tab.dataset.tab === "conv" && !conversationsLoaded) loadConversations();
  });
});

// ── Helpers ────────────────────────────────────────────────────────────
function pctClass(pct) {
  if (pct >= 85) return "crit";
  if (pct >= 60) return "warn";
  return "ok";
}

function formatCountdown(isoStr) {
  if (!isoStr) return "";
  const diff = Math.floor((new Date(isoStr) - Date.now()) / 1000);
  if (diff <= 0) return "expired";
  const d = Math.floor(diff / 86400);
  const h = Math.floor((diff % 86400) / 3600);
  const m = Math.floor((diff % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return "< 1m";
}

function fmtTokens(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
  return Math.round(n).toString();
}

function fmtCost(cost) {
  if (cost >= 1) return "$" + cost.toFixed(2);
  if (cost >= 0.01) return "$" + cost.toFixed(3);
  return "$" + cost.toFixed(4);
}

function renderLimit(label, pct, resetAt) {
  if (pct === null || pct === undefined) return "";
  const cls = pctClass(pct);
  const reset = resetAt ? formatCountdown(resetAt) : "";
  return `
    <div class="limit-row">
      <span class="limit-label">${label}</span>
      <div class="bar-container">
        <div class="bar-fill ${cls}" style="width: ${Math.min(pct, 100)}%"></div>
      </div>
      <span class="limit-pct ${cls}">${Math.round(pct)}%</span>
      <span class="limit-reset">${reset}</span>
    </div>`;
}

// ── Usage Tab ──────────────────────────────────────────────────────────
async function loadUsageTab() {
  const stored = await chrome.storage.local.get([
    "lastUsage", "lastUpdate", "orgInfo", "rateLimits",
    "usageHistory", "conversationCost"
  ]);

  const manifest = chrome.runtime.getManifest();
  document.getElementById("version").textContent = "v" + manifest.version;

  const data = stored.lastUsage;
  if (!data || !data.five_hour) {
    document.getElementById("no-data").style.display = "block";
    document.getElementById("data").style.display = "none";
    return;
  }

  document.getElementById("no-data").style.display = "none";
  document.getElementById("data").style.display = "block";

  // Rate limits
  let html = "";
  html += renderLimit("5-Hour", data.five_hour?.utilization, data.five_hour?.resets_at);
  html += renderLimit("7-Day", data.seven_day?.utilization, data.seven_day?.resets_at);
  if (data.seven_day_sonnet?.utilization != null)
    html += renderLimit("Sonnet", data.seven_day_sonnet.utilization, data.seven_day_sonnet.resets_at);
  if (data.seven_day_opus?.utilization != null)
    html += renderLimit("Opus", data.seven_day_opus.utilization, data.seven_day_opus.resets_at);
  if (data.extra_usage?.is_enabled && data.extra_usage?.utilization != null)
    html += renderLimit("Credits", data.extra_usage.utilization, null);
  document.getElementById("limits").innerHTML = html;

  // Stats: show conversation cost data if available
  const conv = stored.conversationCost;
  if (conv && conv.total > 0) {
    document.getElementById("cost").textContent = fmtCost(conv.total);
    document.getElementById("msgs").textContent = conv.messages || "-";
    document.getElementById("burn").textContent = "-";
  }

  document.getElementById("source").textContent = "live";

  // Updated time
  if (stored.lastUpdate) {
    const ago = Math.floor((Date.now() - stored.lastUpdate) / 1000);
    let agoStr = ago < 60 ? "now" : ago < 3600 ? Math.floor(ago / 60) + "m" : Math.floor(ago / 3600) + "h";
    document.getElementById("updated").textContent = agoStr + " ago";
  }

  // Sparkline
  const history = stored.usageHistory;
  if (history && history.length > 2) {
    document.getElementById("sparkline-section").style.display = "block";
    drawSparkline(history);
  }

  // Current conversation cost
  if (conv && conv.total > 0 && (Date.now() - conv.updatedAt) < 3600000) {
    document.getElementById("conv-cost-section").style.display = "block";
    document.getElementById("conv-cost").textContent = fmtCost(conv.total);
    document.getElementById("conv-tokens").textContent = fmtTokens(conv.tokens);
    document.getElementById("conv-turns").textContent = conv.messages.toString();
    document.getElementById("conv-model").textContent = conv.model
      ? conv.model.replace("claude-", "").replace(/-\d{8}$/, "")
      : "-";
  }
}

// ── Sparkline chart ────────────────────────────────────────────────────
function drawSparkline(history) {
  const canvas = document.getElementById("sparkline");
  const ctx = canvas.getContext("2d");
  const dpr = window.devicePixelRatio || 1;

  canvas.width = canvas.offsetWidth * dpr;
  canvas.height = 40 * dpr;
  ctx.scale(dpr, dpr);

  const w = canvas.offsetWidth;
  const h = 40;
  const points = history.map(p => p.h5);
  const max = Math.max(...points, 100);

  ctx.clearRect(0, 0, w, h);

  // Fill gradient
  const grad = ctx.createLinearGradient(0, 0, 0, h);
  grad.addColorStop(0, "rgba(166, 227, 161, 0.3)");
  grad.addColorStop(1, "rgba(166, 227, 161, 0)");

  ctx.beginPath();
  ctx.moveTo(0, h);
  for (let i = 0; i < points.length; i++) {
    const x = (i / (points.length - 1)) * w;
    const y = h - (points[i] / max) * (h - 4);
    if (i === 0) ctx.lineTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.lineTo(w, h);
  ctx.closePath();
  ctx.fillStyle = grad;
  ctx.fill();

  // Line
  ctx.beginPath();
  for (let i = 0; i < points.length; i++) {
    const x = (i / (points.length - 1)) * w;
    const y = h - (points[i] / max) * (h - 4);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.strokeStyle = "#a6e3a1";
  ctx.lineWidth = 1.5;
  ctx.stroke();

  // Threshold lines
  for (const [pct, color] of [[60, "#f9e2af"], [85, "#f38ba8"]]) {
    const y = h - (pct / max) * (h - 4);
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.strokeStyle = color;
    ctx.lineWidth = 0.5;
    ctx.setLineDash([3, 3]);
    ctx.stroke();
    ctx.setLineDash([]);
  }

  // Time range label
  if (history.length > 0) {
    const oldest = new Date(history[0].t);
    const newest = new Date(history[history.length - 1].t);
    const diffH = Math.round((newest - oldest) / 3600000);
    document.getElementById("sparkline-range").textContent = diffH + "h span";
  }
}

// ── Conversations Tab ──────────────────────────────────────────────────
let conversationsLoaded = false;
let allConversations = [];

async function loadConversations() {
  const listEl = document.getElementById("conv-list");
  listEl.innerHTML = '<div class="no-data">Loading...</div>';

  try {
    const cookies = await chrome.cookies.getAll({ domain: "claude.ai", name: "lastActiveOrg" });
    if (cookies.length === 0) {
      listEl.innerHTML = '<div class="no-data">Not logged in</div>';
      return;
    }
    const orgId = cookies[0].value;

    const response = await fetch(`https://claude.ai/api/organizations/${orgId}/chat_conversations`, {
      credentials: "include",
      headers: { "Accept": "application/json", "Referer": "https://claude.ai/chats" }
    });

    if (!response.ok) {
      listEl.innerHTML = '<div class="no-data">Failed to load</div>';
      return;
    }

    allConversations = await response.json();
    conversationsLoaded = true;
    renderConversations(allConversations);
  } catch (e) {
    listEl.innerHTML = `<div class="no-data">Error: ${e.message}</div>`;
  }
}

function renderConversations(convs) {
  const listEl = document.getElementById("conv-list");
  if (convs.length === 0) {
    listEl.innerHTML = '<div class="no-data">No conversations found</div>';
    return;
  }

  listEl.innerHTML = convs.slice(0, 50).map(c => {
    const name = c.name || "Untitled";
    const model = (c.model || "").replace("claude-", "").replace(/-\d{8}$/, "");
    const date = new Date(c.updated_at || c.created_at);
    const ago = formatTimeAgo(date);
    const starred = c.is_starred ? "&#9733; " : "";
    return `
      <div class="conv-item" data-uuid="${c.uuid}">
        <div class="name">${starred}${escapeHtml(name)}</div>
        <div class="meta">
          <span class="model-tag">${model}</span>
          <span>${ago}</span>
        </div>
      </div>`;
  }).join("");

  // Click to open
  listEl.querySelectorAll(".conv-item").forEach(el => {
    el.addEventListener("click", () => {
      chrome.tabs.create({ url: `https://claude.ai/chat/${el.dataset.uuid}` });
    });
  });
}

function formatTimeAgo(date) {
  const diff = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  return Math.floor(diff / 86400) + "d ago";
}

function escapeHtml(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

// Search filter
document.getElementById("conv-search").addEventListener("input", (e) => {
  const q = e.target.value.toLowerCase();
  if (!q) {
    renderConversations(allConversations);
    return;
  }
  const filtered = allConversations.filter(c =>
    (c.name || "").toLowerCase().includes(q) ||
    (c.model || "").toLowerCase().includes(q)
  );
  renderConversations(filtered);
});

// ── Token Estimator Tab ────────────────────────────────────────────────
document.getElementById("est-input").addEventListener("input", (e) => {
  const text = e.target.value;
  // Rough tokenizer: ~1 token per 4 chars for English, ~1 per 2 for code
  const charCount = text.length;
  const tokens = Math.ceil(charCount / 3.5);

  document.getElementById("est-tokens").textContent = fmtTokens(tokens);
  // Opus: $15/M input, Sonnet: $3/M input
  document.getElementById("est-opus").textContent = fmtCost(tokens * 15 / 1e6);
  document.getElementById("est-sonnet").textContent = fmtCost(tokens * 3 / 1e6);
});

// ── Badge update ───────────────────────────────────────────────────────
async function updateBadge() {
  const stored = await chrome.storage.local.get(["lastUsage", "settings"]);
  const data = stored.lastUsage;
  const settings = stored.settings || {};

  if (settings.showBadge === false || !data?.five_hour) {
    chrome.action.setBadgeText({ text: "" });
    return;
  }

  const pct = Math.round(data.five_hour.utilization || 0);
  chrome.action.setBadgeText({ text: pct + "%" });

  const cls = pctClass(pct);
  const colors = { ok: "#a6e3a1", warn: "#f9e2af", crit: "#f38ba8" };
  chrome.action.setBadgeBackgroundColor({ color: colors[cls] });
  chrome.action.setBadgeTextColor({ color: "#1e1e2e" });
}

// ── Init ───────────────────────────────────────────────────────────────
loadUsageTab();
updateBadge();
