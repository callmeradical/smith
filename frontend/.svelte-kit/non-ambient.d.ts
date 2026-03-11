
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	export interface AppTypes {
		RouteId(): "/" | "/controls" | "/documents" | "/pod-view" | "/pod-view/[id]" | "/pods" | "/projects" | "/providers";
		RouteParams(): {
			"/pod-view/[id]": { id: string }
		};
		LayoutParams(): {
			"/": { id?: string };
			"/controls": Record<string, never>;
			"/documents": Record<string, never>;
			"/pod-view": { id?: string };
			"/pod-view/[id]": { id: string };
			"/pods": Record<string, never>;
			"/projects": Record<string, never>;
			"/providers": Record<string, never>
		};
		Pathname(): "/" | "/controls" | "/documents" | `/pod-view/${string}` & {} | "/pods" | "/projects" | "/providers";
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): string & {};
	}
}