(function () {
  var key = "smith-docs-theme";
  var stored = window.localStorage.getItem(key);
  if (stored) {
    document.body.setAttribute("data-smith-theme", stored);
  }

  document.addEventListener("click", function (event) {
    var toggle = event.target.closest("[data-md-color-switch]");
    if (!toggle) {
      return;
    }
    var next = document.documentElement.getAttribute("data-md-color-scheme") || "default";
    window.localStorage.setItem(key, next);
    document.body.setAttribute("data-smith-theme", next);
  });
})();
