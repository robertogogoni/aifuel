// AIFuel Live Feed — Chrome Extension Background Service Worker
// Fetches Claude usage every 2 minutes and sends to native host for file output.

const POLL_INTERVAL_MIN = 2;
const NATIVE_HOST = "com.aifuel.live_feed";
const USAGE_API = "https://claude.ai/api/organizations";

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

// Main poll function
async function pollUsage() {
  const data = await fetchUsage();
  if (data) {
    // Merge org info and rate limits into the usage payload
    const [orgInfo, rateLimits] = await Promise.all([fetchOrgInfo(), fetchRateLimits()]);
    if (orgInfo) data._org = orgInfo;
    if (rateLimits) data._rate_limits = rateLimits;
    sendToNativeHost(data);
  }
}

// Set up periodic polling via chrome.alarms
chrome.alarms.create("poll-usage", { periodInMinutes: POLL_INTERVAL_MIN });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "poll-usage") {
    pollUsage();
  }
});

// Also poll on extension install/startup
chrome.runtime.onInstalled.addListener(() => {
  console.log("[aifuel] Extension installed, starting live feed");
  pollUsage();
});

chrome.runtime.onStartup.addListener(() => {
  pollUsage();
});

// Initial poll
pollUsage();
