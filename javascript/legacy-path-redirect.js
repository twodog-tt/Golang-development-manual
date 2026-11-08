(function () {
  "use strict";
  var path = window.location.pathname;
  var next = path;
  if (path.indexOf("/interview/") !== -1) {
    next = path.split("/interview/").join("/topics/");
  } else if (/\/interview-catalog\/?$/.test(path)) {
    next = path.replace(/interview-catalog\/?$/, "topic-catalog/");
  }
  if (next !== path) {
    window.location.replace(next + window.location.search + window.location.hash);
  }
})();
