/* Keep Material's native search, palette and clipboard behavior behind the new UI. */
(function () {
  "use strict";
  let searchOpener;

  function openSearch(opener) {
    const toggle = document.getElementById("__search");
    const input = document.querySelector('[data-md-component="search-query"]');
    if (!toggle || !input) return;
    searchOpener = opener || document.activeElement;
    toggle.checked = true;
    toggle.dispatchEvent(new Event("change", { bubbles: true }));
    requestAnimationFrame(() => input.focus());
  }

  document.addEventListener("click", (event) => {
    const trigger = event.target.closest("[data-library-search]");
    if (trigger) openSearch(trigger);
  });

  document.addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      openSearch();
    }
    if (event.key === "Escape") {
      const toggle = document.getElementById("__search");
      if (toggle?.checked) {
        toggle.checked = false;
        toggle.dispatchEvent(new Event("change", { bubbles: true }));
        searchOpener?.focus();
      }
    }
  });

  // Material listens to keyup; also submit pasted text and completed IME input.
  function submitSearchInput(event) {
    if (!event.isComposing && event.target.matches('[data-md-component="search-query"]')) {
      event.target.dispatchEvent(new KeyboardEvent("keyup", { bubbles: true }));
    }
  }
  document.addEventListener("input", submitSearchInput);
  document.addEventListener("compositionend", submitSearchInput);

  function enhanceArticle() {
    const article = document.querySelector("[data-library-article]");
    const meta = document.querySelector("[data-library-meta]");
    const title = article?.querySelector("h1");
    if (meta && title && title.nextElementSibling !== meta) title.after(meta);
    // Keep the selected article visible without centering the whole navigation
    // past the shelf headings on first load.
    requestAnimationFrame(() => {
      const wrap = document.querySelector(".md-sidebar--primary .md-sidebar__scrollwrap");
      const selected = wrap?.querySelector('[aria-current="page"]');
      if (wrap && selected) {
        const bottom = selected.getBoundingClientRect().bottom - wrap.getBoundingClientRect().top + wrap.scrollTop;
        wrap.scrollTop = Math.max(0, bottom - wrap.clientHeight + 24);
      }
    });
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhanceArticle, { once: true });
  } else {
    enhanceArticle();
  }
  if (typeof document$ !== "undefined") document$.subscribe(enhanceArticle);
})();
