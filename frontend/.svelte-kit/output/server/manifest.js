export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.0XDy_UqM.js",app:"_app/immutable/entry/app.-82mROQz.js",imports:["_app/immutable/entry/start.0XDy_UqM.js","_app/immutable/chunks/DB7mAMPy.js","_app/immutable/chunks/Cuk7NfDh.js","_app/immutable/chunks/BtZfy_-z.js","_app/immutable/entry/app.-82mROQz.js","_app/immutable/chunks/Cuk7NfDh.js","_app/immutable/chunks/QQvL0PKK.js","_app/immutable/chunks/D7LCSliF.js","_app/immutable/chunks/BtZfy_-z.js","_app/immutable/chunks/Bd61_9cQ.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
