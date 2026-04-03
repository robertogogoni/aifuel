// AIFuel Options — Persists settings to chrome.storage.local

const DEFAULTS = {
  pollInterval: 2,
  showBadge: true,
  notifications: true,
  warnThreshold: 80,
  critThreshold: 95,
  notifyCooldown: 15
};

async function loadSettings() {
  const stored = await chrome.storage.local.get(["settings"]);
  const settings = { ...DEFAULTS, ...(stored.settings || {}) };

  document.getElementById("pollInterval").value = settings.pollInterval;
  document.getElementById("showBadge").checked = settings.showBadge;
  document.getElementById("notifications").checked = settings.notifications;
  document.getElementById("warnThreshold").value = settings.warnThreshold;
  document.getElementById("critThreshold").value = settings.critThreshold;
  document.getElementById("notifyCooldown").value = settings.notifyCooldown;
}

async function saveSettings() {
  const settings = {
    pollInterval: parseInt(document.getElementById("pollInterval").value),
    showBadge: document.getElementById("showBadge").checked,
    notifications: document.getElementById("notifications").checked,
    warnThreshold: parseInt(document.getElementById("warnThreshold").value),
    critThreshold: parseInt(document.getElementById("critThreshold").value),
    notifyCooldown: parseInt(document.getElementById("notifyCooldown").value)
  };

  await chrome.storage.local.set({ settings });

  // Update alarm interval if changed
  chrome.alarms.clear("poll-usage");
  chrome.alarms.create("poll-usage", { periodInMinutes: settings.pollInterval });

  // Update badge visibility
  if (!settings.showBadge) {
    chrome.action.setBadgeText({ text: "" });
  }

  // Show saved indicator
  const el = document.getElementById("saved");
  el.classList.add("show");
  setTimeout(() => el.classList.remove("show"), 2000);
}

// Auto-save on any change
document.querySelectorAll("select, input").forEach(el => {
  el.addEventListener("change", saveSettings);
});

loadSettings();
