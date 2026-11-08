/**
 * 同级导航手风琴：用户手动展开一个分组时，折叠同级其它已展开分组。
 * 不影响页面加载时 Material 为当前路径设置的 checked / indeterminate。
 */
(function () {
  "use strict";

  const PRIMARY = ".md-nav--primary";

  function labelsFor(toggle) {
    if (!toggle.id) return [];
    return [...document.querySelectorAll(`label[for="${toggle.id}"]`)];
  }

  function toggleName(toggle, labels = labelsFor(toggle)) {
    const labelText = labels
      .map((label) => label.textContent.trim())
      .find(Boolean);
    if (labelText) return labelText;

    const item = toggle.closest(".md-nav__item");
    return (
      item
        ?.querySelector(":scope > .md-nav__link .md-ellipsis")
        ?.textContent.trim() || "导航分组"
    );
  }

  function syncToggleAccessibility(toggle) {
    const labels = labelsFor(toggle);
    const name = toggleName(toggle, labels);
    const expanded = String(toggle.checked);

    toggle.setAttribute("tabindex", "-1");
    toggle.setAttribute("aria-label", name);
    toggle.setAttribute("aria-expanded", expanded);

    for (const label of labels) {
      if (!label.closest(PRIMARY)) continue;
      label.setAttribute("role", "button");
      label.setAttribute("tabindex", "0");
      label.setAttribute("aria-label", name);
      label.setAttribute("aria-expanded", expanded);
    }
  }

  function enhanceDrawerAccessibility() {
    const drawer = document.getElementById("__drawer");
    if (!drawer) return;

    const expanded = String(drawer.checked);
    drawer.setAttribute("tabindex", "-1");
    drawer.setAttribute("aria-label", "导航菜单");
    drawer.setAttribute("aria-expanded", expanded);

    for (const label of labelsFor(drawer)) {
      label.setAttribute("role", "button");
      label.setAttribute("tabindex", "0");
      label.setAttribute("aria-label", "导航菜单");
      label.setAttribute("aria-expanded", expanded);
    }
  }

  function addLeafTitle(link) {
    const text = link.querySelector(".md-ellipsis")?.textContent.trim();
    if (text && !link.hasAttribute("title")) {
      link.setAttribute("title", text);
    }
  }

  function enhanceNavigation() {
    document
      .querySelectorAll(`${PRIMARY} input.md-nav__toggle`)
      .forEach(syncToggleAccessibility);
    document
      .querySelectorAll(
        `${PRIMARY} .md-nav__item:not(.md-nav__item--section) > a.md-nav__link`,
      )
      .forEach(addLeafTitle);
    enhanceDrawerAccessibility();
  }

  function siblingToggles(toggle) {
    const item = toggle.closest(".md-nav__item");
    const list = item?.parentElement;
    if (!list?.classList.contains("md-nav__list")) return [];
    return [...list.children]
      .filter((el) => el.classList.contains("md-nav__item"))
      .map((el) => el.querySelector(":scope > input.md-nav__toggle"))
      .filter(Boolean);
  }

  function onToggleChange(event) {
    const toggle = event.target;
    if (!(toggle instanceof HTMLInputElement)) return;
    if (!toggle.matches(`${PRIMARY} input.md-nav__toggle`)) return;

    queueMicrotask(() => {
      if (toggle.checked) {
        for (const other of siblingToggles(toggle)) {
          if (other === toggle) continue;
          other.checked = false;
          other.indeterminate = false;
          syncToggleAccessibility(other);
        }
      }
      syncToggleAccessibility(toggle);
    });
  }

  function onDrawerChange(event) {
    if (event.target.id !== "__drawer") return;
    queueMicrotask(enhanceDrawerAccessibility);
  }

  function onToggleKeydown(event) {
    const label = event.target.closest(
      `${PRIMARY} label.md-nav__link[role="button"], ` +
        ".md-header label.md-header__button[role=\"button\"]",
    );
    if (!label || (event.key !== "Enter" && event.key !== " ")) return;
    event.preventDefault();

    const toggle = document.getElementById(label.htmlFor);
    if (!(toggle instanceof HTMLInputElement)) return;

    toggle.checked = !toggle.checked;
    toggle.indeterminate = false;
    toggle.dispatchEvent(new Event("change", { bubbles: true }));
  }

  document.addEventListener("change", onToggleChange, true);
  document.addEventListener("change", onDrawerChange, true);
  document.addEventListener("keydown", onToggleKeydown, true);

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhanceNavigation, {
      once: true,
    });
  } else {
    enhanceNavigation();
  }

  if (typeof document$ !== "undefined") {
    document$.subscribe(enhanceNavigation);
  }
})();
