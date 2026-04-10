// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	interface ImportMetaEnv {
		readonly STORYBOOK_PUBLIC_API_BASE_URL?: string;
	}

	interface ImportMeta {
		readonly env: ImportMetaEnv;
	}

	var __PUBLIC_ENV__:
		| {
				PUBLIC_API_BASE_URL?: string;
				STORYBOOK_PUBLIC_API_BASE_URL?: string;
		  }
		| undefined;

	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
