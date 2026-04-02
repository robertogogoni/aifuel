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
