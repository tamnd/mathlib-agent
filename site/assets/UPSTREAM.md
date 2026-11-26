# Upstream

`site.css`, `site.js`, `icon.svg` and `fonts/` come from [math2-agent](https://github.com/tamnd/math2-agent) at commit `a82a3ddc7a828fed3258dee146c0c7249b3747d2` (`site/assets`), under Apache-2.0, through the copy in [mmp-agent](https://github.com/tamnd/mmp-agent). The fonts keep their own licences in `fonts/`.

`site.css` has no change, so the site looks the same as Math 2.0. The style that Math 2.0 does not have is in `mathlib.css`, which loads after `site.css`. The claim bar of `mathlib.css` comes from `mmp.css` of mmp-agent.

`site.js` has the changes of mmp-agent and a new first line. `lookup.js` is new. It finds the module of a declaration name in the shards of `decl/shard/`.
