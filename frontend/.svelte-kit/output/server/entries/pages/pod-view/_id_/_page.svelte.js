import { c as escape_html, g as derived, b as attr } from "../../../../chunks/index2.js";
import { p as page } from "../../../../chunks/index3.js";
import { T as TopBar } from "../../../../chunks/TopBar.js";
import "clsx";
import { o as onDestroy } from "../../../../chunks/index-server.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/root.js";
import "../../../../chunks/state.svelte.js";
function Journal($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { loopID, children } = $$props;
    onDestroy(() => {
    });
    $$renderer2.push(`<div class="waypoint-terminal-window svelte-z46ttj"><div class="terminal-header svelte-z46ttj"><div class="terminal-traffic-lights svelte-z46ttj"><span class="light close svelte-z46ttj"></span> <span class="light minimize svelte-z46ttj"></span> <span class="light maximize svelte-z46ttj"></span></div> <div class="terminal-title svelte-z46ttj">Live Journal: ${escape_html(loopID)}</div></div> <div class="terminal-scroll-area svelte-z46ttj"><pre class="terminal-body svelte-z46ttj">[journal] attaching to stream...</pre> `);
    if (children) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<div class="terminal-input-area svelte-z46ttj">`);
      children($$renderer2);
      $$renderer2.push(`<!----></div>`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]--></div></div>`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    const id = derived(() => page.params.id);
    let command = "";
    let busy = false;
    function controls($$renderer3) {
      $$renderer3.push(`<button type="button">← back</button> <button type="button" class="danger">terminate</button>`);
    }
    TopBar($$renderer2, { title: `Pod: ${id()}`, controls });
    $$renderer2.push(`<!----> `);
    Journal($$renderer2, {
      loopID: id(),
      children: ($$renderer3) => {
        $$renderer3.push(`<div class="pod-command-row journal-prompt-row svelte-1uykdk8"><span class="journal-prompt-glyph svelte-1uykdk8" aria-hidden="true">$</span> <input type="text" placeholder="Run command (e.g. pwd)"${attr("value", command)}${attr("disabled", busy, true)} style="background: transparent; border: none; color: #fff; outline: none; font-family: var(--mono); width: 100%;"/> <button type="button"${attr("disabled", busy, true)} class="primary" style="margin-left: 12px; padding: 4px 12px;">${escape_html("run")}</button></div>`);
      }
    });
    $$renderer2.push(`<!---->`);
  });
}
export {
  _page as default
};
