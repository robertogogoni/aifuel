// AIFuel Content Script — Injected into claude.ai pages
// Intercepts streaming responses to track per-message token costs
// and displays a floating usage widget + per-message cost badges.

(() => {
  "use strict";

  // ── Token pricing (USD per token, as of 2026-04) ─────────────────────
  const PRICING = {
    "claude-opus-4-6":         { input: 15 / 1e6, output: 75 / 1e6, cache_read: 1.5 / 1e6, cache_create: 18.75 / 1e6 },
    "claude-sonnet-4-6":       { input: 3 / 1e6,  output: 15 / 1e6, cache_read: 0.3 / 1e6, cache_create: 3.75 / 1e6 },
    "claude-haiku-4-5":        { input: 0.8 / 1e6, output: 4 / 1e6, cache_read: 0.08 / 1e6, cache_create: 1 / 1e6 },
    "claude-sonnet-4-5":       { input: 3 / 1e6,  output: 15 / 1e6, cache_read: 0.3 / 1e6, cache_create: 3.75 / 1e6 },
    "claude-opus-4-5":         { input: 15 / 1e6, output: 75 / 1e6, cache_read: 1.5 / 1e6, cache_create: 18.75 / 1e6 },
  };

  // Fallback pricing for unknown models
  const DEFAULT_PRICING = { input: 3 / 1e6, output: 15 / 1e6, cache_read: 0.3 / 1e6, cache_create: 3.75 / 1e6 };

  function getPricing(model) {
    if (!model) return DEFAULT_PRICING;
    for (const [key, pricing] of Object.entries(PRICING)) {
      if (model.includes(key) || model.startsWith(key)) return pricing;
    }
    // Match by family
    if (model.includes("opus")) return PRICING["claude-opus-4-6"];
    if (model.includes("haiku")) return PRICING["claude-haiku-4-5"];
    return DEFAULT_PRICING; // sonnet default
  }

  function calcCost(usage, model) {
    const p = getPricing(model);
    return (
      (usage.input_tokens || 0) * p.input +
      (usage.output_tokens || 0) * p.output +
      (usage.cache_read_input_tokens || 0) * p.cache_read +
      (usage.cache_creation_input_tokens || 0) * p.cache_create
    );
  }

  function fmtCost(cost) {
    if (cost >= 1) return "$" + cost.toFixed(2);
    if (cost >= 0.01) return "$" + cost.toFixed(3);
    return "$" + cost.toFixed(4);
  }

  function fmtTokens(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
    return n.toString();
  }

  // ── Conversation state ───────────────────────────────────────────────
  let convTotalCost = 0;
  let convTotalTokens = 0;
  let convMessageCount = 0;
  let currentModel = null;
  let lastUrl = location.href;

  // Reset on navigation (new conversation)
  function checkNavigation() {
    if (location.href !== lastUrl) {
      lastUrl = location.href;
      if (location.pathname.includes("/chat/")) {
        convTotalCost = 0;
        convTotalTokens = 0;
        convMessageCount = 0;
        updateWidget();
      }
    }
  }
  setInterval(checkNavigation, 1000);

  // ── Intercept fetch to capture streaming responses ───────────────────
  const originalFetch = window.fetch;
  window.fetch = async function (...args) {
    const response = await originalFetch.apply(this, args);
    const url = typeof args[0] === "string" ? args[0] : args[0]?.url || "";

    // Intercept conversation completion responses
    if (url.includes("/completion") || url.includes("/chat_conversations") && url.includes("/completion")) {
      try {
        // Clone to avoid consuming the stream
        const cloned = response.clone();
        processStream(cloned);
      } catch (e) {
        // Don't break the page
      }
    }

    return response;
  };

  async function processStream(response) {
    try {
      const reader = response.body?.getReader();
      if (!reader) return;

      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // Process SSE events
        const lines = buffer.split("\n");
        buffer = lines.pop() || ""; // keep incomplete line

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const jsonStr = line.slice(6).trim();
          if (!jsonStr || jsonStr === "[DONE]") continue;

          try {
            const event = JSON.parse(jsonStr);
            handleStreamEvent(event);
          } catch (e) {
            // Not all data lines are JSON
          }
        }
      }
    } catch (e) {
      // Stream reading failed, not critical
    }
  }

  function handleStreamEvent(event) {
    // Detect model from the event
    if (event.model) {
      currentModel = event.model;
    }

    // Look for usage data in the event (sent at end of stream)
    const usage = event.usage || event.message?.usage;
    if (usage && (usage.input_tokens || usage.output_tokens)) {
      const cost = calcCost(usage, currentModel);
      const totalTokens = (usage.input_tokens || 0) + (usage.output_tokens || 0) +
        (usage.cache_read_input_tokens || 0) + (usage.cache_creation_input_tokens || 0);

      convTotalCost += cost;
      convTotalTokens += totalTokens;
      convMessageCount++;

      updateWidget();
      injectMessageBadge(cost, totalTokens, currentModel);

      // Store in extension storage for popup access
      // Dispatch to content-inject.js (which has chrome.storage access)
      window.dispatchEvent(new CustomEvent("aifuel-usage-update", {
        detail: {
          total: convTotalCost,
          tokens: convTotalTokens,
          messages: convMessageCount,
          model: currentModel,
          url: location.href,
          updatedAt: Date.now()
        }
      }));
    }
  }

  // ── Floating widget ──────────────────────────────────────────────────
  let widget = null;

  function createWidget() {
    if (widget) return widget;

    widget = document.createElement("div");
    widget.id = "aifuel-widget";
    widget.innerHTML = `
      <div class="aifuel-w-header">
        <span class="aifuel-w-icon">⛽</span>
        <span class="aifuel-w-title">AIFuel</span>
        <button class="aifuel-w-close" title="Hide">&times;</button>
      </div>
      <div class="aifuel-w-body">
        <div class="aifuel-w-row">
          <span class="aifuel-w-label">Cost</span>
          <span class="aifuel-w-value" id="aifuel-conv-cost">$0.00</span>
        </div>
        <div class="aifuel-w-row">
          <span class="aifuel-w-label">Tokens</span>
          <span class="aifuel-w-value" id="aifuel-conv-tokens">0</span>
        </div>
        <div class="aifuel-w-row">
          <span class="aifuel-w-label">Turns</span>
          <span class="aifuel-w-value" id="aifuel-conv-msgs">0</span>
        </div>
        <div class="aifuel-w-row">
          <span class="aifuel-w-label">Model</span>
          <span class="aifuel-w-value aifuel-w-model" id="aifuel-conv-model">-</span>
        </div>
      </div>
    `;

    const style = document.createElement("style");
    style.textContent = `
      #aifuel-widget {
        position: fixed;
        bottom: 16px;
        right: 16px;
        width: 200px;
        background: #1e1e2e;
        border: 1px solid #313244;
        border-radius: 10px;
        box-shadow: 0 4px 20px rgba(0,0,0,0.4);
        font-family: 'JetBrains Mono', 'Fira Code', monospace;
        font-size: 11px;
        color: #cdd6f4;
        z-index: 99999;
        overflow: hidden;
        transition: opacity 0.2s;
      }
      #aifuel-widget.hidden { display: none; }
      .aifuel-w-header {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 8px 10px;
        background: #181825;
        border-bottom: 1px solid #313244;
      }
      .aifuel-w-icon { font-size: 14px; }
      .aifuel-w-title { font-weight: 700; color: #fab387; flex: 1; }
      .aifuel-w-close {
        background: none;
        border: none;
        color: #6c7086;
        font-size: 16px;
        cursor: pointer;
        padding: 0 2px;
        line-height: 1;
      }
      .aifuel-w-close:hover { color: #f38ba8; }
      .aifuel-w-body { padding: 8px 10px; }
      .aifuel-w-row {
        display: flex;
        justify-content: space-between;
        padding: 3px 0;
      }
      .aifuel-w-label { color: #6c7086; }
      .aifuel-w-value { font-weight: 600; color: #cdd6f4; }
      #aifuel-conv-cost { color: #f9e2af; }
      #aifuel-conv-tokens { color: #89b4fa; }
      .aifuel-w-model { color: #a6adc8; font-size: 10px; }

      .aifuel-msg-badge {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        background: #181825;
        border: 1px solid #313244;
        border-radius: 4px;
        padding: 1px 6px;
        font-family: 'JetBrains Mono', monospace;
        font-size: 10px;
        color: #6c7086;
        margin-left: 8px;
        vertical-align: middle;
      }
      .aifuel-msg-badge .cost { color: #f9e2af; font-weight: 600; }
      .aifuel-msg-badge .tokens { color: #89b4fa; }
    `;

    document.head.appendChild(style);
    document.body.appendChild(widget);

    // Close button
    widget.querySelector(".aifuel-w-close").addEventListener("click", () => {
      widget.classList.add("hidden");
      // Store preference via event
      window.dispatchEvent(new CustomEvent("aifuel-usage-update", { detail: { widgetHidden: true } }));
    });

    // Widget visibility is managed by the close button above
    // (persisted via CustomEvent -> content-inject.js -> chrome.storage)

    return widget;
  }

  function updateWidget() {
    const w = createWidget();
    if (convMessageCount === 0 && !location.pathname.includes("/chat/")) {
      w.classList.add("hidden");
      return;
    }
    w.classList.remove("hidden");

    document.getElementById("aifuel-conv-cost").textContent = fmtCost(convTotalCost);
    document.getElementById("aifuel-conv-tokens").textContent = fmtTokens(convTotalTokens);
    document.getElementById("aifuel-conv-msgs").textContent = convMessageCount.toString();
    document.getElementById("aifuel-conv-model").textContent = currentModel
      ? currentModel.replace("claude-", "").replace(/-\d{8}$/, "")
      : "-";
  }

  // ── Per-message cost badges ──────────────────────────────────────────
  let badgeCount = 0;

  function injectMessageBadge(cost, tokens, model) {
    // Find the latest assistant message container
    // claude.ai uses various selectors; we look for the most recent response
    requestAnimationFrame(() => {
      setTimeout(() => {
        const messages = document.querySelectorAll('[data-testid="chat-message-content"], .font-claude-message, [class*="response"], [class*="assistant"]');
        if (messages.length === 0) return;

        const lastMsg = messages[messages.length - 1];
        // Don't double-badge
        if (lastMsg.querySelector(".aifuel-msg-badge")) return;

        const badge = document.createElement("span");
        badge.className = "aifuel-msg-badge";
        badge.title = `${fmtTokens(tokens)} tokens | ${model || "unknown"}`;
        badge.innerHTML = `<span class="cost">${fmtCost(cost)}</span><span class="tokens">${fmtTokens(tokens)}</span>`;

        // Try to append after the message content
        const parent = lastMsg.closest('[class*="group"]') || lastMsg.parentElement;
        if (parent) {
          parent.appendChild(badge);
          badgeCount++;
        }
      }, 500); // Wait for DOM to settle after streaming ends
    });
  }

  // ── Init ─────────────────────────────────────────────────────────────
  if (location.pathname.includes("/chat/")) {
    createWidget();
  }

  console.log("[aifuel] Content script loaded on", location.href);
})();
