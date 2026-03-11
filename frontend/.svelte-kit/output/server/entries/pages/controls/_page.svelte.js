import { c as escape_html, s as store_get, b as attr, u as unsubscribe_stores } from "../../../chunks/index2.js";
import { a as appState } from "../../../chunks/stores.js";
import { T as TopBar } from "../../../chunks/TopBar.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let overrideState = "unresolved";
    let reason = "";
    let actor = "operator";
    let confirmText = "";
    TopBar($$renderer2, { title: "Manual Controls" });
    $$renderer2.push(`<!----> <section class="panel"><div class="panel-title"><span>Loop Override</span></div> <div class="status-note">selected loop: ${escape_html(store_get($$store_subs ??= {}, "$appState", appState).selectedLoop || "--")}</div> `);
    $$renderer2.select({ value: overrideState }, ($$renderer3) => {
      $$renderer3.option({ value: "unresolved" }, ($$renderer4) => {
        $$renderer4.push(`unresolved`);
      });
      $$renderer3.option({ value: "overwriting" }, ($$renderer4) => {
        $$renderer4.push(`overwriting`);
      });
      $$renderer3.option({ value: "synced" }, ($$renderer4) => {
        $$renderer4.push(`synced`);
      });
      $$renderer3.option({ value: "flatline" }, ($$renderer4) => {
        $$renderer4.push(`flatline`);
      });
      $$renderer3.option({ value: "cancelled" }, ($$renderer4) => {
        $$renderer4.push(`cancelled`);
      });
    });
    $$renderer2.push(` <input type="text" placeholder="override reason (required)"${attr("value", reason)}/> <input type="text" placeholder="actor (default: operator)"${attr("value", actor)}/> <input type="text" placeholder="type APPLY to confirm"${attr("value", confirmText)}/> <div class="override-controls"><button class="danger">apply override</button></div></section>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
