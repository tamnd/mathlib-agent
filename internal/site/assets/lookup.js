// The lookup of a declaration name. The build writes decl/shard/<xx>.json, where xx is the low byte of the FNV-1a hash of the UTF-8 name, as two hex digits. Each shard maps a name to its module. The page /decl/<name>/ does not exist as a file, so the server returns 404.html, and this script sends the reader to the module page of the name.
(function () {
  "use strict";

  var base = document.body.dataset.base || "/";
  var status = document.getElementById("lookup-status");

  function shard(name) {
    var bytes = new TextEncoder().encode(name);
    var h = 0x811c9dc5;
    for (var i = 0; i < bytes.length; i++) {
      h ^= bytes[i];
      h = Math.imul(h, 0x01000193) >>> 0;
    }
    var x = (h & 255).toString(16);
    return x.length < 2 ? "0" + x : x;
  }

  function say(text) {
    if (status) status.textContent = text;
  }

  function go(name) {
    name = name.trim();
    if (!name) return;
    say("Looking for " + name + ".");
    fetch(base + "decl/shard/" + shard(name) + ".json").then(function (r) { return r.json(); }).then(function (names) {
      var mod = names[name];
      if (mod) {
        location.replace(base + "module/" + mod + "/#" + encodeURIComponent(name));
      } else {
        say("No declaration of this index has the name " + name + ". Give the full name, for example Nat.add_comm.");
      }
    }).catch(function () { say("The lookup table did not load."); });
  }

  var prefix = base + "decl/";
  var path = location.pathname;
  if (path.indexOf(prefix) === 0 && path.length > prefix.length && path.indexOf(prefix + "shard/") !== 0) {
    go(decodeURIComponent(path.slice(prefix.length).replace(/\/$/, "")));
    return;
  }
  var q = new URL(location.href).searchParams.get("q");
  var box = document.getElementById("lookup");
  if (box && q) {
    box.value = q;
    go(q);
  }
})();
