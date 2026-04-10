import type { Preview } from "@storybook/sveltekit";
import "../src/routes/main.css";
import "bootstrap-icons/font/bootstrap-icons.css";

globalThis.__PUBLIC_ENV__ = Object.freeze({
	...globalThis.__PUBLIC_ENV__,
	STORYBOOK_PUBLIC_API_BASE_URL: import.meta.env.STORYBOOK_PUBLIC_API_BASE_URL,
});

const preview: Preview = {
	parameters: {
		layout: "fullscreen",
		controls: {
			matchers: {
				color: /(background|color)$/i,
				date: /Date$/i,
			},
		},

		a11y: {
			// 'todo' - show a11y violations in the test UI only
			// 'error' - fail CI on a11y violations
			// 'off' - skip a11y checks entirely
			test: "todo",
		},
	},
};

export default preview;
