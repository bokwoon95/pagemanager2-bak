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
  function render() {
    for (const [name, group] of Object.entries(groups)) {
      group.hidden = name !== selectPageType.value;
    }
  }
  render();
  selectPageType.addEventListener("input", render);
});
