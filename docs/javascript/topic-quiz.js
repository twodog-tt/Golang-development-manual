(function () {
  "use strict";

  const STORAGE_KEY = "gdm-topic-quiz-v1";
  const FAMILIAR_DECAY = 0.55;
  const MIN_WEIGHT = 0.08;

  const MODULE_FILTERS = [
    { value: "", label: "全部模块" },
    { value: "concurrency", label: "并发与运行时" },
    { value: "memory_gc", label: "内存与 GC" },
    { value: "system_design", label: "系统设计" },
    { value: "mysql", label: "MySQL" },
    { value: "redis", label: "Redis" },
    { value: "kafka", label: "Kafka" },
    { value: "rocketmq", label: "RocketMQ" },
    { value: "rabbitmq", label: "RabbitMQ" },
    { value: "elasticsearch", label: "Elasticsearch" },
    { value: "distributed", label: "分布式事务" },
    { value: "network", label: "网络" },
    { value: "coding", label: "编码练习" },
    { value: "cloud_native", label: "云原生" },
    { value: "ai_engineering", label: "AI 工程" },
    { value: "solution_architecture", label: "解决方案架构" },
    { value: "blockchain_web3", label: "区块链 Web3" },
    { value: "solidity_contracts", label: "Solidity" },
    { value: "dex_cex_engineering", label: "DEX / CEX" },
    { value: "leadership", label: "工程与领导力" },
  ];

  let catalog = [];
  let state = loadState();
  let currentId = null;
  let moduleFilter = "";

  const root = document.getElementById("topic-quiz-root");
  if (!root) return;

  function loadState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return { familiar: {}, sessions: 0 };
      const parsed = JSON.parse(raw);
      return {
        familiar: parsed.familiar || {},
        sessions: parsed.sessions || 0,
      };
    } catch {
      return { familiar: {}, sessions: 0 };
    }
  }

  function saveState() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }

  function familiarCount(id) {
    return state.familiar[id] || 0;
  }

  function weight(q) {
    const base = q.frequency || 3;
    const decay = Math.pow(FAMILIAR_DECAY, familiarCount(q.id));
    return Math.max(MIN_WEIGHT, base * decay);
  }

  function filteredQuestions() {
    if (!moduleFilter) return catalog;
    return catalog.filter((q) => q.module_key === moduleFilter);
  }

  function pickRandom(excludeId) {
    const pool = filteredQuestions().filter((q) => q.id !== excludeId);
    if (!pool.length) return null;
    const total = pool.reduce((sum, q) => sum + weight(q), 0);
    let roll = Math.random() * total;
    for (const q of pool) {
      roll -= weight(q);
      if (roll <= 0) return q;
    }
    return pool[pool.length - 1];
  }

  function siteBaseUrl() {
    const configEl = document.getElementById("__config");
    if (configEl) {
      try {
        const config = JSON.parse(configEl.textContent);
        if (config.base) {
          return new URL(config.base + "/", window.location.href).href;
        }
      } catch {
        /* ignore */
      }
    }
    if (typeof __md_scope !== "undefined") {
      return new URL(__md_scope, window.location.href).href;
    }
    const base = document.querySelector("base");
    if (base && base.href) return base.href;
    return window.location.origin + "/";
  }

  function answerUrl(path) {
    return new URL(path, siteBaseUrl()).href;
  }

  function dataUrl() {
    return new URL("data/questions.json", siteBaseUrl()).href;
  }

  function markFamiliar(id) {
    state.familiar[id] = familiarCount(id) + 1;
    saveState();
  }

  function resetProgress() {
    if (!window.confirm("确定清空本机熟练度记录？此操作不可恢复。")) return;
    state = { familiar: {}, sessions: 0 };
    saveState();
    render();
    showQuestion(pickRandom(null));
  }

  function renderStats() {
    const familiarTotal = Object.keys(state.familiar).length;
    const sessions = state.sessions;
    return `
      <div class="topic-quiz-stats">
        <span>专题库 <strong>${catalog.length}</strong> 篇</span>
        <span>已标记熟悉 <strong>${familiarTotal}</strong> 篇</span>
        <span>本轮已练 <strong>${sessions}</strong> 篇</span>
      </div>
    `;
  }

  function renderToolbar() {
    const options = MODULE_FILTERS.map(
      (f) =>
        `<option value="${f.value}" ${f.value === moduleFilter ? "selected" : ""}>${f.label}</option>`
    ).join("");
    return `
      <div class="topic-quiz-toolbar">
        <label class="topic-quiz-filter">
          <span>模块筛选</span>
          <select id="topic-quiz-module-filter">${options}</select>
        </label>
        <button type="button" class="md-button topic-quiz-btn-secondary" id="topic-quiz-reset-btn">
          重置熟练度
        </button>
      </div>
    `;
  }

  function renderCard(q) {
    if (!q) {
      return `
        <div class="topic-quiz-card topic-quiz-empty">
          <p>当前筛选下没有可用专题，请切换模块或重置熟练度。</p>
        </div>
      `;
    }
    const familiar = familiarCount(q.id);
    const w = weight(q).toFixed(2);
    const focusBadge = q.resume_focus
      ? '<span class="topic-quiz-badge topic-quiz-badge-focus">方向重点</span>'
      : "";
    return `
      <article class="topic-quiz-card" aria-live="polite">
        <header class="topic-quiz-card-header">
          <span class="topic-quiz-id">${q.id}</span>
          <span class="topic-quiz-module">${q.module}</span>
          ${focusBadge}
        </header>
        <h2 class="topic-quiz-title">${escapeHtml(q.title)}</h2>
        <blockquote class="topic-quiz-prompt">${escapeHtml(q.prompt)}</blockquote>
        <p class="topic-quiz-meta">
          出现权重 ${w} · 已点「下一篇」${familiar} 次（越多越不容易再抽到）
        </p>
        <div class="topic-quiz-actions">
          <a class="md-button md-button--primary topic-quiz-btn-answer" href="${answerUrl(q.url)}">
            看看答案
          </a>
          <button type="button" class="md-button topic-quiz-btn-next" id="topic-quiz-next-btn">
            下一篇
          </button>
        </div>
      </article>
    `;
  }

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function bindEvents() {
    const nextBtn = document.getElementById("topic-quiz-next-btn");
    if (nextBtn) {
      nextBtn.addEventListener("click", () => {
        if (currentId) markFamiliar(currentId);
        state.sessions += 1;
        saveState();
        showQuestion(pickRandom(currentId));
      });
    }
    const filter = document.getElementById("topic-quiz-module-filter");
    if (filter) {
      filter.addEventListener("change", (e) => {
        moduleFilter = e.target.value;
        showQuestion(pickRandom(null));
      });
    }
    const resetBtn = document.getElementById("topic-quiz-reset-btn");
    if (resetBtn) resetBtn.addEventListener("click", resetProgress);
  }

  function showQuestion(q) {
    currentId = q ? q.id : null;
    const cardHost = document.getElementById("topic-quiz-card-host");
    const statsHost = document.getElementById("topic-quiz-stats-host");
    if (cardHost) cardHost.innerHTML = renderCard(q);
    if (statsHost) statsHost.innerHTML = renderStats();
    bindEvents();
  }

  function render() {
    root.innerHTML = `
      <div class="topic-quiz-app">
        <p class="topic-quiz-intro">
          随机抽专题做专题自测。点 <strong>看看答案</strong> 跳转完整解析；点 <strong>下一篇</strong> 表示已熟悉，降低再次出现概率（记录保存在本机浏览器）。
        </p>
        <div id="topic-quiz-stats-host">${renderStats()}</div>
        ${renderToolbar()}
        <div id="topic-quiz-card-host">${renderCard(null)}</div>
      </div>
    `;
    bindEvents();
  }

  async function init() {
    root.innerHTML = '<p class="topic-quiz-loading">正在加载专题库…</p>';
    try {
      const res = await fetch(dataUrl());
      if (!res.ok) throw new Error(res.statusText);
      const data = await res.json();
      catalog = data.questions || [];
      render();
      showQuestion(pickRandom(null));
    } catch (err) {
      root.innerHTML = `
        <div class="topic-quiz-card topic-quiz-empty">
          <p>专题库加载失败：${escapeHtml(err.message)}</p>
          <p>请确认已运行 <code>python scripts/generate_topic_quiz_data.py</code> 后重新构建站点。</p>
        </div>
      `;
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
