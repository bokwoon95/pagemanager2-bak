"use strict";
document.addEventListener("DOMContentLoaded", function () {
  const selectPageType = document.querySelector("#pm-page-type");
  if (!selectPageType) {
    throw new Error("#pm-page-type not found");
  }
  const groups = {
    template: document.querySelector("#template-group"),
    plugin: document.querySelector("#plugin-group"),
    content: document.querySelector("#content-group"),
    redirect: document.querySelector("#redirect-group"),
    disabled: document.querySelector("#disabled-group"),
  };
  const themePath = document.querySelector("#pm-theme-path");
  if (!themePath) {
    throw new Error("#pm-theme-path not found");
  }
  const themes = {};
  for (const opt of themePath?.children) {
    if (!(opt instanceof HTMLOptionElement)) {
      continue;
    }
    themes[opt.value] = document.getElementById(`theme:${opt.value}`);
  }
  window.themes = themes;
  function render() {
    for (const [name, group] of Object.entries(groups)) {
      group.hidden = name !== selectPageType.value;
    }
    for (const opt of themePath.children) {
      if (!(opt instanceof HTMLOptionElement)) {
        continue;
      }
      themes[opt.value].hidden = !(selectPageType.value === "template" && opt.selected);
    }
  }
  render();
  selectPageType.addEventListener("input", render);
  themePath.addEventListener("input", render);
});
