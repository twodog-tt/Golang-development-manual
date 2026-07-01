(function () {
  "use strict";

  const STORAGE_KEY = "gdm-mock-interview-v1";
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
    { value: "coding", label: "手写题" },
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

  const root = document.getElementById("mock-interview-root");
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

  function baseUrl() {
    const base = document.querySelector("base");
    if (base && base.href) return base.href;
    return window.location.origin + window.location.pathname.replace(/[^/]*$/, "");
  }

  function answerUrl(path) {
    return new URL(path, baseUrl()).href;
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
      <div class="mock-interview-stats">
        <span>题库 <strong>${catalog.length}</strong> 题</span>
        <span>已标记熟悉 <strong>${familiarTotal}</strong> 题</span>
        <span>本轮已练 <strong>${sessions}</strong> 题</span>
      </div>
    `;
  }

  function renderToolbar() {
    const options = MODULE_FILTERS.map(
      (f) =>
        `<option value="${f.value}" ${f.value === moduleFilter ? "selected" : ""}>${f.label}</option>`
    ).join("");
    return `
      <div class="mock-interview-toolbar">
        <label class="mock-interview-filter">
          <span>模块筛选</span>
          <select id="mock-module-filter">${options}</select>
        </label>
        <button type="button" class="md-button mock-interview-btn-secondary" id="mock-reset-btn">
          重置熟练度
        </button>
      </div>
    `;
  }

  function renderCard(q) {
    if (!q) {
      return `
        <div class="mock-interview-card mock-interview-empty">
          <p>当前筛选下没有可用题目，请切换模块或重置熟练度。</p>
        </div>
      `;
    }
    const familiar = familiarCount(q.id);
    const w = weight(q).toFixed(2);
    const focusBadge = q.resume_focus
      ? '<span class="mock-interview-badge mock-interview-badge-focus">岗位重点</span>'
      : "";
    return `
      <article class="mock-interview-card" aria-live="polite">
        <header class="mock-interview-card-header">
          <span class="mock-interview-id">${q.id}</span>
          <span class="mock-interview-module">${q.module}</span>
          ${focusBadge}
        </header>
        <h2 class="mock-interview-title">${escapeHtml(q.title)}</h2>
        <blockquote class="mock-interview-prompt">${escapeHtml(q.prompt)}</blockquote>
        <p class="mock-interview-meta">
          出现权重 ${w} · 已点「下一题」${familiar} 次（越多越不容易再抽到）
        </p>
        <div class="mock-interview-actions">
          <a class="md-button md-button--primary mock-interview-btn-answer" href="${answerUrl(q.url)}">
            看看答案
          </a>
          <button type="button" class="md-button mock-interview-btn-next" id="mock-next-btn">
            下一题
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
    const nextBtn = document.getElementById("mock-next-btn");
    if (nextBtn) {
      nextBtn.addEventListener("click", () => {
        if (currentId) markFamiliar(currentId);
        state.sessions += 1;
        saveState();
        showQuestion(pickRandom(currentId));
      });
    }
    const filter = document.getElementById("mock-module-filter");
    if (filter) {
      filter.addEventListener("change", (e) => {
        moduleFilter = e.target.value;
        showQuestion(pickRandom(null));
      });
    }
    const resetBtn = document.getElementById("mock-reset-btn");
    if (resetBtn) resetBtn.addEventListener("click", resetProgress);
  }

  function showQuestion(q) {
    currentId = q ? q.id : null;
    const cardHost = document.getElementById("mock-interview-card-host");
    const statsHost = document.getElementById("mock-interview-stats-host");
    if (cardHost) cardHost.innerHTML = renderCard(q);
    if (statsHost) statsHost.innerHTML = renderStats();
    bindEvents();
  }

  function render() {
    root.innerHTML = `
      <div class="mock-interview-app">
        <p class="mock-interview-intro">
          随机抽题模拟面试开场。点 <strong>看看答案</strong> 跳转完整解析；点 <strong>下一题</strong> 表示已熟悉，降低再次出现概率（记录保存在本机浏览器）。
        </p>
        <div id="mock-interview-stats-host">${renderStats()}</div>
        ${renderToolbar()}
        <div id="mock-interview-card-host">${renderCard(null)}</div>
      </div>
    `;
    bindEvents();
  }

  async function init() {
    root.innerHTML = '<p class="mock-interview-loading">正在加载题库…</p>';
    try {
      const res = await fetch(new URL("data/questions.json", baseUrl()));
      if (!res.ok) throw new Error(res.statusText);
      const data = await res.json();
      catalog = data.questions || [];
      render();
      showQuestion(pickRandom(null));
    } catch (err) {
      root.innerHTML = `
        <div class="mock-interview-card mock-interview-empty">
          <p>题库加载失败：${escapeHtml(err.message)}</p>
          <p>请确认已运行 <code>python scripts/generate_mock_interview_data.py</code> 后重新构建站点。</p>
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
