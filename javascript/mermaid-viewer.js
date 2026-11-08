/**
 * Mermaid 图表渐进增强：
 * - Mermaid 可用时将源码渲染为 SVG。
 * - 自动提供全屏、缩放、拖拽、新标签打开与复制源码。
 * - Mermaid 不可用或单图渲染失败时保留原始代码块。
 */
(function () {
  "use strict";

  const MERMAID_SELECTOR = ".md-content pre.mermaid";
  const MIN_SCALE = 0.1;
  const MAX_SCALE = 6;
  let renderId = 0;
  let mermaidInitialized = false;
  let dialogState;

  function createButton(action, text, label = text) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "diagram-button";
    button.dataset.diagramAction = action;
    button.textContent = text;
    button.setAttribute("aria-label", label);
    return button;
  }

  function diagramTitle(block, index) {
    const article = block.closest("article");
    let heading = block.previousElementSibling;
    while (heading && !heading.matches("h2, h3, h4")) {
      heading = heading.previousElementSibling;
    }
    heading ||= article?.querySelector("h1");

    const cleanHeading = heading?.cloneNode(true);
    cleanHeading?.querySelectorAll(".headerlink").forEach((link) => link.remove());
    const prefix = cleanHeading?.textContent.trim() || "知识库图表";
    return `${prefix}（图 ${index}）`;
  }

  function sourceFrom(block) {
    return (block.querySelector("code")?.textContent || block.textContent || "")
      .trim();
  }

  function setStatus(viewer, message, kind = "info") {
    const status = viewer.querySelector(".diagram-status");
    status.textContent = message;
    status.dataset.kind = kind;
    status.hidden = !message;
  }

  function setButtonEnabled(viewer, action, enabled) {
    const button = viewer.querySelector(`[data-diagram-action="${action}"]`);
    if (button) button.disabled = !enabled;
  }

  function buildViewer(block, index) {
    if (block.closest(".diagram-viewer")) return block.closest(".diagram-viewer");

    const parent = block.parentNode;
    const source = sourceFrom(block);
    const title = diagramTitle(block, index);
    block.classList.remove("mermaid");
    block.classList.add("mermaid-source");
    const viewer = document.createElement("figure");
    viewer.className = "diagram-viewer";
    viewer.dataset.mermaidSource = source;
    viewer.dataset.diagramTitle = title;
    viewer.setAttribute("aria-label", title);

    const toolbar = document.createElement("div");
    toolbar.className = "diagram-toolbar";
    toolbar.setAttribute("role", "toolbar");
    toolbar.setAttribute("aria-label", `${title}操作`);

    const hint = document.createElement("span");
    hint.className = "diagram-toolbar__hint";
    hint.textContent = "点击图表可全屏查看";

    const fullscreen = createButton("fullscreen", "全屏", "全屏查看图表");
    fullscreen.setAttribute("aria-keyshortcuts", "Enter");
    const open = createButton("open", "新标签", "在新标签页打开 SVG");
    open.disabled = true;
    const copy = createButton("copy", "复制源码", "复制 Mermaid 源码");
    toolbar.append(hint, fullscreen, open, copy);

    const stage = document.createElement("div");
    stage.className = "diagram-stage";

    const status = document.createElement("figcaption");
    status.className = "diagram-status";
    status.setAttribute("role", "status");
    status.hidden = true;

    parent.insertBefore(viewer, block);
    stage.append(block);
    viewer.append(toolbar, stage, status);

    toolbar.addEventListener("click", (event) => {
      const button = event.target.closest("[data-diagram-action]");
      if (!button || button.disabled) return;
      handleAction(viewer, button);
    });

    stage.addEventListener("click", (event) => {
      if (!viewer.classList.contains("diagram-viewer--rendered")) return;
      if (event.target.closest("a")) return;
      openDialog(viewer);
    });

    stage.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" && event.key !== " ") return;
      if (!viewer.classList.contains("diagram-viewer--rendered")) return;
      event.preventDefault();
      openDialog(viewer);
    });

    return viewer;
  }

  async function renderViewer(viewer) {
    const stage = viewer.querySelector(".diagram-stage");
    const source = viewer.dataset.mermaidSource;

    if (!window.mermaid) {
      viewer.classList.add("diagram-viewer--fallback");
      setStatus(viewer, "图表引擎加载失败，已保留 Mermaid 源码。", "warning");
      return;
    }

    try {
      const id = `knowledge-diagram-${Date.now()}-${renderId++}`;
      const result = await window.mermaid.render(id, source);
      stage.innerHTML = result.svg;
      result.bindFunctions?.(stage);

      const svg = stage.querySelector("svg");
      if (!svg) throw new Error("Mermaid 未返回 SVG");

      svg.removeAttribute("height");
      svg.setAttribute("role", "img");
      svg.setAttribute("aria-label", viewer.dataset.diagramTitle);
      svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
      stage.tabIndex = 0;
      stage.setAttribute("role", "button");
      stage.setAttribute("aria-label", `${viewer.dataset.diagramTitle}，打开全屏查看`);

      const viewBox = svg.viewBox?.baseVal;
      const complex =
        (viewBox && (viewBox.width > 1100 || viewBox.height > 760)) ||
        source.split("\n").length > 28;
      viewer.classList.toggle("diagram-viewer--complex", Boolean(complex));
      viewer.classList.add("diagram-viewer--rendered");
      setButtonEnabled(viewer, "open", true);
      setStatus(viewer, "");
    } catch (error) {
      viewer.classList.add("diagram-viewer--fallback");
      setStatus(
        viewer,
        `图表渲染失败，已保留源码：${error?.message || "未知错误"}`,
        "error",
      );
    }
  }

  function initializeMermaid() {
    if (!window.mermaid || mermaidInitialized) return;
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "base",
      themeVariables: {
        primaryColor: "#e0f2f1",
        primaryTextColor: "#0f172a",
        primaryBorderColor: "#0f766e",
        lineColor: "#475569",
        secondaryColor: "#eef2ff",
        tertiaryColor: "#f8fafc",
        background: "#ffffff",
        mainBkg: "#e0f2f1",
        nodeBorder: "#0f766e",
        clusterBkg: "#f8fafc",
        clusterBorder: "#94a3b8",
        edgeLabelBackground: "#ffffff",
        fontFamily:
          '"Inter", "Noto Sans SC", -apple-system, BlinkMacSystemFont, sans-serif',
      },
      flowchart: {
        htmlLabels: true,
        useMaxWidth: true,
        curve: "basis",
      },
      sequence: {
        useMaxWidth: true,
        wrap: true,
      },
    });
    mermaidInitialized = true;
  }

  async function enhanceDiagrams() {
    const blocks = [...document.querySelectorAll(MERMAID_SELECTOR)].filter(
      (block) => !block.closest(".diagram-viewer"),
    );
    if (!blocks.length) return;

    initializeMermaid();
    const viewers = blocks.map((block, index) => buildViewer(block, index + 1));
    for (const viewer of viewers) {
      await renderViewer(viewer);
    }
  }

  function copySource(viewer, button) {
    const source = viewer.dataset.mermaidSource;
    const fallbackCopy = () => {
      const textarea = document.createElement("textarea");
      textarea.value = source;
      textarea.setAttribute("readonly", "");
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.append(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    };

    const operation = navigator.clipboard?.writeText
      ? navigator.clipboard.writeText(source)
      : Promise.resolve().then(fallbackCopy);

    operation
      .then(() => flashButton(button, "已复制"))
      .catch(() => {
        fallbackCopy();
        flashButton(button, "已复制");
      });
  }

  function flashButton(button, text) {
    const original = button.textContent;
    button.textContent = text;
    window.setTimeout(() => {
      button.textContent = original;
    }, 1400);
  }

  function namespacedSvg(svg) {
    const clone = svg.cloneNode(true);
    const prefix = `diagram-export-${Date.now()}-${renderId++}`;
    const idMap = new Map();

    [clone, ...clone.querySelectorAll("[id]")].forEach((element) => {
      if (!element.id) return;
      const oldId = element.id;
      const newId = `${prefix}-${oldId}`;
      idMap.set(oldId, newId);
      element.id = newId;
    });

    const references = [...idMap].sort(
      ([left], [right]) => right.length - left.length,
    );
    const replaceReferences = (value) => {
      let next = value;
      for (const [oldId, newId] of references) {
        next = next
          .replaceAll(`#${oldId}`, `#${newId}`)
          .replaceAll(`url(#${oldId})`, `url(#${newId})`)
          .replaceAll(`href="#${oldId}"`, `href="#${newId}"`);
      }
      return next;
    };

    [clone, ...clone.querySelectorAll("*")].forEach((element) => {
      for (const attribute of [...element.attributes]) {
        const next = replaceReferences(attribute.value);
        if (next !== attribute.value) element.setAttribute(attribute.name, next);
      }
      if (element.tagName.toLowerCase() === "style") {
        element.textContent = replaceReferences(element.textContent);
      }
    });

    return clone;
  }

  function standaloneSvg(viewer) {
    const svg = viewer.querySelector(".diagram-stage svg");
    if (!svg) return null;
    const clone = namespacedSvg(svg);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("xmlns:xlink", "http://www.w3.org/1999/xlink");
    return clone;
  }

  function openSvg(viewer, button) {
    const svg = standaloneSvg(viewer);
    if (!svg) return;
    const source = `<?xml version="1.0" encoding="UTF-8"?>\n${new XMLSerializer().serializeToString(svg)}`;
    const url = URL.createObjectURL(
      new Blob([source], { type: "image/svg+xml;charset=utf-8" }),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.target = "_blank";
    anchor.rel = "noopener noreferrer";
    document.body.append(anchor);
    anchor.click();
    anchor.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    flashButton(button, "已打开");
  }

  function ensureDialog() {
    if (dialogState) return dialogState;

    const dialog = document.createElement("dialog");
    dialog.className = "diagram-dialog";
    dialog.innerHTML = `
      <div class="diagram-dialog__shell">
        <header class="diagram-dialog__header">
          <h2 class="diagram-dialog__title"></h2>
          <div class="diagram-dialog__controls" role="toolbar" aria-label="图表缩放">
            <button type="button" class="diagram-button" data-modal-action="zoom-out" aria-label="缩小图表">−</button>
            <output class="diagram-dialog__scale" aria-live="polite">100%</output>
            <button type="button" class="diagram-button" data-modal-action="zoom-in" aria-label="放大图表">＋</button>
            <button type="button" class="diagram-button" data-modal-action="reset">重置</button>
            <button type="button" class="diagram-button" data-modal-action="close">关闭</button>
          </div>
        </header>
        <div class="diagram-dialog__viewport">
          <div class="diagram-dialog__canvas"></div>
        </div>
      </div>
    `;
    document.body.append(dialog);

    dialogState = {
      dialog,
      viewport: dialog.querySelector(".diagram-dialog__viewport"),
      canvas: dialog.querySelector(".diagram-dialog__canvas"),
      title: dialog.querySelector(".diagram-dialog__title"),
      scaleOutput: dialog.querySelector(".diagram-dialog__scale"),
      scale: 1,
      x: 0,
      y: 0,
      pointers: new Map(),
      returnFocus: null,
    };

    dialog.addEventListener("click", (event) => {
      const button = event.target.closest("[data-modal-action]");
      if (button) handleModalAction(button.dataset.modalAction);
      if (event.target === dialog) dialog.close();
    });
    dialog.addEventListener("close", () => {
      dialogState.pointers.clear();
      dialogState.returnFocus?.focus();
      dialogState.returnFocus = null;
    });
    dialog.addEventListener("keydown", (event) => {
      if (event.key === "+" || event.key === "=") {
        event.preventDefault();
        zoomBy(1.2);
      } else if (event.key === "-") {
        event.preventDefault();
        zoomBy(1 / 1.2);
      } else if (event.key === "0") {
        event.preventDefault();
        fitDialog();
      }
    });

    const viewport = dialogState.viewport;
    viewport.addEventListener(
      "wheel",
      (event) => {
        event.preventDefault();
        const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
        zoomAt(event.clientX, event.clientY, factor);
      },
      { passive: false },
    );
    viewport.addEventListener("pointerdown", onPointerDown);
    viewport.addEventListener("pointermove", onPointerMove);
    viewport.addEventListener("pointerup", onPointerEnd);
    viewport.addEventListener("pointercancel", onPointerEnd);
    window.addEventListener("resize", () => {
      if (dialog.open) fitDialog();
    });

    return dialogState;
  }

  function openDialog(viewer) {
    const state = ensureDialog();
    state.title.textContent = viewer.dataset.diagramTitle;
    state.canvas.replaceChildren();
    state.returnFocus = document.activeElement;

    const svg = standaloneSvg(viewer);
    if (svg) {
      state.canvas.classList.remove("diagram-dialog__canvas--source");
      state.canvas.append(svg);
      dialogControlsEnabled(true);
    } else {
      const code = document.createElement("pre");
      code.className = "diagram-dialog__source";
      code.textContent = viewer.dataset.mermaidSource;
      state.canvas.classList.add("diagram-dialog__canvas--source");
      state.canvas.append(code);
      dialogControlsEnabled(false);
    }

    state.dialog.showModal();
    window.requestAnimationFrame(fitDialog);
  }

  function dialogControlsEnabled(enabled) {
    const state = ensureDialog();
    state.dialog
      .querySelectorAll(
        '[data-modal-action="zoom-in"], [data-modal-action="zoom-out"], [data-modal-action="reset"]',
      )
      .forEach((button) => {
        button.disabled = !enabled;
      });
  }

  function naturalSvgSize(svg) {
    const viewBox = svg.viewBox?.baseVal;
    const width = viewBox?.width || Number.parseFloat(svg.getAttribute("width")) || 800;
    const height =
      viewBox?.height || Number.parseFloat(svg.getAttribute("height")) || 600;
    svg.style.width = `${width}px`;
    svg.style.height = `${height}px`;
    svg.style.maxWidth = "none";
    return { width, height };
  }

  function fitDialog() {
    const state = ensureDialog();
    const svg = state.canvas.querySelector("svg");
    if (!svg) {
      state.scale = 1;
      state.x = 0;
      state.y = 0;
      applyTransform();
      return;
    }

    const size = naturalSvgSize(svg);
    const bounds = state.viewport.getBoundingClientRect();
    state.scale = Math.min(
      (bounds.width - 48) / size.width,
      (bounds.height - 48) / size.height,
      1,
    );
    state.scale = Math.max(MIN_SCALE, state.scale);
    state.x = (bounds.width - size.width * state.scale) / 2;
    state.y = (bounds.height - size.height * state.scale) / 2;
    applyTransform();
  }

  function applyTransform() {
    const state = ensureDialog();
    state.canvas.style.transform = `translate(${state.x}px, ${state.y}px) scale(${state.scale})`;
    state.scaleOutput.value = `${Math.round(state.scale * 100)}%`;
    state.scaleOutput.textContent = state.scaleOutput.value;
  }

  function zoomAt(clientX, clientY, factor) {
    const state = ensureDialog();
    if (!state.canvas.querySelector("svg")) return;
    const bounds = state.viewport.getBoundingClientRect();
    const localX = clientX - bounds.left;
    const localY = clientY - bounds.top;
    const nextScale = Math.min(
      MAX_SCALE,
      Math.max(MIN_SCALE, state.scale * factor),
    );
    const contentX = (localX - state.x) / state.scale;
    const contentY = (localY - state.y) / state.scale;
    state.x = localX - contentX * nextScale;
    state.y = localY - contentY * nextScale;
    state.scale = nextScale;
    applyTransform();
  }

  function zoomBy(factor) {
    const state = ensureDialog();
    const bounds = state.viewport.getBoundingClientRect();
    zoomAt(
      bounds.left + bounds.width / 2,
      bounds.top + bounds.height / 2,
      factor,
    );
  }

  function onPointerDown(event) {
    const state = ensureDialog();
    if (!state.canvas.querySelector("svg")) return;
    state.viewport.setPointerCapture(event.pointerId);
    state.pointers.set(event.pointerId, {
      x: event.clientX,
      y: event.clientY,
    });
    state.viewport.classList.add("is-dragging");
  }

  function pointerGeometry(points) {
    if (points.length < 2) return null;
    const [a, b] = points;
    return {
      distance: Math.hypot(b.x - a.x, b.y - a.y),
      midpoint: {
        x: (a.x + b.x) / 2,
        y: (a.y + b.y) / 2,
      },
    };
  }

  function onPointerMove(event) {
    const state = ensureDialog();
    const previous = state.pointers.get(event.pointerId);
    if (!previous) return;
    event.preventDefault();

    const oldPoints = [...state.pointers.values()];
    const oldGeometry = pointerGeometry(oldPoints);
    const current = { x: event.clientX, y: event.clientY };
    state.pointers.set(event.pointerId, current);
    const newGeometry = pointerGeometry([...state.pointers.values()]);

    if (oldGeometry && newGeometry && oldGeometry.distance > 0) {
      zoomAt(
        oldGeometry.midpoint.x,
        oldGeometry.midpoint.y,
        newGeometry.distance / oldGeometry.distance,
      );
      state.x += newGeometry.midpoint.x - oldGeometry.midpoint.x;
      state.y += newGeometry.midpoint.y - oldGeometry.midpoint.y;
    } else {
      state.x += current.x - previous.x;
      state.y += current.y - previous.y;
    }
    applyTransform();
  }

  function onPointerEnd(event) {
    const state = ensureDialog();
    state.pointers.delete(event.pointerId);
    if (!state.pointers.size) state.viewport.classList.remove("is-dragging");
  }

  function handleModalAction(action) {
    const state = ensureDialog();
    if (action === "zoom-in") zoomBy(1.2);
    if (action === "zoom-out") zoomBy(1 / 1.2);
    if (action === "reset") fitDialog();
    if (action === "close") state.dialog.close();
  }

  function handleAction(viewer, button) {
    const action = button.dataset.diagramAction;
    if (action === "fullscreen") openDialog(viewer);
    if (action === "open") openSvg(viewer, button);
    if (action === "copy") copySource(viewer, button);
  }

  function start() {
    enhanceDiagrams();
  }

  start();
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  }

  if (typeof document$ !== "undefined") {
    document$.subscribe(start);
  }
})();
