import { j as ssr_context } from "./index2.js";
import "clsx";
function onDestroy(fn) {
  /** @type {SSRContext} */
  ssr_context.r.on_destroy(fn);
}
export {
  onDestroy as o
};
