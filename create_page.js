"use strict";
document.addEventListener("DOMContentLoaded", function () {
  const pageType = document.querySelector("select#pm-page-type");
  if (!pageType) {
    throw new Error("select#pm-page-type not found");
  }
  const groups = {
    template: document.querySelector("#template-group"),
    plugin: document.querySelector("#plugin-group"),
    content: document.querySelector("#content-group"),
    redirect: document.querySelector("#redirect-group"),
    disabled: document.querySelector("#disabled-group"),
  };
  const themePath = document.querySelector("select#pm-theme-path");
  if (!themePath) {
    throw new Error("select#pm-theme-path not found");
  }
  function render() {
    for (const [name, group] of Object.entries(groups)) {
      group.hidden = pageType.value !== name;
    }
    for (const group of document.querySelectorAll("[id^=pm-templatefor-]")) {
      group.hidden = !(pageType.value === "template" && group.id === `pm-templatefor-${themePath.value}`);
    }
  }
  render();
  pageType.addEventListener("input", render);
  themePath.addEventListener("input", render);
});
