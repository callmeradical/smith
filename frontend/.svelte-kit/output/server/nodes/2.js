

export const index = 2;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_page.svelte.js')).default;
export const imports = ["_app/immutable/nodes/2.zfPAHLMB.js","_app/immutable/chunks/D7LCSliF.js","_app/immutable/chunks/Cuk7NfDh.js","_app/immutable/chunks/BmXgWk4A.js"];
export const stylesheets = [];
export const fonts = [];
