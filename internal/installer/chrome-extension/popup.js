// AIFuel Popup — Reads cached usage data from extension storage

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

function renderModelLimits(rateLimits) {
  if (!rateLimits || !rateLimits.model_limits) return "";
  const groups = rateLimits.model_limits;
  let html = "";
  for (const [group, limits] of Object.entries(groups)) {
    const name = group.replace(/_/g, " ");
    const conc = limits.concurrents || "-";
    const rpm = limits.raw_thinking_requests_per_minute;
    const rpmStr = rpm ? `${rpm}/min thinking` : "";
    html += `
      <div class="model-row">
        <span class="name">${name}</span>
        <span class="val">${conc} concurrent${rpmStr ? "  |  " + rpmStr : ""}</span>
      </div>`;
  }
  return html;
}

async function loadData() {
  const stored = await chrome.storage.local.get([
    "lastUsage", "lastUpdate", "orgInfo", "rateLimits"
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
  let limitsHTML = "";
  limitsHTML += renderLimit("5-Hour", data.five_hour?.utilization, data.five_hour?.resets_at);
  limitsHTML += renderLimit("7-Day", data.seven_day?.utilization, data.seven_day?.resets_at);
  if (data.seven_day_sonnet?.utilization != null) {
    limitsHTML += renderLimit("Sonnet", data.seven_day_sonnet?.utilization, data.seven_day_sonnet?.resets_at);
  }
  if (data.seven_day_opus?.utilization != null) {
    limitsHTML += renderLimit("Opus", data.seven_day_opus?.utilization, data.seven_day_opus?.resets_at);
  }

  // Extra usage
  if (data.extra_usage?.is_enabled && data.extra_usage?.utilization != null) {
    limitsHTML += renderLimit("Credits", data.extra_usage.utilization, null);
  }

  document.getElementById("limits").innerHTML = limitsHTML;

  // Stats (from native host cached output or extension storage)
  // These come from the aifuel-claude.sh output cached in the native host
  // The popup shows what's available in the extension's storage
  const orgInfo = stored.orgInfo || data._org;
  const plan = orgInfo?.rate_limit_tier || "-";
  document.getElementById("plan").textContent = plan.replace("default_claude_", "").replace(/_/g, " ");

  // Cost/burn/msgs from the cached aifuel output (if available via native host response)
  // These aren't in the raw API response, so show what we have
  document.getElementById("cost").textContent = "$" + (data.extra_usage?.used_credits || 0).toFixed(2);
  document.getElementById("msgs").textContent = "-";
  document.getElementById("burn").textContent = "-";

  document.getElementById("source").textContent = "live";

  // Model limits
  const rateLimits = stored.rateLimits || data._rate_limits;
  if (rateLimits && rateLimits.model_limits && Object.keys(rateLimits.model_limits).length > 0) {
    document.getElementById("model-limits-section").style.display = "block";
    document.getElementById("model-limits").innerHTML = renderModelLimits(rateLimits);
  }

  // Updated time
  if (stored.lastUpdate) {
    const ago = Math.floor((Date.now() - stored.lastUpdate) / 1000);
    let agoStr;
    if (ago < 60) agoStr = "just now";
    else if (ago < 3600) agoStr = Math.floor(ago / 60) + "m ago";
    else agoStr = Math.floor(ago / 3600) + "h ago";
    document.getElementById("updated").textContent = agoStr;
  }
}

// Update badge from stored data
async function updateBadge() {
  const stored = await chrome.storage.local.get(["lastUsage"]);
  const data = stored.lastUsage;
  if (!data || !data.five_hour) {
    chrome.action.setBadgeText({ text: "" });
    return;
  }

  const pct = Math.round(data.five_hour.utilization || 0);
  chrome.action.setBadgeText({ text: pct + "%" });

  const cls = pctClass(pct);
  const colors = {
    ok: "#a6e3a1",
    warn: "#f9e2af",
    crit: "#f38ba8"
  };
  chrome.action.setBadgeBackgroundColor({ color: colors[cls] });
  chrome.action.setBadgeTextColor({ color: "#1e1e2e" });
}

loadData();
updateBadge();
