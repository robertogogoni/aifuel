// AIFuel Live Feed — Chrome Extension Background Service Worker
// Fetches Claude usage, updates badge, sends to native host, and triggers notifications.

const DEFAULT_POLL_MIN = 2;
const NATIVE_HOST = "com.aifuel.live_feed";
const USAGE_API = "https://claude.ai/api/organizations";

// Load user settings (poll interval, thresholds, badge toggle)
async function getSettings() {
  const stored = await chrome.storage.local.get(["settings"]);
  return {
    pollInterval: 2,
    showBadge: true,
    notifications: true,
    warnThreshold: 80,
    critThreshold: 95,
    notifyCooldown: 15,
    ...(stored.settings || {})
  };
}

// Get the org ID from cookies
async function getOrgId() {
  const cookies = await chrome.cookies.getAll({ domain: "claude.ai", name: "lastActiveOrg" });
  return cookies.length > 0 ? cookies[0].value : null;
}

// Fetch usage data using the browser's authenticated session
async function fetchUsage() {
  try {
    const orgId = await getOrgId();
    if (!orgId) {
      console.log("[aifuel] No lastActiveOrg cookie found");
      return null;
    }

    const response = await fetch(`${USAGE_API}/${orgId}/usage`, {
      credentials: "include",
      headers: {
        "Accept": "application/json",
        "Referer": "https://claude.ai/chats"
      }
    });

    if (!response.ok) {
      console.log(`[aifuel] API returned ${response.status}`);
      return null;
    }

    const data = await response.json();
    console.log(`[aifuel] Got usage: 5h=${data.five_hour?.utilization}% 7d=${data.seven_day?.utilization}%`);
    return data;
  } catch (err) {
    console.error("[aifuel] Fetch error:", err.message);
    return null;
  }
}

// Fetch org/account data (rate tier, billing, models) — cached in extension storage
// Only refreshed every 30 minutes since it rarely changes
const ORG_CACHE_TTL_MS = 30 * 60 * 1000;

async function fetchOrgInfo() {
  try {
    // Check cached version first
    const cached = await chrome.storage.local.get(["orgInfo", "orgInfoUpdated"]);
    if (cached.orgInfo && cached.orgInfoUpdated && (Date.now() - cached.orgInfoUpdated) < ORG_CACHE_TTL_MS) {
      return cached.orgInfo;
    }

    const orgId = await getOrgId();
    if (!orgId) return cached.orgInfo || null;

    const response = await fetch(`${USAGE_API}/${orgId}`, {
      credentials: "include",
      headers: {
        "Accept": "application/json",
        "Referer": "https://claude.ai/chats"
      }
    });

    if (!response.ok) {
      console.log(`[aifuel] Org API returned ${response.status}`);
      return cached.orgInfo || null;
    }

    const raw = await response.json();
    const orgInfo = {
      rate_limit_tier: raw.rate_limit_tier,
      billing_type: raw.billing_type,
      capabilities: raw.capabilities,
      created_at: raw.created_at,
      active_flags: raw.active_flags || [],
      models: (raw.claude_ai_bootstrap_models_config || [])
        .filter(m => !m.inactive && !m.overflow)
        .map(m => ({ model: m.model, name: m.name, description: m.description || "" }))
    };

    await chrome.storage.local.set({ orgInfo, orgInfoUpdated: Date.now() });
    console.log(`[aifuel] Got org: tier=${orgInfo.rate_limit_tier} billing=${orgInfo.billing_type}`);
    return orgInfo;
  } catch (err) {
    console.error("[aifuel] Org fetch error:", err.message);
    const cached = await chrome.storage.local.get(["orgInfo"]);
    return cached.orgInfo || null;
  }
}

// Fetch per-model rate limits (concurrency, thinking RPM) — same cache TTL as org
async function fetchRateLimits() {
  try {
    const cached = await chrome.storage.local.get(["rateLimits", "rateLimitsUpdated"]);
    if (cached.rateLimits && cached.rateLimitsUpdated && (Date.now() - cached.rateLimitsUpdated) < ORG_CACHE_TTL_MS) {
      return cached.rateLimits;
    }

    const orgId = await getOrgId();
    if (!orgId) return cached.rateLimits || null;

    const response = await fetch(`${USAGE_API}/${orgId}/rate_limits`, {
      credentials: "include",
      headers: {
        "Accept": "application/json",
        "Referer": "https://claude.ai/chats"
      }
    });

    if (!response.ok) {
      console.log(`[aifuel] Rate limits API returned ${response.status}`);
      return cached.rateLimits || null;
    }

    const raw = await response.json();
    const rateLimits = {
      rate_limit_tier: raw.rate_limit_tier,
      model_limits: (raw.tier_model_rate_limiters || []).reduce((acc, item) => {
        if (!acc[item.model_group]) acc[item.model_group] = {};
        acc[item.model_group][item.limiter] = item.value;
        return acc;
      }, {}),
      spend_threshold: raw.spend_threshold,
      custom_limiters: raw.custom_model_rate_limiters
    };

    await chrome.storage.local.set({ rateLimits, rateLimitsUpdated: Date.now() });
    console.log(`[aifuel] Got rate limits: ${Object.keys(rateLimits.model_limits).length} model groups`);
    return rateLimits;
  } catch (err) {
    console.error("[aifuel] Rate limits fetch error:", err.message);
    const cached = await chrome.storage.local.get(["rateLimits"]);
    return cached.rateLimits || null;
  }
}

