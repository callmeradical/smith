

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/fallbacks/layout.svelte.js')).default;
export const imports = ["_app/immutable/nodes/0.aj79lFY2.js","_app/immutable/chunks/D7LCSliF.js","_app/immutable/chunks/Cuk7NfDh.js","_app/immutable/chunks/Bd61_9cQ.js"];
export const stylesheets = [];
export const fonts = [];
