/// <reference types="vitest/config" />
import tailwindcss from "@tailwindcss/vite";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

export default defineConfig({
	envPrefix: ["VITE_", "STORYBOOK_PUBLIC_"],
	plugins: [tailwindcss(), sveltekit()],
	server: {
		fs: {
			allow: [".."],
		},
	},
	test: {
		projects: [
			{
				extends: true,
				test: {
					name: "unit",
					environment: "node",
					include: ["src/**/*.test.ts"],
					exclude: ["src/**/*.stories.@(js|ts|svelte)", "src/**/*.mdx"],
				},
			},
		],
	},
});