// Send data to native messaging host (which writes to file)
function sendToNativeHost(data) {
  try {
    chrome.runtime.sendNativeMessage(NATIVE_HOST, {
      action: "write_usage",
      data: data,
      timestamp: new Date().toISOString()
    }, (response) => {
      if (chrome.runtime.lastError) {
        // Native host not available — write to extension storage as fallback
        chrome.storage.local.set({ lastUsage: data, lastUpdate: Date.now() });
        console.log("[aifuel] Saved to extension storage (native host unavailable)");
      } else {
        console.log("[aifuel] Sent to native host");
      }
    });
  } catch (err) {
    chrome.storage.local.set({ lastUsage: data, lastUpdate: Date.now() });
  }
}

// Update toolbar badge with current usage percentage
async function updateBadge(data) {
  const settings = await getSettings();
  if (!settings.showBadge || !data?.five_hour) {
    chrome.action.setBadgeText({ text: "" });
    return;
  }

  const pct = Math.round(data.five_hour.utilization || 0);
  chrome.action.setBadgeText({ text: pct + "%" });

  let color = "#a6e3a1"; // green
  if (pct >= 85) color = "#f38ba8"; // red
  else if (pct >= 60) color = "#f9e2af"; // yellow

  chrome.action.setBadgeBackgroundColor({ color });
  chrome.action.setBadgeTextColor({ color: "#1e1e2e" });
}

// Send desktop notification when approaching limits
async function checkNotifications(data) {
  const settings = await getSettings();
  if (!settings.notifications || !data?.five_hour) return;

  const pct = data.five_hour.utilization || 0;
  const cooldownMs = settings.notifyCooldown * 60 * 1000;

  const stored = await chrome.storage.local.get(["lastNotifyTime", "lastNotifyLevel"]);
  const now = Date.now();
  const lastTime = stored.lastNotifyTime || 0;
  const lastLevel = stored.lastNotifyLevel || "";

  if (now - lastTime < cooldownMs) return;

  let level = "";
  let title = "";
  let message = "";

  if (pct >= settings.critThreshold) {
    level = "critical";
    title = "AIFuel: Critical Usage";
    message = `5-hour limit at ${Math.round(pct)}%. Consider pausing to let it reset.`;
  } else if (pct >= settings.warnThreshold) {
    level = "warning";
    title = "AIFuel: High Usage";
    message = `5-hour limit at ${Math.round(pct)}%. Approaching rate limit.`;
  }

  if (level && level !== lastLevel) {
    chrome.notifications.create("aifuel-" + level, {
      type: "basic",
      title: title,
      message: message,
      iconUrl: "data:image/svg+xml," + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y="80" font-size="80">⛽</text></svg>'),
      priority: level === "critical" ? 2 : 1
    });
    await chrome.storage.local.set({ lastNotifyTime: now, lastNotifyLevel: level });
    console.log(`[aifuel] Sent ${level} notification at ${Math.round(pct)}%`);
  }
}

// Main poll function
async function pollUsage() {
  const data = await fetchUsage();
  if (data) {
    // Merge org info and rate limits into the usage payload
    const [orgInfo, rateLimits] = await Promise.all([fetchOrgInfo(), fetchRateLimits()]);
    if (orgInfo) data._org = orgInfo;
    if (rateLimits) data._rate_limits = rateLimits;

    sendToNativeHost(data);
    updateBadge(data);
    checkNotifications(data);
  }
}

// Set up periodic polling (reads user-configured interval)
async function setupAlarm() {
  const settings = await getSettings();
  chrome.alarms.clear("poll-usage");
  chrome.alarms.create("poll-usage", { periodInMinutes: settings.pollInterval });
  console.log(`[aifuel] Polling every ${settings.pollInterval} min`);
}

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "poll-usage") {
    pollUsage();
  }
});

// Also poll on extension install/startup
chrome.runtime.onInstalled.addListener(() => {
  console.log("[aifuel] Extension installed, starting live feed");
  setupAlarm();
  pollUsage();
});

chrome.runtime.onStartup.addListener(() => {
  setupAlarm();
  pollUsage();
});

// Initial poll
setupAlarm();
pollUsage();
