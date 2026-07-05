/**
 * 同级导航手风琴：用户手动展开一个分组时，折叠同级其它已展开分组。
 * 不影响页面加载时 Material 为当前路径设置的 checked / indeterminate。
 */
(function () {
  "use strict";

  const PRIMARY = ".md-nav--primary";

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
    if (!toggle.matches(`${PRIMARY} input.md-nav__toggle`)) return;
    if (!toggle.checked) return;

    for (const other of siblingToggles(toggle)) {
      if (other === toggle) continue;
      other.checked = false;
      other.indeterminate = false;
    }
  }

  document.addEventListener("change", onToggleChange, true);
})();
