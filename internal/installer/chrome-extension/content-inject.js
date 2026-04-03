// AIFuel Content Injector — Runs in the content script context
// Injects the cost tracker into the page's JS context so it can intercept fetch.
// Communicates back to the extension via CustomEvents.

(() => {
  // Inject the main tracker script into the page context
  const script = document.createElement("script");
  script.src = chrome.runtime.getURL("content.js");
  script.onload = () => script.remove();
  (document.head || document.documentElement).appendChild(script);

  // Listen for messages from the injected script via CustomEvents
  window.addEventListener("aifuel-usage-update", (e) => {
    const data = e.detail;
    if (data) {
      chrome.storage.local.set({
        conversationCost: data
      });
    }
  });
})();
